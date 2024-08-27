package main

import (
	"encoding/json"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
)

type SearchIndex struct {
	trigramMap map[string]*roaring64.Bitmap
}

var search = NewSearchIndex()

func NewSearchIndex() *SearchIndex {
	return &SearchIndex{
		trigramMap: make(map[string]*roaring64.Bitmap),
	}
}

func (s *SearchIndex) Index(name string, id uint64) {
	name = strings.TrimSpace(name)
	trigrams := getTrigrams(name)
	for _, trigram := range trigrams {
		if _, exists := s.trigramMap[trigram]; !exists {
			s.trigramMap[trigram] = roaring64.New()
		}
		s.trigramMap[trigram].Add(id)
	}
}

func (s *SearchIndex) Search(query string) []uint64 {
	query = strings.TrimSpace(query)
	trigrams := getTrigrams(query)
	var results *roaring64.Bitmap
	for _, trigram := range trigrams {
		if strings.Contains(trigram, " ") {
			continue
		}
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
		return []uint64{}
	}
	return results.ToArray()
}

func getTrigrams(s string) []string {
	var trigrams []string
	for i := 0; i < len(s)-2; i++ {
		trigrams = append(trigrams, strings.ToLower(s[i:i+3]))
	}
	return trigrams
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	lowerQuery := strings.ToLower(query)
	log.Info().Msgf("/api/search q: %v", query)
	ids := search.Search(query)
	var results []Media
	var lastIds []uint64
	for _, id := range ids {
		found := false
		media := mediaMap[id]
		if strings.Contains(strings.ToLower(media.Title), lowerQuery) {
			results = append(results, mediaMap[id])
			found = true
		} else {
			for _, creator := range media.Creators {
				if strings.Contains(strings.ToLower(creator.Name), lowerQuery) {
					results = append(results, mediaMap[id])
					found = true
					break
				}
			}
			for _, language := range media.Languages {
				if strings.Contains(strings.ToLower(language), lowerQuery) {
					results = append(results, mediaMap[id])
					found = true
					break
				}
			}
		}
		if found == false {
			lastIds = append(lastIds, id)
		}
		if len(results) >= 1000 {
			break
		}
	}
	for _, id := range lastIds {
		if len(results) >= 1000 {
			break
		}
		results = append(results, mediaMap[id])
	}
	result := map[string][]Media{}
	result["results"] = results
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		log.Error().Err(err)
	}
}
