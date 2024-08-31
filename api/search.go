package main

import (
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/rs/zerolog/log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type SearchIndex struct {
	sync.RWMutex
	trigramMap map[string]*ConcurrentBitmap
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

func (s *SearchIndex) Index(name string, id uint64) {
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
			go bitmapWorker(queue.(chan uint32), bitmap)
		}
		queue.(chan uint32) <- uint32(id)
	}
}

func bitmapWorker(queue chan uint32, bitmap *ConcurrentBitmap) {
	for id := range queue {
		bitmap.Add(id)
	}
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

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	lowerQuery := strings.ToLower(query)
	log.Debug().Msgf("/api/search q: %v", query)
	startTime := time.Now()
	var results []*Media
	ids := search.Search(query)
	for _, id := range ids {
		media, _ := mediaMap.Get(id)
		media.LibraryCount = len(availabilityMap[id])
		results = append(results, media)
		if len(results) >= 500 {
			break
		}
	}
	// TODO paginate this better
	result := map[string][]*Media{}
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
	results []*Media
}

func (a ByCustomSearchSortOrder) Len() int {
	return len(a.results)
}

func (a ByCustomSearchSortOrder) Swap(i, j int) {
	a.results[i], a.results[j] = a.results[j], a.results[i]
}

func (a ByCustomSearchSortOrder) Less(i, j int) bool {
	//sortScoreI := longestCommonSubstring(a.query, a.results[i].SearchString) * 10
	//sortScoreJ := longestCommonSubstring(a.query, a.results[j].SearchString) * 10
	lcsI := longestCommonSubstring(a.query, a.results[i].SearchString)
	lcsJ := longestCommonSubstring(a.query, a.results[j].SearchString)
	if lcsI == lcsJ {
		if a.results[i].SeriesName != a.results[j].SeriesName {
			log.Debug().
				Any("i.Id", a.results[i].Id).Any("j.Id", a.results[j].Id).
				Str("a.results[i].SeriesName", a.results[i].SeriesName).
				Str("a.results[j].SeriesName", a.results[j].SeriesName).
				Msg("SeriesName...")
			return a.results[i].SeriesName < a.results[j].SeriesName
		}
		if a.results[i].SeriesReadOrder != a.results[j].SeriesReadOrder {
			log.Debug().
				Any("i.Id", a.results[i].Id).Any("j.Id", a.results[j].Id).
				Int("a.results[i].SeriesReadOrder", a.results[i].SeriesReadOrder).
				Int("a.results[j].SeriesReadOrder", a.results[j].SeriesReadOrder).
				Msg("readOrder...")
			return a.results[i].SeriesReadOrder < a.results[j].SeriesReadOrder
		}
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
