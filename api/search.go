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
	trigrams := getTrigrams(name)
	for _, trigram := range trigrams {
		if _, exists := s.trigramMap[trigram]; !exists {
			s.trigramMap[trigram] = roaring64.New()
		}
		s.trigramMap[trigram].Add(id)
	}
}

func (s *SearchIndex) Search(query string) []uint64 {
	trigrams := getTrigrams(query)
	var results *roaring64.Bitmap
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
	ids := search.Search(query)
	var results []Media
	for _, id := range ids {
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
