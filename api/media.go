package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	Id               uint32
	TitleStart       uint32
	Creators         []MediaCreator
	CoverUrlStart    uint32
	SubtitleStart    uint32
	DescriptionStart uint32
	SeriesStart      uint32
	SeriesReadOrder  uint16
}

var mediaMap *MediaMap
var stringContainer *StringContainer

type MediaMap struct {
	sync.RWMutex
	m map[uint32]*Media
}

func NewMediaMap() *MediaMap {
	return &MediaMap{
		m: make(map[uint32]*Media),
	}
}

func (mm *MediaMap) Set(key uint32, value *Media) {
	mm.Lock()
	defer mm.Unlock()
	mm.m[key] = value
}

func (mm *MediaMap) Get(key uint32) (*Media, bool) {
	mm.RLock()
	defer mm.RUnlock()
	media, ok := mm.m[key]
	return media, ok
}

func (mm *MediaMap) Len() int {
	mm.RLock()
	defer mm.RUnlock()
	return len(mm.m)
}

func readMedia() {
	mediaMap = NewMediaMap()
	var err error
	stringContainer, err = NewStringContainer("media_strings")
	if err != nil {
		log.Error().Err(err)
	}
	languageMap = sync.Map{}
	formatMap = sync.Map{}
	startTime := time.Now()
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
			stringContainer.Flush()
			duration := time.Since(startTime)
			avgTimePerRecord := duration.Nanoseconds() / int64(count)
			log.Info().Msgf("worker%d read %d media; avgTimePerRecord(ns): %d", 0, count, avgTimePerRecord)
		}
	}
	gzr.Close()
	search.Finalize()
	stringContainer.Flush()
	log.Info().Msgf("string container used: %d", stringContainer.currentOffset)
	// TODO probably do this a better way
	dataLoaded = true
	log.Info().Msg("done reading media")
}

func handleRecord(record []string) {
	var creators []MediaCreator
	err := json.Unmarshal([]byte(record[2]), &creators)
	if err != nil {
		log.Error().Err(err)
	}
	languages := strings.Split(record[3], ";")
	formats := strings.Split(record[5], ";")
	mediaId, err := strconv.ParseUint(record[0], 10, 32)
	if mediaId > math.MaxUint32 {
		log.Warn().Msgf("mediaId too large %d", mediaId)
	}
	if err != nil {
		panic(err)
	}
	seriesReadOrder, err := strconv.Atoi(record[9])
	if err != nil {
		log.Error().Err(err)
	}
	titleStart := stringContainer.Add(record[1])
	log.Trace().Str("title", record[1]).
		Str("startLength", fmt.Sprintf("start: %d length: %d", titleStart)).
		Msg("indexing media")
	coverUrlStart := stringContainer.Add(record[4])
	descriptionStart := stringContainer.Add(record[7])

	media := &Media{
		Id:               uint32(mediaId),
		TitleStart:       titleStart,
		Creators:         creators,
		CoverUrlStart:    coverUrlStart,
		DescriptionStart: descriptionStart,
	}
	if record[6] != "" {
		subtitleStart := stringContainer.Add(record[6])
		media.SubtitleStart = subtitleStart
	}
	if record[8] != "" {
		seriesStart := stringContainer.Add(record[8])
		media.SeriesStart = seriesStart
		media.SeriesReadOrder = uint16(seriesReadOrder)
	}
	mediaMap.Set(media.Id, media)

	indexMedia(media, languages, formats)
}

func indexMedia(media *Media, languages []string, formats []string) {
	indexStrings(languages, &languageMap, media.Id)
	indexStrings(formats, &formatMap, media.Id)
	title := stringContainer.Get(media.TitleStart)
	log.Trace().Str("title", title).Msg("indexing media")
	search.Index(" "+title+" ", media.Id)
	if media.SeriesStart != 0 {
		seriesName := stringContainer.Get(media.SeriesStart)
		search.Index(fmt.Sprintf("#%d", media.SeriesReadOrder), media.Id)
		search.Index(" "+seriesName+" ", media.Id)
	}
	for _, creator := range media.Creators {
		search.Index(" "+creator.Name+" ", media.Id)
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
