package main

import (
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/rs/zerolog/log"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SearchIndex struct {
	sync.RWMutex
	trigramMap map[string]*ConcurrentBitmap
}

type SearchResult struct {
	Id              uint64         `json:"id"`
	Title           string         `json:"title"`
	Creators        []MediaCreator `json:"creators"`
	CoverUrl        string         `json:"coverUrl"`
	Subtitle        string         `json:"subtitle"`
	Description     string         `json:"description"`
	SeriesName      string         `json:"seriesName"`
	SeriesReadOrder int            `json:"seriesReadOrder"`
	SearchString    string         `json:"searchString"`
	LibraryCount    int            `json:"libraryCount"`
	Languages       []string       `json:"languages"`
	Formats         []string       `json:"formats"`
}

var search = NewSearchIndex()

var ngramIDQueues = &sync.Map{}

func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		trigramMap: make(map[string]*ConcurrentBitmap),
	}
}

func (s *SearchIndex) Set(key string, value *ConcurrentBitmap) {
	s.Lock()
	defer s.Unlock()
	s.trigramMap[key] = value
}

func (s *SearchIndex) Get(key string) (*ConcurrentBitmap, bool) {
	s.RLock()
	defer s.RUnlock()
	bitmap, ok := s.trigramMap[key]
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

func (s *SearchIndex) Index(name string, id uint64, group *sync.WaitGroup) {
	name = strings.TrimSpace(name)
	trigrams := getNgrams(name)
	for _, trigram := range trigrams {
		queue, ok := ngramIDQueues.Load(trigram)
		if !ok {
			queue = make(chan uint32, 1000)
			ngramIDQueues.Store(trigram, queue)
			bitmap, exists := s.Get(trigram)
			if !exists {
				bitmap = NewConcurrentBitmap()
				s.Set(trigram, bitmap)
			}
			go func() {
				bitmapWorker(queue.(chan uint32), bitmap, trigram)
			}()
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
	query = strings.TrimSpace(query)
	// TODO remove this hackiness
	query = strings.Replace(query, " and ", " ", -1)
	query = strings.Replace(query, " & ", " ", -1)
	trigrams := getNgrams(query)
	log.Debug().Any("trigrams", trigrams).Msg("trigrams...")
	var results *roaring.Bitmap
	for _, trigram := range trigrams {
		bitmap, exists := s.Get(trigram)
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
	if results == nil {
		return []uint32{}
	}
	return results.ToArray()
}

func (s *SearchIndex) Finalize() {
	ngramIDQueues.Range(func(key, value interface{}) bool {
		close(value.(chan uint32))
		return true
	})
}

func substring(s string, start int, end int) string {
	start_str_idx := 0
	i := 0
	for j := range s {
		if i == start {
			start_str_idx = j
		}
		if i == end {
			return s[start_str_idx:j]
		}
		i++
	}
	return s[start_str_idx:]
}

func getRune(s string, idx int) rune {
	for j, r := range s {
		if j == idx {
			return r
		}
	}
	return ' '
}

func getNgrams(s string) []string {
	lower := strings.ToLower(s)
	ngrams := make(map[string]struct{})
	for i := 0; i < len(lower)-2; i++ {
		trigram := lower[i : i+3]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams[trigram] = struct{}{}
	}
	for i := 0; i < len(lower)-1; i++ {
		trigram := lower[i : i+2]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams[trigram] = struct{}{}
	}
	for i := 0; i < len(lower); i++ {
		trigram := lower[i : i+1]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams[trigram] = struct{}{}
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
		if bitmap.(*ConcurrentBitmap).Contains(uint32(media.Id)) {
			formats = append(formats, format.(string))
		}
		return true
	})
	languageMap.Range(func(language, bitmap interface{}) bool {
		if bitmap.(*ConcurrentBitmap).Contains(uint32(media.Id)) {
			languages = append(languages, language.(string))
		}
		return true
	})
	sort.Strings(formats)
	sort.Strings(languages)
	return &SearchResult{
		Id:              media.Id,
		Title:           media.Title,
		Creators:        media.Creators,
		CoverUrl:        media.CoverUrl,
		Subtitle:        media.Subtitle,
		Description:     media.Description,
		SeriesName:      media.SeriesName,
		SeriesReadOrder: media.SeriesReadOrder,
		SearchString:    media.SearchString,
		LibraryCount:    len(availabilityMap[uint32(media.Id)]),
		Languages:       languages,
		Formats:         formats,
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	lowerQuery := strings.ToLower(query)
	log.Debug().Msgf("/api/search q: %v", query)
	startTime := time.Now()
	var results []*SearchResult
	ids := search.Search(query)
	for _, id := range ids {
		media, _ := mediaMap.Get(id)
		searchResult := NewSearchResult(media)
		results = append(results, searchResult)
		if len(results) >= 500 {
			break
		}
	}
	// TODO paginate this better
	result := map[string][]*SearchResult{}
	sort.Sort(ByCustomSearchSortOrder{query: lowerQuery, results: results})
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

func longestCommonSubstring(a, b string) int {
	log.Debug().Str("a", a).Str("b", b).Msg("longestCommonSubstring...")
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}

	maxLen := 0

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
				if dp[i][j] > maxLen {
					maxLen = dp[i][j]
				}
			} else {
				dp[i][j] = 0
			}
		}
	}

	return maxLen
}

type ByCustomSearchSortOrder struct {
	query   string
	results []*SearchResult
}

func (a ByCustomSearchSortOrder) Len() int {
	return len(a.results)
}

func (a ByCustomSearchSortOrder) Swap(i, j int) {
	a.results[i], a.results[j] = a.results[j], a.results[i]
}

func (a ByCustomSearchSortOrder) Less(i, j int) bool {
	lcsI := longestCommonSubstring(a.query, a.results[i].SearchString)
	lcsJ := longestCommonSubstring(a.query, a.results[j].SearchString)
	if lcsI == lcsJ {
		log.Debug().
			Any("i.Id", a.results[i].Id).Any("j.Id", a.results[j].Id).
			Int("a.results[i].LibraryCount", a.results[i].LibraryCount).
			Int("a.results[j].LibraryCount", a.results[j].LibraryCount).
			Msg("librarycount...")
		return a.results[i].LibraryCount > a.results[j].LibraryCount
	}
	log.Debug().
		Any("i.Id", a.results[i].Id).Any("j.Id", a.results[j].Id).
		Int("lcsI", lcsI).
		Int("lcsJ", lcsJ).
		Msg("lcs...")
	return lcsI > lcsJ
}
