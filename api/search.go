package main

import (
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/rs/zerolog/log"
	"net/http"
	"sort"
	"strings"
	"time"
)

type SearchIndex struct {
	trigramMap map[string]*roaring.Bitmap
}

var search = NewSearchIndex()

func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		trigramMap: make(map[string]*roaring.Bitmap),
	}
}

func (s *SearchIndex) Index(name string, id uint64) {
	name = strings.TrimSpace(name)
	trigrams := getNgrams(name)
	for _, trigram := range trigrams {
		if _, exists := s.trigramMap[trigram]; !exists {
			s.trigramMap[trigram] = roaring.New()
		}
		s.trigramMap[trigram].Add(uint32(id))
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
		if _, exists := s.trigramMap[trigram]; !exists {
			return nil
		}
		if results == nil {
			results = s.trigramMap[trigram].Clone()
		} else {
			results.And(s.trigramMap[trigram])
		}
	}
	if results == nil {
		return []uint32{}
	}
	return results.ToArray()
}

func getNgrams(s string) []string {
	lower := strings.ToLower(s)
	var ngrams []string
	/*
		for i := 0; i < len(lower)-3; i++ {
			trigram := lower[i : i+4]
			if strings.Contains(trigram, " ") {
				continue
			}
			ngrams = append(ngrams, trigram)
		}*/
	for i := 0; i < len(lower)-2; i++ {
		trigram := lower[i : i+3]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams = append(ngrams, trigram)
	}
	for i := 0; i < len(lower)-1; i++ {
		trigram := lower[i : i+2]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams = append(ngrams, trigram)
	}
	for i := 0; i < len(lower); i++ {
		trigram := lower[i : i+1]
		if strings.Contains(trigram, " ") {
			continue
		}
		ngrams = append(ngrams, trigram)
	}
	return ngrams
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
		if len(results) >= 1000 {
			break
		}
	}
	// TODO paginate this better
	// TODO sort better
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
	lcsI := longestCommonSubstring(a.query, a.results[i].SearchString)
	lcsJ := longestCommonSubstring(a.query, a.results[j].SearchString)

	if lcsI == lcsJ {
		if a.results[i].SeriesName != a.results[j].SeriesName {
			return a.results[i].SeriesName < a.results[j].SeriesName
		}
		if a.results[i].SeriesReadOrder != a.results[j].SeriesReadOrder {
			return a.results[i].SeriesReadOrder < a.results[j].SeriesReadOrder
		}
		return a.results[i].LibraryCount > a.results[j].LibraryCount
	}

	return lcsI > lcsJ
}
