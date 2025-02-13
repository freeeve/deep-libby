package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MediaCreator struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	SortName string `json:"sortName"`
}

var formatMap sync.Map
var languageMap sync.Map

type Media struct {
	Id              uint32
	Title           string
	Languages       []string
	Creators        []MediaCreator
	Publisher       string
	PublisherId     uint32
	Formats         []string
	CoverUrl        string
	Subtitle        string
	Description     string
	Series          string
	SeriesReadOrder uint16
	Ids             []string
}

func getMediaKey(mediaId uint32) []byte {
	return append([]byte("mk"), []byte(strconv.Itoa(int(mediaId)))...)
}

func readMedia() {
	loadDone := false
	if onDiskSize, _ := db.EstimateSize(nil); onDiskSize > 10000 {
		log.Info().Msg("media already loaded")
		if os.Getenv("LOAD_ONLY") == "true" {
			log.Info().Msg("load only mode, running load anyway")
		} else {
			loadDone = true
		}
	}
	var err error
	if err != nil {
		log.Error().Err(err)
	}
	languageMap = sync.Map{}
	formatMap = sync.Map{}
	startTime := time.Now()
	if !loadDone {
		var gzr *gzip.Reader
		if os.Getenv("LOCAL_TESTING") == "true" {
			f, err := os.Open("../../librarylibrary/media.csv.gz")
			defer f.Close()
			if err != nil {
				log.Error().Err(err)
			}
			gzr, err = gzip.NewReader(f)
			if err != nil {
				log.Error().Err(err)
			}
		} else {
			s3Path := "media.csv.gz"
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
		var count int
		for {
			record, err := cr.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Error().Err(err)
			}
			handleRecord(record)
			count++
			if count%100000 == 0 {
				duration := time.Since(startTime)
				avgTimePerRecord := duration.Nanoseconds() / int64(count)
				log.Info().Msgf("worker%d read %d media; avgTimePerRecord(ns): %d", 0, count, avgTimePerRecord)
			}
		}
		gzr.Close()
	}

	// index media
	log.Info().Msg("indexing media")
	db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("mk")
		iter := txn.NewIterator(opts)
		defer iter.Close()
		count := 0
		for iter.Rewind(); iter.ValidForPrefix([]byte("mk")); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				media := &Media{}
				err := gob.NewDecoder(bytes.NewReader(val)).Decode(media)
				if err != nil {
					log.Error().Err(err)
				}
				indexMedia(media)
				return nil
			})
			if err != nil {
				return err
			}
			count++
			if count%1000000 == 0 {
				log.Info().Msgf("indexed %d media", count)
			}
		}
		return nil
	})
	search.Finalize()
	log.Info().Msg("done reading media")
}

func handleRecord(record []string) *Media {
	var creators []MediaCreator
	err := json.Unmarshal([]byte(record[2]), &creators)
	if err != nil {
		log.Error().Err(err)
	}
	languages := strings.Split(record[3], ";")
	formats := strings.Split(record[5], ";")
	identifiers := strings.Split(record[10], ";")
	mediaId, err := strconv.ParseUint(record[0], 10, 32)
	if mediaId > math.MaxUint32 {
		log.Warn().Msgf("mediaId too large %d", mediaId)
	}
	if err != nil {
		panic(err)
	}
	publisherId, err := strconv.ParseUint(record[12], 10, 32)
	if err != nil {
		panic(err)
	}
	seriesReadOrder, err := strconv.Atoi(record[9])
	if err != nil {
		log.Error().Err(err)
	}
	media := &Media{
		Id:              uint32(mediaId),
		Title:           record[1],
		Creators:        creators,
		Publisher:       record[11],
		PublisherId:     uint32(publisherId),
		Languages:       languages,
		CoverUrl:        record[4],
		Formats:         formats,
		Subtitle:        record[6],
		Description:     record[7],
		Series:          record[8],
		SeriesReadOrder: uint16(seriesReadOrder),
		Ids:             identifiers,
	}
	buf := bytes.Buffer{}
	err = gob.NewEncoder(&buf).Encode(media)
	if err != nil {
		panic(err)
	}
	// insert into db
	txn := db.NewTransaction(true)
	err = txn.Set(getMediaKey(media.Id), buf.Bytes())
	if err != nil {
		panic(err)
	}
	err = txn.Commit()
	if err != nil {
		panic(err)
	}
	//log.Debug().Msgf("mediaId: %d %v", media.Id, identifiers)
	return media
}

func getMedia(mediaId uint32) (*Media, error) {
	txn := db.NewTransaction(false)
	buf, err := txn.Get(getMediaKey(mediaId))
	if err != nil {
		return nil, err
	}
	media := &Media{}
	err = buf.Value(func(val []byte) error {
		return gob.NewDecoder(bytes.NewReader(val)).Decode(media)
	})
	txn.Discard()
	if err != nil {
		return nil, err
	}
	return media, nil
}

func indexMedia(media *Media) {
	indexStrings(media.Languages, &languageMap, media.Id)
	indexStrings(media.Formats, &formatMap, media.Id)
	search.Index(" "+media.Title+" ", media.Id)
	search.Index(" "+media.Publisher+" ", media.Id)
	search.Index(fmt.Sprintf(" %s-%d ", media.Publisher, media.PublisherId), media.Id)
	if media.Series != "" {
		search.Index(fmt.Sprintf("#%d", media.SeriesReadOrder), media.Id)
		search.Index(" "+media.Series+" ", media.Id)
	}
	for _, creator := range media.Creators {
		search.Index(" "+creator.Name+" ", media.Id)
	}
	for _, identifier := range media.Ids {
		search.Index(" "+identifier+" ", media.Id)
	}
}

func indexStrings(stringSlice []string, bitmapMap *sync.Map, mediaId uint32) {
	for _, str := range stringSlice {
		bitmap, bitmapExists := bitmapMap.Load(strings.ToLower(str))
		if !bitmapExists {
			bitmap = &ConcurrentBitmap{
				bitmap: roaring.New(),
			}
			bitmapMap.Store(strings.ToLower(str), bitmap)
		}
		bitmap.(*ConcurrentBitmap).Add(mediaId)
		search.Index(strings.ToLower(str), mediaId)
	}
}
