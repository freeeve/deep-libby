package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type HardcoverGraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type HardcoverBooksResponse struct {
	Data struct {
		Users []struct {
			UserBooks []struct {
				Book struct {
					Editions []struct {
						ISBN13 string `json:"isbn_13"`
					} `json:"editions"`
				} `json:"book"`
			} `json:"user_books"`
		} `json:"users"`
	} `json:"data"`
}

func getHardcoverBooksByUsername(username, additionalFilters string) []*SearchResult {
	query := `
    query MyQuery($username: citext) {
	  users(where: {username: {_eq: $username}}) {
		user_books(where: {status_id: {_eq: 1}}) {
		  book {
			editions(where: {isbn_13: {_is_null: false}}) {
			  isbn_13
			}
		  }
		}
	  }
	}`

	buf, err := json.Marshal(HardcoverGraphQLRequest{
		Query: query,
		Variables: map[string]interface{}{
			"username": username,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal GraphQL request")
		return nil
	}

	req, err := http.NewRequest("POST", "https://api.hardcover.app/v1/graphql", bytes.NewBuffer(buf))
	if err != nil {
		log.Error().Err(err).Msg("failed to create HTTP request")
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("HARDCOVER_API_TOKEN")))

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to post GraphQL request")
		return nil
	}
	defer resp.Body.Close()
	duration := time.Since(start)
	log.Info().Int64("durationNs", duration.Nanoseconds()).
		Int64("durationMs", duration.Milliseconds()).
		Msg("Hardcover API request duration")

	var result HardcoverBooksResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Error().Err(err).Msg("failed to decode GraphQL response")
		return nil
	}

	if len(result.Data.Users) == 0 {
		log.Warn().Msg("no books found for the user")
		return nil
	}

	var isbns []string
	for _, users := range result.Data.Users {
		for _, userBooks := range users.UserBooks {
			for _, edition := range userBooks.Book.Editions {
				isbns = append(isbns, edition.ISBN13)
			}
		}
	}

	return searchMediaByIsbns(isbns, additionalFilters)
}

func searchMediaByIsbns(isbns []string, additionalFilters string) []*SearchResult {
	log.Trace().Msgf("Searching media by ISBNs: %v", isbns)
	bitmap := roaring.NewBitmap()
	start := time.Now()
	for _, isbn := range isbns {
		if len(isbn) == 13 && (strings.HasPrefix(isbn, "978") || strings.HasPrefix(isbn, "979")) {
			isbnInt, err := strconv.ParseUint(isbn, 10, 64)
			if err == nil {
				id, exists := search.SearchISBN(isbnInt)
				if exists {
					log.Trace().Msgf("Found media id %d with ISBN: %d", id, isbnInt)
					bitmap.Add(id)
				}
			}
		}
	}
	duration := time.Since(start)
	log.Info().Int64("durationNs", duration.Nanoseconds()).
		Int64("durationMs", duration.Milliseconds()).
		Int("isbnCount", len(isbns)).
		Uint64("mediaCount", bitmap.GetCardinality()).
		Msg("ISBN search")

	start = time.Now()
	if additionalFilters != "" {
		additionalFiltersBitmap := search.SearchBitmapResult(additionalFilters)
		duration = time.Since(start)
		log.Info().Int64("durationNs", duration.Nanoseconds()).
			Int64("durationMs", duration.Milliseconds()).
			Msg("Additional filters search duration")

		log.Debug().Msgf("Additional filters bitmap length: %d", additionalFiltersBitmap.GetCardinality())
		log.Debug().Msgf("results bitmap length: %d", bitmap.GetCardinality())

		bitmap.And(additionalFiltersBitmap)
		log.Debug().Msgf("results bitmap length after AND: %d", bitmap.GetCardinality())
	} else {
		log.Debug().Msg("No additional filters")
	}

	start = time.Now()
	results := make([]*SearchResult, 0, bitmap.GetCardinality())
	bitmap.Iterate(func(id uint32) bool {
		media, err := getMedia(id)
		if err == nil {
			searchResult := NewSearchResult(media)
			results = append(results, searchResult)
		}
		return true
	})
	duration = time.Since(start)
	log.Info().Int64("durationNs", duration.Nanoseconds()).
		Int64("durationMs", duration.Milliseconds()).
		Msg("Get media from badger")
	return results
}

func searchMediaByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	additionalFilters := r.URL.Query().Get("additionalFilters")
	log.Info().Msgf("Searching media for user: %s", username)
	startTime := time.Now()
	if username == "" {
		http.Error(w, "Missing username", http.StatusBadRequest)
		return
	}

	results := getHardcoverBooksByUsername(username, additionalFilters)
	if results == nil {
		http.Error(w, "Failed to search media", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(results)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	totalDuration := time.Since(startTime)
	log.Info().Int("results", len(results)).
		Int("mediaCount", len(results)).
		Int64("durationMs", totalDuration.Milliseconds()).
		Int64("durationNs", totalDuration.Nanoseconds()).
		Msg("Search media by username full request")
}
