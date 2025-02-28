package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
)

type HardcoverGraphQLRequest struct {
	Query string `json:"query"`
}

type HardcoverUserSearchResponse struct {
	Data struct {
		Search struct {
			Results struct {
				Hits []struct {
					Document struct {
						ID string `json:"id"`
					} `json:"document"`
				} `json:"hits"`
			} `json:"results"`
		} `json:"search"`
	} `json:"data"`
}

type HardcoverBooksResponse struct {
	Data struct {
		Books []struct {
			Book struct {
				ID       int    `json:"id"`
				Title    string `json:"title"`
				Editions []struct {
					ISBN10        *string `json:"isbn_10"`
					ISBN13        *string `json:"isbn_13"`
					EditionFormat *string `json:"edition_format"`
				} `json:"editions"`
				CachedContributors []struct {
					Author struct {
						Slug        string `json:"slug"`
						Name        string `json:"name"`
						CachedImage struct {
							ID        int    `json:"id"`
							URL       string `json:"url"`
							Color     string `json:"color"`
							Width     int    `json:"width"`
							Height    int    `json:"height"`
							ColorName string `json:"color_name"`
						} `json:"cachedImage"`
					} `json:"author"`
					Contribution *string `json:"contribution"`
				} `json:"cached_contributors"`
			} `json:"book"`
		} `json:"books"`
	} `json:"data"`
}

func getUserId(userName string) string {
	buf, err := json.Marshal(HardcoverGraphQLRequest{
		Query: fmt.Sprintf("query UsersWithUsername {search(query: \"%s\", query_type: \"User\",per_page: 5,page: 1) {results}}", userName),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal GraphQL request")
		return ""
	}

	req, err := http.NewRequest("POST", "https://api.hardcover.app/v1/graphql", bytes.NewBuffer(buf))
	if err != nil {
		log.Error().Err(err).Msg("failed to create HTTP request")
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("HARDCOVER_API_TOKEN")))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to post GraphQL request")
		return ""
	}
	defer resp.Body.Close()

	var result HardcoverUserSearchResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Error().Err(err).Msg("failed to decode GraphQL response")
		return ""
	}

	if len(result.Data.Search.Results.Hits) > 0 {
		return result.Data.Search.Results.Hits[0].Document.ID
	}

	log.Error().Msg("user not found")
	return ""
}

func getHardcoverBooksWithIsbns(userId string) []map[string]interface{} {
	buf, err := json.Marshal(HardcoverGraphQLRequest{
		Query: fmt.Sprintf("query MyBooks { books: user_books(where: {status_id: {_eq: 1}, user_id: {_eq: \"%s\"}}) {book {id title editions {isbn_10 isbn_13 edition_format} cached_contributors }}}", userId),
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
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to post GraphQL request")
		return nil
	}
	defer resp.Body.Close()

	var result HardcoverBooksResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Error().Err(err).Msg("failed to decode GraphQL response")
		return nil
	}

	log.Info().Msgf("GraphQL response: %v", result)

	if len(result.Data.Books) == 0 {
		log.Warn().Msg("no books found for the user")
		return nil
	}

	bookDetails := make([]map[string]interface{}, len(result.Data.Books))
	for i, book := range result.Data.Books {
		formats := []string{}
		isbns := []string{}
		authors := []string{}
		for _, edition := range book.Book.Editions {
			if edition.EditionFormat != nil && *edition.EditionFormat != "Hardcover" && *edition.EditionFormat != "Paperback" {
				formats = append(formats, *edition.EditionFormat)
			}
			if edition.ISBN10 != nil {
				isbns = append(isbns, *edition.ISBN10)
			}
			if edition.ISBN13 != nil {
				isbns = append(isbns, *edition.ISBN13)
			}
		}
		for _, contributor := range book.Book.CachedContributors {
			if contributor.Author.Name != "" {
				authors = append(authors, contributor.Author.Name)
			}
		}
		bookDetails[i] = map[string]interface{}{
			"id":      book.Book.ID,
			"title":   book.Book.Title,
			"formats": formats,
			"isbns":   isbns,
			"authors": authors,
		}
	}

	return bookDetails
}

func searchMediaByIsbns(isbns []string, additionalFilters string) []*SearchResult {
	resultMap := make(map[uint32]*SearchResult)
	for _, isbn := range isbns {
		ids := search.Search(isbn + " " + additionalFilters)
		for _, id := range ids {
			media, err := getMedia(id)
			if err != nil {
				log.Error().Err(err).Msgf("failed to get media for id: %d", id)
				continue
			}
			searchResult := NewSearchResult(media)
			resultMap[media.Id] = searchResult
		}
	}
	results := make([]*SearchResult, 0, len(resultMap))
	for _, result := range resultMap {
		results = append(results, result)
	}
	return results
}

func searchMediaByUsername(username, additionalFilters string) []*SearchResult {
	userId := getUserId(username)
	if userId == "" {
		log.Error().Msg("failed to get user ID")
		return nil
	}
	log.Info().Msgf("User ID: %s", userId)

	books := getHardcoverBooksWithIsbns(userId)
	if books == nil {
		log.Error().Msg("failed to get books")
		return nil
	}
	log.Info().Msgf("Books: %v", books)

	var isbns []string
	for _, book := range books {
		for _, isbn := range book["isbns"].([]string) {
			isbns = append(isbns, isbn)
		}
	}

	return searchMediaByIsbns(isbns, additionalFilters)
}

func searchMediaByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	additionalFilters := r.URL.Query().Get("additionalFilters")
	log.Info().Msgf("Searching media for user: %s", username)
	if username == "" {
		http.Error(w, "Missing username", http.StatusBadRequest)
		return
	}

	results := searchMediaByUsername(username, additionalFilters)
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
}
