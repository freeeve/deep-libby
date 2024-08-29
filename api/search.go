package main

import (
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
	"time"
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
	trigrams := getNgrams(name)
	for _, trigram := range trigrams {
		if _, exists := s.trigramMap[trigram]; !exists {
			s.trigramMap[trigram] = roaring64.New()
		}
		s.trigramMap[trigram].Add(id)
	}
}

func (s *SearchIndex) Search(query string) []uint64 {
	query = strings.TrimSpace(query)
	// TODO remove this hackiness
	query = strings.Replace(query, " and ", " ", -1)
	query = strings.Replace(query, " & ", " ", -1)
	trigrams := getNgrams(query)
	log.Debug().Any("trigrams", trigrams).Msg("trigrams...")
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
	if results == nil {
		return []uint64{}
	}
	return results.ToArray()
}

func getNgrams(s string) []string {
	lower := strings.ToLower(s)
	var ngrams []string
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
	log.Debug().Msgf("/api/search q: %v", query)
	startTime := time.Now()
	var results []Media
	if dataLoaded {
		ids := search.Search(query)
		for _, id := range ids {
			availability, availabilityExists := availabilityMap[id]
			if !availabilityExists || len(availability) == 0 {
				// ignore books without availability; preorder or requested? it is a mystery
				continue
			}
			results = append(results, mediaMap[id])
			if len(results) >= 1000 {
				break
			}
		}
	} else {
		log.Error().Msg("search while data not loaded")
		results = append(results, Media{
			Id:    1,
			Title: "server is initializing. please try again in a minute...",
			Creators: []MediaCreator{
				{
					Name: "deep-libby",
					Role: "server",
				},
			},
			Formats:   []string{"text"},
			Languages: []string{"english"},
			CoverUrl:  "https://img2.od-cdn.com/ImageType-150/1523-1/%7B10C96090-70C0-4FA7-842A-AA657883F9B1%7DIMG150.JPG",
		})
	}
	// TODO paginate this better
	// TODO sort better
	result := map[string][]Media{}
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
