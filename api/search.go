package main

import (
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type SearchIndex struct {
	sync.RWMutex
	ngramMap     map[string]*ConcurrentBitmap
	isbn13Lookup map[uint64]uint32
}

type SearchResult struct {
	Id              uint32         `json:"id"`
	Title           string         `json:"title"`
	Creators        []MediaCreator `json:"creators"`
	Publisher       string         `json:"publisher"`
	PublisherId     uint32         `json:"publisherId"`
	CoverUrl        string         `json:"coverUrl"`
	Subtitle        string         `json:"subtitle"`
	Description     string         `json:"description"`
	SeriesName      string         `json:"seriesName"`
	SeriesReadOrder uint16         `json:"seriesReadOrder"`
	LibraryCount    int            `json:"libraryCount"`
	Languages       []string       `json:"languages"`
	Formats         []string       `json:"formats"`
}

var search = NewSearchIndex()

var ngramIDQueues = &sync.Map{}

func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		ngramMap:     make(map[string]*ConcurrentBitmap),
		isbn13Lookup: map[uint64]uint32{},
	}
}

func (s *SearchIndex) Set(key string, value *ConcurrentBitmap) {
	s.Lock()
	defer s.Unlock()
	s.ngramMap[key] = value
}

func (s *SearchIndex) Get(key string) (*ConcurrentBitmap, bool) {
	s.RLock()
	defer s.RUnlock()
	bitmap, ok := s.ngramMap[key]
	return bitmap, ok
}

type ConcurrentBitmap struct {
	sync.RWMutex
	bitmap *roaring.Bitmap
}

func NewConcurrentBitmap() *ConcurrentBitmap {
	return &ConcurrentBitmap{
		bitmap: roaring.New(),
	}
}

func (cb *ConcurrentBitmap) Add(x uint32) {
	cb.Lock()
	defer cb.Unlock()
	cb.bitmap.Add(x)
}

func (cb *ConcurrentBitmap) Clone() *roaring.Bitmap {
	cb.RLock()
	defer cb.RUnlock()
	return cb.bitmap.Clone()
}

func (cb *ConcurrentBitmap) ToArray() []uint32 {
	cb.RLock()
	defer cb.RUnlock()
	return cb.bitmap.ToArray()
}

func (cb *ConcurrentBitmap) Contains(id uint32) bool {
	cb.RLock()
	defer cb.RUnlock()
	return cb.bitmap.Contains(id)
}

func (cb *ConcurrentBitmap) UnsafeBitmap() *roaring.Bitmap {
	return cb.bitmap
}

func (s *SearchIndex) Index(name string, id uint32) {
	name = strings.TrimSpace(name)
	ngrams := getNgrams(name)
	for _, ngram := range ngrams {
		bitmap, exists := s.Get(ngram)
		if !exists {
			bitmap = NewConcurrentBitmap()
			s.Set(ngram, bitmap)
		}
		bitmap.Add(id)
	}
}

func (s *SearchIndex) IndexISBN(isbn13 uint64, id uint32) {
	s.isbn13Lookup[isbn13] = id
}

func (s *SearchIndex) SearchISBN(isbn13 uint64) (uint32, bool) {
	id, exists := s.isbn13Lookup[isbn13]
	if !exists {
		return 0, false
	}
	return id, true
}

func (s *SearchIndex) IndexWG(name string, id uint64, group *sync.WaitGroup) {
	name = strings.TrimSpace(name)
	ngrams := getNgrams(name)
	for _, ngram := range ngrams {
		queue, ok := ngramIDQueues.Load(ngram)
		if !ok {
			queue = make(chan uint32, 200)
			ngramIDQueues.Store(ngram, queue)
			bitmap, exists := s.Get(ngram)
			if !exists {
				bitmap = NewConcurrentBitmap()
				s.Set(ngram, bitmap)
			}
			go bitmapWorker(queue.(chan uint32), bitmap, ngram)

		}
		queue.(chan uint32) <- uint32(id)
	}
	group.Done()
}

func bitmapWorker(queue chan uint32, bitmap *ConcurrentBitmap, ngram string) {
	log.Trace().Msgf("bitmapWorker starting for... %s", ngram)
	var count int
	for id := range queue {
		bitmap.Add(id)
		count++
	}
	time.Sleep(time.Duration(rand.Int()) * time.Millisecond)
	log.Trace().Msgf("bitmapWorker done for... %s; count: %d", ngram, count)
}

func (s *SearchIndex) Search(query string) []uint32 {
	results := s.SearchBitmapResult(query)
	if results == nil {
		return []uint32{}
	}
	return results.ToArray()
}

func (s *SearchIndex) SearchBitmapResult(query string) *roaring.Bitmap {
	query = strings.TrimSpace(query)
	// TODO remove this hackiness
	query = strings.Replace(query, " and ", " ", -1)
	query = strings.Replace(query, " & ", " ", -1)
	query = strings.Replace(query, " by ", " ", -1)
	ngrams := getNgrams(query)
	log.Trace().Any("ngrams", ngrams).Msg("ngrams...")
	var results *roaring.Bitmap
	for _, ngram := range ngrams {
		bitmap, exists := s.Get(ngram)
		if !exists {
			return nil
		}
		if results == nil {
			results = bitmap.Clone()
		} else {
			bitmap.RWMutex.RLock()
			results.And(bitmap.bitmap)
			bitmap.RWMutex.RUnlock()
		}
	}
	return results
}

func (s *SearchIndex) Finalize() {
	ngramIDQueues.Range(func(key, value interface{}) bool {
		close(value.(chan uint32))
		return true
	})
}

func getNgrams(s string) []string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	lower, _, err := transform.String(t, s)
	if err != nil {
		lower = s
	}
	lower = strings.ToLower(lower)
	ngrams := make(map[string]struct{})
	for i := 0; i < len(lower)-2; i++ {
		ngram := lower[i : i+3]
		if strings.Contains(ngram, " ") {
			continue
		}
		ngrams[ngram] = struct{}{}
	}
	for i := 0; i < len(lower)-1; i++ {
		ngram := lower[i : i+2]
		if strings.Contains(ngram, " ") {
			continue
		}
		ngrams[ngram] = struct{}{}
	}
	for i := 0; i < len(lower); i++ {
		ngram := lower[i : i+1]
		if strings.Contains(ngram, " ") {
			continue
		}
		ngrams[ngram] = struct{}{}
	}
	uniqueNgrams := make([]string, 0, len(ngrams))
	for ngram := range ngrams {
		uniqueNgrams = append(uniqueNgrams, ngram)
	}
	return uniqueNgrams
}

func NewSearchResult(media *Media) *SearchResult {
	var formats []string
	var languages []string
	formatMap.Range(func(format, bitmap interface{}) bool {
		if bitmap.(*ConcurrentBitmap).Contains(media.Id) {
			formats = append(formats, format.(string))
		}
		return true
	})
	languageMap.Range(func(language, bitmap interface{}) bool {
		if bitmap.(*ConcurrentBitmap).Contains(media.Id) {
			languages = append(languages, language.(string))
		}
		return true
	})
	sort.Strings(formats)
	sort.Strings(languages)
	title := media.Title
	coverUrl := media.CoverUrl
	description := media.Description
	libraryCount := 0
	err := db.View(func(txn *badger.Txn) error {
		prefix := getMediaAvailabilityPrefix(media.Id)
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		iter := txn.NewIterator(opts)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			libraryCount++
		}
		return nil
	})
	if err != nil {
		log.Err(err)
	}
	result := &SearchResult{
		Id:              media.Id,
		Title:           title,
		Creators:        media.Creators,
		CoverUrl:        coverUrl,
		Description:     description,
		LibraryCount:    libraryCount,
		Languages:       languages,
		Formats:         formats,
		Subtitle:        media.Subtitle,
		SeriesName:      media.Series,
		SeriesReadOrder: media.SeriesReadOrder,
		Publisher:       media.Publisher,
		PublisherId:     media.PublisherId,
	}
	return result
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	log.Debug().Msgf("/api/search q: %v", query)
	startTime := time.Now()
	var results []*SearchResult
	ids := search.Search(query)
	for _, id := range ids {
		media, _ := getMedia(id)
		searchResult := NewSearchResult(media)
		results = append(results, searchResult)
		if len(results) >= 500 {
			break
		}
	}
	result := map[string][]*SearchResult{}
	result["results"] = results
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		log.Error().Err(err)
	}
	log.Info().Int("results", len(results)).
		Str("duration", fmt.Sprintf("%dms", time.Since(startTime)/time.Millisecond)).
		Msgf("/api/search q: %v", query)
}

func searchDebugHandler(w http.ResponseWriter, r *http.Request) {
	ngram := r.URL.Query().Get("ngram")
	mediaId := r.URL.Query().Get("mediaId")
	mediaIdInt, err := strconv.Atoi(mediaId)
	if err != nil {
		panic(err)
	}
	log.Debug().Msgf("/api/search-debug: ngram %v, mediaId %v", ngram, mediaId)
	result := map[string]bool{}
	bitmap, exists := search.Get(ngram)
	if !exists {
		result["ngramBitmapExists"] = false
		w.Header().Add("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(result)
		if err != nil {
			panic(err)
		}
		return
	}
	result["mediaSetForNgram"] = bitmap.Contains(uint32(mediaIdInt))
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}
