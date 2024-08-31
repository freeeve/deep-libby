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
	*Media
	Availability []LibraryMediaCounts `json:"availability"`
}

type DiffResponse struct {
	Diff []DiffMediaCounts `json:"diff"`
}

type DiffMediaCounts struct {
	*Media
	LibraryMediaCounts
}

type UniqueResponse struct {
	Library Library             `json:"library"`
	Unique  []UniqueMediaCounts `json:"unique"`
}

type UniqueMediaCounts struct {
	*Media
	MediaCounts
}

type IntersectResponse struct {
	Intersect []IntersectMediaCounts `json:"intersect"`
}

type IntersectMediaCounts struct {
	*Media
	LeftLibraryMediaCounts  LibraryMediaCounts `json:"leftLibraryMediaCounts"`
	RightLibraryMediaCounts LibraryMediaCounts `json:"rightLibraryMediaCounts"`
}

var availabilityMap map[uint32]map[string]MediaCounts
var libraryMediaMap map[string]map[uint32]MediaCounts

func readAvailability() {
	availabilityMap = make(map[uint32]map[string]MediaCounts)
	libraryMediaMap = make(map[string]map[uint32]MediaCounts)
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
		id, err := strconv.ParseUint(record[0], 10, 32)
		if err != nil {
			log.Error().Err(err)
		}
		libraryId := record[1]
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
		if _, exists := availabilityMap[uint32(id)]; !exists {
			availabilityMap[uint32(id)] = map[string]MediaCounts{}
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
		availabilityMap[uint32(id)][libraryId] = mediaCounts
		if _, exists := libraryMediaMap[libraryId]; !exists {
			libraryMediaMap[libraryId] = map[uint32]MediaCounts{}
		}
		libraryMediaMap[libraryId][uint32(id)] = mediaCounts
	}
	log.Info().Msg("done reading availability")
}

func availabilityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	media, _ := mediaMap.Get(uint32(id))
	log.Info().Msgf("/api/availability media: %v", media)
	var results []LibraryMediaCounts
	for libraryId, counts := range availabilityMap[uint32(id)] {
		library, exists := libraryMap[libraryId]
		if !exists {
			log.Error().Msgf("library not found for library id %s", libraryId)
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
	media.LibraryCount = len(availabilityMap[uint32(id)])
	availability := AvailabilityResponse{
		Media:        media,
		Availability: results,
	}
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(availability)
	if err != nil {
		log.Error().Err(err)
	}
}

func diffHandler(w http.ResponseWriter, r *http.Request) {
	leftLibraryId := r.URL.Query().Get("leftLibraryId")
	rightLibraryId := r.URL.Query().Get("rightLibraryId")
	leftLibrary, leftExists := libraryMap[leftLibraryId]
	rightLibrary, rightExists := libraryMap[rightLibraryId]
	if !leftExists || !rightExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/intersect left: %s right: %s", leftLibrary.Id, rightLibrary.Id)
	leftCounts := libraryMediaMap[leftLibraryId]
	rightCounts := libraryMediaMap[rightLibraryId]
	diff := []DiffMediaCounts{}
	for id, leftCount := range leftCounts {
		_, exists := rightCounts[id]
		if !exists {
			mediaRecord, _ := mediaMap.Get(id)
			diff = append(diff, DiffMediaCounts{
				Media: mediaRecord,
				LibraryMediaCounts: LibraryMediaCounts{
					Library:     leftLibrary,
					MediaCounts: leftCount,
				},
			})
		}
	}
	if len(diff) == 0 {
		diff = []DiffMediaCounts{}
	}
	diffResponse := DiffResponse{
		Diff: diff,
	}
	// TODO paginate this
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(diffResponse)
	if err != nil {
		log.Error().Err(err)
	}
}

func intersectHandler(w http.ResponseWriter, r *http.Request) {
	leftLibraryId := r.URL.Query().Get("leftLibraryId")
	rightLibraryId := r.URL.Query().Get("rightLibraryId")
	leftLibrary, leftExists := libraryMap[leftLibraryId]
	rightLibrary, rightExists := libraryMap[rightLibraryId]
	if !leftExists || !rightExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/intersect left: %s right: %s", leftLibrary.Id, rightLibrary.Id)
	leftMedia := libraryMediaMap[leftLibraryId]
	rightMedia := libraryMediaMap[rightLibraryId]
	var intersect []IntersectMediaCounts
	for id, leftCount := range leftMedia {
		rightCount, exists := rightMedia[id]
		if exists {
			media, _ := mediaMap.Get(id)
			intersect = append(intersect, IntersectMediaCounts{
				Media:                   media,
				LeftLibraryMediaCounts:  LibraryMediaCounts{leftLibrary, leftCount},
				RightLibraryMediaCounts: LibraryMediaCounts{rightLibrary, rightCount},
			})
		}
	}
	if len(intersect) == 0 {
		intersect = []IntersectMediaCounts{}
	}
	diffResponse := IntersectResponse{
		Intersect: intersect,
	}

	// TODO paginate this
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(diffResponse)
	if err != nil {
		log.Error().Err(err)
	}
}

func uniqueHandler(w http.ResponseWriter, r *http.Request) {
	libraryId := r.URL.Query().Get("libraryId")
	library, libraryExists := libraryMap[libraryId]
	if !libraryExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/unique libraryId %s", library.Id)
	media := libraryMediaMap[libraryId]
	var unique []UniqueMediaCounts
	for id, count := range media {
		if len(availabilityMap[id]) == 1 {
			mediaRecord, _ := mediaMap.Get(id)
			unique = append(unique, UniqueMediaCounts{
				Media:       mediaRecord,
				MediaCounts: count,
			})
		}
	}
	if len(unique) == 0 {
		unique = []UniqueMediaCounts{}
	}
	diffResponse := UniqueResponse{
		Library: library,
		Unique:  unique,
	}

	// TODO paginate this
	w.Header().Add("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(diffResponse)
	if err != nil {
		log.Error().Err(err)
	}
}
