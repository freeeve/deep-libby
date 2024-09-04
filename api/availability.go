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
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type MediaCounts struct {
	OwnedCount        uint16  `json:"ownedCount"`
	AvailableCount    uint16  `json:"availableCount"`
	HoldsCount        uint16  `json:"holdsCount"`
	EstimatedWaitDays int16   `json:"estimatedWaitDays"`
	Formats           []uint8 `json:"formats"`
}

type MediaCountResults struct {
	OwnedCount        uint16   `json:"ownedCount"`
	AvailableCount    uint16   `json:"availableCount"`
	HoldsCount        uint16   `json:"holdsCount"`
	EstimatedWaitDays int16    `json:"estimatedWaitDays"`
	Formats           []string `json:"formats"`
}

func NewMediaCountResults(mediaCounts *MediaCounts) MediaCountResults {
	var formats []string
	for _, format := range mediaCounts.Formats {
		formatInt, _ := formatReverseMap[format]
		formats = append(formats, formatInt)
	}
	return MediaCountResults{
		OwnedCount:        mediaCounts.OwnedCount,
		AvailableCount:    mediaCounts.AvailableCount,
		HoldsCount:        mediaCounts.HoldsCount,
		EstimatedWaitDays: mediaCounts.EstimatedWaitDays,
		Formats:           formats,
	}
}

type LibraryMediaCounts struct {
	Library Library `json:"library"`
	MediaCountResults
}

type AvailabilityResponse struct {
	*SearchResult
	Availability []LibraryMediaCounts `json:"availability"`
}

type DiffResponse struct {
	Diff []DiffMediaCounts `json:"diff"`
}

type DiffMediaCounts struct {
	*SearchResult
	LibraryMediaCounts
}

type UniqueResponse struct {
	Library Library             `json:"library"`
	Unique  []UniqueMediaCounts `json:"unique"`
}

type UniqueMediaCounts struct {
	*SearchResult
	*MediaCounts
}

type IntersectResponse struct {
	Intersect []IntersectMediaCounts `json:"intersect"`
}

type IntersectMediaCounts struct {
	*SearchResult
	LeftLibraryMediaCounts  LibraryMediaCounts `json:"leftLibraryMediaCounts"`
	RightLibraryMediaCounts LibraryMediaCounts `json:"rightLibraryMediaCounts"`
}

var availabilityMap map[uint32]map[uint16]*MediaCounts
var libraryMediaMap map[uint16]map[uint32]*MediaCounts
var formatStringMap = map[string]uint8{}
var formatReverseMap = map[uint8]string{}

func readAvailability() {
	availabilityMap = make(map[uint32]map[uint16]*MediaCounts)
	libraryMediaMap = make(map[uint16]map[uint32]*MediaCounts)
	var gzr *gzip.Reader
	if os.Getenv("LOCAL_TESTING") == "true" {
		f, err := os.Open("../../librarylibrary/availability.csv.gz")
		defer f.Close()
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
		libraryIdInt, exists := libraryIdMap[libraryId]
		if !exists {
			panic("library id not found" + libraryId)
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
		if _, exists := availabilityMap[uint32(id)]; !exists {
			availabilityMap[uint32(id)] = map[uint16]*MediaCounts{}
		}
		estimatedWaitDays = estimatedWaitDays
		if availableCount > holdsCount {
			estimatedWaitDays = 0
		}
		var formats []uint8
		if record[6] != "" {
			splitFormats := strings.Split(record[6], ";")
			for _, format := range splitFormats {
				formatInt, exists := formatStringMap[format]
				if !exists {
					formatInt = uint8(len(formatStringMap))
					formatStringMap[format] = formatInt
					formatReverseMap[formatInt] = format
				}
				formats = append(formats, formatInt)
			}
		}
		if ownedCount > math.MaxUint16 {
			log.Warn().Msgf("owned count %d is greater than max uint16", ownedCount)
			ownedCount = math.MaxUint16
		}
		if availableCount > math.MaxUint16 {
			log.Warn().Msgf("available count %d is greater than max uint16", availableCount)
			availableCount = math.MaxUint16
		}
		if holdsCount > math.MaxUint16 {
			log.Warn().Msgf("holds count %d is greater than max uint16", holdsCount)
			holdsCount = math.MaxUint16
		}
		if estimatedWaitDays > math.MaxInt16 {
			log.Warn().Msgf("estimated wait days %d is greater than max int16", estimatedWaitDays)
			estimatedWaitDays = math.MaxInt16
		}
		if estimatedWaitDays < math.MinInt16 {
			log.Warn().Msgf("estimated wait days %d is less than min int16", estimatedWaitDays)
			estimatedWaitDays = math.MinInt16
		}
		mediaCounts := &MediaCounts{
			OwnedCount:        uint16(ownedCount),
			AvailableCount:    uint16(availableCount),
			HoldsCount:        uint16(holdsCount),
			EstimatedWaitDays: int16(estimatedWaitDays),
			Formats:           formats,
		}
		availabilityMap[uint32(id)][libraryIdInt] = mediaCounts
		if _, exists := libraryMediaMap[libraryIdInt]; !exists {
			libraryMediaMap[libraryIdInt] = map[uint32]*MediaCounts{}
		}
		libraryMediaMap[libraryIdInt][uint32(id)] = mediaCounts
	}
	gzr.Close()
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
			Library:           library,
			MediaCountResults: NewMediaCountResults(counts),
		})
	}
	availability := AvailabilityResponse{
		SearchResult: NewSearchResult(media),
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
	leftLibraryIdInt := libraryIdMap[leftLibraryId]
	rightLibraryIdInt := libraryIdMap[rightLibraryId]
	leftLibrary, leftExists := libraryMap[leftLibraryIdInt]
	rightLibrary, rightExists := libraryMap[rightLibraryIdInt]
	if !leftExists || !rightExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/intersect left: %s right: %s", leftLibrary.Id, rightLibrary.Id)
	leftCounts := libraryMediaMap[leftLibraryIdInt]
	rightCounts := libraryMediaMap[rightLibraryIdInt]
	diff := []DiffMediaCounts{}
	for id, leftCount := range leftCounts {
		_, exists := rightCounts[id]
		if !exists {
			mediaRecord, _ := mediaMap.Get(id)
			diff = append(diff, DiffMediaCounts{
				SearchResult: NewSearchResult(mediaRecord),
				LibraryMediaCounts: LibraryMediaCounts{
					Library:           leftLibrary,
					MediaCountResults: NewMediaCountResults(leftCount),
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
	leftLibraryIdInt := libraryIdMap[leftLibraryId]
	rightLibraryIdInt := libraryIdMap[rightLibraryId]
	leftLibrary, leftExists := libraryMap[leftLibraryIdInt]
	rightLibrary, rightExists := libraryMap[rightLibraryIdInt]
	if !leftExists || !rightExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/intersect left: %s right: %s", leftLibrary.Id, rightLibrary.Id)
	leftMedia := libraryMediaMap[leftLibraryIdInt]
	rightMedia := libraryMediaMap[rightLibraryIdInt]
	var intersect []IntersectMediaCounts
	for id, leftCount := range leftMedia {
		rightCount, exists := rightMedia[id]
		if exists {
			media, _ := mediaMap.Get(id)
			intersect = append(intersect, IntersectMediaCounts{
				SearchResult: NewSearchResult(media),
				LeftLibraryMediaCounts: LibraryMediaCounts{
					leftLibrary,
					NewMediaCountResults(leftCount),
				},
				RightLibraryMediaCounts: LibraryMediaCounts{
					rightLibrary,
					NewMediaCountResults(rightCount),
				},
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
	libraryIdInt, exists := libraryIdMap[libraryId]
	if !exists {
		panic("library id not found" + libraryId)
	}
	library, libraryExists := libraryMap[libraryIdInt]
	if !libraryExists {
		http.Error(w, "invalid library id", http.StatusBadRequest)
		return
	}
	log.Info().Msgf("/api/unique libraryId %s", library.Id)
	media := libraryMediaMap[libraryIdInt]
	var unique []UniqueMediaCounts
	for id, count := range media {
		if len(availabilityMap[id]) == 1 {
			mediaRecord, _ := mediaMap.Get(id)
			unique = append(unique, UniqueMediaCounts{
				SearchResult: NewSearchResult(mediaRecord),
				MediaCounts:  count,
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
