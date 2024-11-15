package main

import (
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dgraph-io/badger/v4"
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

var formatStringMap = map[string]uint8{}
var formatReverseMap = map[uint8]string{}

func readAvailability() {
	if onDiskSize, _ := db.EstimateSize(nil); onDiskSize > 10000 {
		log.Info().Msg("availability already loaded")
		db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = []byte("fmt")
			iter := txn.NewIterator(opts)
			defer iter.Close()
			for iter.Rewind(); iter.Valid(); iter.Next() {
				item := iter.Item()
				err := item.Value(func(val []byte) error {
					formatStringMap[string(item.Key()[3])] = val[0]
					formatReverseMap[val[0]] = string(item.Key()[3])
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		return
	}

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
	writeBatch := db.NewWriteBatch()
	count := 0
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
					err := writeBatch.Set(getFormatKey(formatInt), []byte(format))
					if err != nil {
						log.Err(err)
					}
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
		// pack ints into byte array
		packedBytes := make([]byte, 8+len(formats))
		binary.BigEndian.PutUint16(packedBytes, mediaCounts.OwnedCount)
		binary.BigEndian.PutUint16(packedBytes, mediaCounts.AvailableCount)
		binary.BigEndian.PutUint16(packedBytes, mediaCounts.HoldsCount)
		binary.BigEndian.PutUint16(packedBytes, uint16(mediaCounts.EstimatedWaitDays))
		maKey := getMediaAvailabilityKey(id, libraryIdInt)
		laKey := getLibraryAvailabilityKey(libraryIdInt, id)
		for i, format := range formats {
			packedBytes[i+8] = format
		}
		if id == 7349338 {
			log.Debug().Msgf("writing to badger. key: %x, mediaCounts: %v", maKey, mediaCounts)
			log.Debug().Msgf("writing to badger. key: %x, mediaCounts: %v", laKey, mediaCounts)
		}
		err = writeBatch.Set(maKey, packedBytes)
		if err != nil {
			log.Err(err)
		}
		err = writeBatch.Set(laKey, packedBytes)
		if err != nil {
			log.Err(err)
		}
		count++
		if count%10000000 == 0 {
			log.Info().Msgf("read %dM availability records", count/1000000)
		}
	}
	err := writeBatch.Flush()
	if err != nil {
		log.Err(err)
	}
	gzr.Close()
	log.Info().Msg("done reading availability")
}

func getFormatKey(formatInt uint8) []byte {
	formatKey := make([]byte, 4)
	formatKey[0] = 'f'
	formatKey[1] = 'm'
	formatKey[2] = 't'
	formatKey[3] = formatInt
	return formatKey
}

func getMediaAvailabilityKey(id uint64, libraryIdInt uint16) []byte {
	mediaAvailabilityKey := make([]byte, 8)
	mediaAvailabilityKey[0] = 'm'
	mediaAvailabilityKey[1] = 'a'
	binary.BigEndian.PutUint32(mediaAvailabilityKey[2:], uint32(id))
	binary.BigEndian.PutUint16(mediaAvailabilityKey[6:], libraryIdInt)
	return mediaAvailabilityKey
}

func getLibraryAvailabilityKey(libraryIdInt uint16, id uint64) []byte {
	libraryAvailabilityKey := make([]byte, 8)
	libraryAvailabilityKey[0] = 'l'
	libraryAvailabilityKey[1] = 'a'
	binary.BigEndian.PutUint16(libraryAvailabilityKey[2:], libraryIdInt)
	binary.BigEndian.PutUint32(libraryAvailabilityKey[4:], uint32(id))
	return libraryAvailabilityKey
}

func getMediaAvailabilityPrefix(mediaId uint32) []byte {
	prefix := make([]byte, 6)
	prefix[0] = 'm'
	prefix[1] = 'a'
	binary.BigEndian.PutUint32(prefix[2:], mediaId)
	return prefix
}

func getLibraryAvailabilityPrefix(libraryId uint16) []byte {
	prefix := make([]byte, 4)
	prefix[0] = 'l'
	prefix[1] = 'a'
	binary.BigEndian.PutUint16(prefix[2:], libraryId)
	return prefix
}

func availabilityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	media, _ := mediaMap.Get(uint32(id))
	log.Info().Msgf("/api/availability media: %v", NewSearchResult(media))
	var results []LibraryMediaCounts
	err = db.View(func(txn *badger.Txn) error {
		prefix := getMediaAvailabilityPrefix(uint32(id))
		log.Info().Msgf("availability using prefix: %x", prefix)
		opt := badger.DefaultIteratorOptions
		opt.Prefix = prefix
		iter := txn.NewIterator(opt)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			log.Trace().Msgf("found item with key: %x", item.Key()) // Log the key
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				libraryId := binary.BigEndian.Uint16(item.Key()[6:])
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				library, exists := libraryMap[libraryId]
				if !exists {
					log.Error().Msgf("library not found for library id %d", libraryId)
					return nil
				}
				if library.Id == "uskindle" {
					return nil
				}
				results = append(results, LibraryMediaCounts{
					Library:           library,
					MediaCountResults: NewMediaCountResults(counts),
				})
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
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
	log.Info().Msgf("/api/diff left: %s right: %s", leftLibrary.Id, rightLibrary.Id)

	leftCounts := map[uint32]*MediaCounts{}
	err := db.View(func(txn *badger.Txn) error {
		prefix := getLibraryAvailabilityPrefix(leftLibraryIdInt)
		opt := badger.DefaultIteratorOptions
		opt.Prefix = prefix
		iter := txn.NewIterator(opt)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				mediaId := binary.BigEndian.Uint32(item.Key()[4:])
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				leftCounts[mediaId] = counts
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Err(err)
	}
	rightCounts := map[uint32]*MediaCounts{}
	err = db.View(func(txn *badger.Txn) error {
		prefix := getLibraryAvailabilityPrefix(rightLibraryIdInt)
		iter := txn.NewIterator(badger.DefaultIteratorOptions)
		defer iter.Close()
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				mediaId := binary.BigEndian.Uint32(item.Key()[4:])
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				rightCounts[mediaId] = counts
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Err(err)
	}
	log.Info().Msgf("diff, left: %d, right: %d", len(leftCounts), len(rightCounts))
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
	log.Info().Msg("hash diff complete")
	diffResponse := DiffResponse{
		Diff: diff,
	}
	// TODO paginate this
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(diffResponse)
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
	leftMedia := map[uint32]*MediaCounts{}
	err := db.View(func(txn *badger.Txn) error {
		prefix := getLibraryAvailabilityPrefix(leftLibraryIdInt)
		opt := badger.DefaultIteratorOptions
		opt.Prefix = prefix
		iter := txn.NewIterator(opt)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				mediaId := binary.BigEndian.Uint32(item.Key()[4:])
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				leftMedia[mediaId] = counts
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Err(err)
	}
	rightMedia := map[uint32]*MediaCounts{}
	err = db.View(func(txn *badger.Txn) error {
		prefix := getLibraryAvailabilityPrefix(rightLibraryIdInt)
		opt := badger.DefaultIteratorOptions
		opt.Prefix = prefix
		iter := txn.NewIterator(opt)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				mediaId := binary.BigEndian.Uint32(item.Key()[4:])
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				rightMedia[mediaId] = counts
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Err(err)
	}
	log.Info().Msgf("intersect, left: %d, right: %d", len(leftMedia), len(rightMedia))
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
	log.Info().Msg("hash intersect complete")
	if len(intersect) == 0 {
		intersect = []IntersectMediaCounts{}
	}
	diffResponse := IntersectResponse{
		Intersect: intersect,
	}
	// TODO paginate this
	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(diffResponse)
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
	var unique []UniqueMediaCounts
	media := map[uint32]*MediaCounts{}
	db.View(func(txn *badger.Txn) error {
		prefix := getLibraryAvailabilityPrefix(libraryIdInt)
		opt := badger.DefaultIteratorOptions
		opt.Prefix = prefix
		iter := txn.NewIterator(opt)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			mediaId := binary.BigEndian.Uint32(iter.Item().Key()[4:])
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				availabilityBytes := val
				counts := &MediaCounts{
					OwnedCount:        binary.BigEndian.Uint16(availabilityBytes[0:2]),
					AvailableCount:    binary.BigEndian.Uint16(availabilityBytes[2:4]),
					HoldsCount:        binary.BigEndian.Uint16(availabilityBytes[4:6]),
					EstimatedWaitDays: int16(binary.BigEndian.Uint16(availabilityBytes[6:8])),
					Formats:           availabilityBytes[8:],
				}
				media[mediaId] = counts
				return nil
			})
			if err != nil {
				return err
			}
		}
		log.Info().Msg("starting unique search")
		for mediaId, count := range media {
			mediaPrefix := getMediaAvailabilityPrefix(mediaId)
			opt := badger.DefaultIteratorOptions
			opt.Prefix = mediaPrefix
			iter := txn.NewIterator(opt)
			defer iter.Close()
			countIterations := 0
			for iter.Rewind(); iter.Valid(); iter.Next() {
				countIterations++
			}
			if countIterations == 1 {
				mediaRecord, _ := mediaMap.Get(mediaId)
				unique = append(unique, UniqueMediaCounts{
					SearchResult: NewSearchResult(mediaRecord),
					MediaCounts:  count,
				})
			}
		}
		return nil
	})
	if len(unique) == 0 {
		unique = []UniqueMediaCounts{}
	}
	log.Info().Msg("returning unique response")
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
