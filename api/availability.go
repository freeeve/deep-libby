package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"strconv"
)

type MediaCounts struct {
	OwnedCount        uint32 `json:"ownedCount"`
	AvailableCount    uint32 `json:"availableCount"`
	HoldsCount        uint32 `json:"holdsCount"`
	EstimatedWaitDays int32  `json:"estimatedWaitDays"`
}

type LibraryMediaCounts struct {
	Library Library `json:"library"`
	MediaCounts
}

type AvailabilityResponse struct {
	Media
	Availability []LibraryMediaCounts `json:"availability"`
}

type DiffResponse struct {
	Diff []DiffMediaCounts `json:"diff"`
}

type DiffMediaCounts struct {
	Media
	LibraryMediaCounts
}

type IntersectResponse struct {
	Intersect []IntersectMediaCounts `json:"intersect"`
}

type IntersectMediaCounts struct {
	Media
	LeftLibraryMediaCounts  LibraryMediaCounts `json:"leftLibraryMediaCounts"`
	RightLibraryMediaCounts LibraryMediaCounts `json:"rightLibraryMediaCounts"`
}

var availabilityMap map[uint64]map[int]MediaCounts
var libraryMediaMap map[int]map[uint64]MediaCounts

func readAvailability() {
	availabilityMap = make(map[uint64]map[int]MediaCounts)
	libraryMediaMap = make(map[int]map[uint64]MediaCounts)
	var gzr *gzip.Reader
	if os.Getenv("LOCAL_TESTING") == "true" {
		f, err := os.Open("../../librarylibrary/availability.csv.gz")
		if err != nil {
			log.Error().Err(err)
		}
		gzr, err = gzip.NewReader(f)
		if err != nil {
			log.Error().Err(err)
		}
	} else {
		s3Path := "availability.csv.gz"
		if s3Client == nil {
			getS3Client()
		}
		resp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String("deep-libby"),
			Key:    aws.String(s3Path),
		})
		if err != nil {
			log.Error().Err(err)
		}
		defer resp.Body.Close()
		gzr, err = gzip.NewReader(resp.Body)
		if err != nil {
			log.Error().Err(err)
		}
	}
	cr := csv.NewReader(gzr)
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error().Err(err)
		}
		id, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			log.Error().Err(err)
		}
		websiteId, err := strconv.Atoi(record[1])
		if err != nil {
			log.Error().Err(err)
		}
		ownedCount, err := strconv.ParseUint(record[2], 10, 32)
		if err != nil {
			log.Error().Err(err)
		}
		availableCount, err := strconv.ParseUint(record[3], 10, 32)
		if err != nil {
			log.Error().Err(err)
		}
		holdsCount, err := strconv.ParseUint(record[4], 10, 16)
		if err != nil {
			log.Error().Err(err)
		}
		estimatedWaitDays, err := strconv.ParseInt(record[5], 10, 32)
		if err != nil {
			log.Error().Err(err)
		}
		if _, exists := availabilityMap[id]; !exists {
			availabilityMap[id] = map[int]MediaCounts{}
		}
		estimatedWaitDays = estimatedWaitDays
		if availableCount > holdsCount {
			estimatedWaitDays = 0
		}
		mediaCounts := MediaCounts{
			OwnedCount:        uint32(ownedCount),
			AvailableCount:    uint32(availableCount),
			HoldsCount:        uint32(holdsCount),
			EstimatedWaitDays: int32(estimatedWaitDays),
		}
		availabilityMap[id][websiteId] = mediaCounts
		if _, exists := libraryMediaMap[websiteId]; !exists {
			libraryMediaMap[websiteId] = map[uint64]MediaCounts{}
		}
		libraryMediaMap[websiteId][id] = mediaCounts
	}
	log.Info().Msg("done reading availability")
}

func availabilityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	results := []LibraryMediaCounts{}
	for websiteId, counts := range availabilityMap[id] {
		library, exists := libraryMap[websiteId]
		if !exists {
			log.Error().Msgf("library not found for website id %d", websiteId)
			continue
		}
		if library.Id == "uskindle" {
			continue
		}
		results = append(results, LibraryMediaCounts{
			Library:     library,
			MediaCounts: counts,
		})
	}
	availability := AvailabilityResponse{
		Media:        mediaMap[id],
		Availability: results,
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(availability)
	if err != nil {
		log.Error().Err(err)
	}
}

func diffHandler(w http.ResponseWriter, r *http.Request) {
	leftWebsiteId, err := strconv.Atoi(r.URL.Query().Get("leftWebsiteId"))
	if err != nil {
		http.Error(w, "invalid leftWebsiteId", http.StatusBadRequest)
		return
	}
	rightWebsiteId, err := strconv.Atoi(r.URL.Query().Get("rightWebsiteId"))
	if err != nil {
		http.Error(w, "invalid rightWebsiteId", http.StatusBadRequest)
		return
	}
	leftLibrary, leftExists := libraryMap[leftWebsiteId]
	rightLibrary, rightExists := libraryMap[rightWebsiteId]
	if !leftExists || !rightExists {
		http.Error(w, "invalid website id", http.StatusBadRequest)
		return
	}
	leftCounts := libraryMediaMap[leftLibrary.WebsiteId]
	rightCounts := libraryMediaMap[rightLibrary.WebsiteId]
	diff := []DiffMediaCounts{}
	for id, leftCount := range leftCounts {
		_, exists := rightCounts[id]
		if !exists {
			diff = append(diff, DiffMediaCounts{
				Media: mediaMap[id],
				LibraryMediaCounts: LibraryMediaCounts{
					Library:     leftLibrary,
					MediaCounts: leftCount,
				},
			})
		}
	}
	diffResponse := DiffResponse{
		Diff: diff,
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(diffResponse)
	if err != nil {
		log.Error().Err(err)
	}
}

func intersectHandler(w http.ResponseWriter, r *http.Request) {
	leftWebsiteId, err := strconv.Atoi(r.URL.Query().Get("leftWebsiteId"))
	if err != nil {
		http.Error(w, "invalid leftWebsiteId", http.StatusBadRequest)
		return
	}
	rightWebsiteId, err := strconv.Atoi(r.URL.Query().Get("rightWebsiteId"))
	if err != nil {
		http.Error(w, "invalid rightWebsiteId", http.StatusBadRequest)
		return
	}
	leftLibrary, leftExists := libraryMap[leftWebsiteId]
	rightLibrary, rightExists := libraryMap[rightWebsiteId]
	if !leftExists || !rightExists {
		http.Error(w, "invalid website id", http.StatusBadRequest)
		return
	}
	leftMedia := libraryMediaMap[leftLibrary.WebsiteId]
	rightMedia := libraryMediaMap[rightLibrary.WebsiteId]
	var intersect []IntersectMediaCounts
	for id, leftCount := range leftMedia {
		rightCount, exists := rightMedia[id]
		if exists {
			intersect = append(intersect, IntersectMediaCounts{
				Media:                   mediaMap[id],
				LeftLibraryMediaCounts:  LibraryMediaCounts{leftLibrary, leftCount},
				RightLibraryMediaCounts: LibraryMediaCounts{rightLibrary, rightCount},
			})
		}
	}
	diffResponse := IntersectResponse{
		Intersect: intersect,
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(diffResponse)
	if err != nil {
		log.Error().Err(err)
	}
}
