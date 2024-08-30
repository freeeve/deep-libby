package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
	"io"
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

type Media struct {
	Id              uint64         `json:"id"`
	Title           string         `json:"title"`
	Creators        []MediaCreator `json:"creators"`
	Languages       []string       `json:"languages"`
	CoverUrl        string         `json:"coverUrl"`
	Formats         []string       `json:"formats"`
	Subtitle        string         `json:"subtitle"`
	Description     string         `json:"description"`
	SeriesName      string         `json:"seriesName"`
	SeriesReadOrder int            `json:"seriesReadOrder"`
	SearchString    string         `json:"searchString"`
	LibraryCount    int            `json:"libraryCount"`
}

var allMedia []*Media
var mediaMap *MediaMap

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

func readMedia() {
	mediaMap = NewMediaMap()
	startTime := time.Now()
	var gzr *gzip.Reader
	if os.Getenv("LOCAL_TESTING") == "true" {
		f, err := os.Open("../../librarylibrary/media.csv.gz")
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
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error().Err(err)
		}
		var creators []MediaCreator
		err = json.Unmarshal([]byte(record[2]), &creators)
		if err != nil {
			log.Error().Err(err)
		}
		languages := strings.Split(record[3], ";")
		formats := strings.Split(record[5], ";")
		mediaId, err := strconv.ParseUint(record[0], 10, 32)
		if err != nil {
			panic(err)
		}
		seriesReadOrder, err := strconv.Atoi(record[9])
		if err != nil {
			log.Error().Err(err)
		}
		media := Media{
			Id:              mediaId,
			Title:           record[1],
			Creators:        creators,
			Languages:       languages,
			CoverUrl:        record[4],
			Formats:         formats,
			Subtitle:        record[6],
			Description:     record[7],
			SeriesName:      record[8],
			SeriesReadOrder: seriesReadOrder,
		}
		allMedia = append(allMedia, &media)
		mediaMap.Set(uint32(mediaId), &media)
		var builder strings.Builder
		for _, creator := range creators {
			builder.WriteString(creator.Name)
		}
		for _, language := range languages {
			builder.WriteString(language)
		}
		for _, format := range formats {
			builder.WriteString(format)
		}
		builder.WriteString(media.Title)
		if media.SeriesName != "" {
			builder.WriteString(fmt.Sprintf("#%d", seriesReadOrder))
			builder.WriteString(media.SeriesName)
		}
		search.Index(builder.String(), mediaId)
		media.SearchString = strings.ToLower(builder.String())
		if len(allMedia)%100000 == 0 {
			duration := time.Since(startTime)
			avgTimePerRecord := duration.Nanoseconds() / int64(len(allMedia))
			log.Debug().Msgf("read %d media; avgTimePerRecord(ns): %d", len(allMedia), avgTimePerRecord)
		}
	}
	// TODO probably do this a better way
	dataLoaded = true
	log.Info().Msg("done reading media")
}
