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
	"runtime"
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
type ConcurrentMediaSlice struct {
	sync.RWMutex
	slice []*Media
}

var allMedia = &ConcurrentMediaSlice{
	slice: make([]*Media, 0),
}

func (cms *ConcurrentMediaSlice) Add(media *Media) {
	cms.Lock()
	defer cms.Unlock()
	cms.slice = append(cms.slice, media)
}

func (cms *ConcurrentMediaSlice) Get(index int) (*Media, bool) {
	cms.RLock()
	defer cms.RUnlock()
	if index < 0 || index >= len(cms.slice) {
		return nil, false
	}
	return cms.slice[index], true
}

func (cms *ConcurrentMediaSlice) Len() int {
	cms.RLock()
	defer cms.RUnlock()
	return len(cms.slice)
}

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
	numCPUs := max(runtime.NumCPU()/2, 2)
	log.Info().Msgf("numCPUs: %d", numCPUs)
	var wg sync.WaitGroup
	records := make(chan []string, numCPUs*2)

	// Create a number of goroutines equal to the number of CPUs
	for i := 0; i < numCPUs; i++ {
		wg.Add(1)
		go func() {
			var count int
			var builder strings.Builder
			defer wg.Done()
			for record := range records {
				handleRecord(record, &builder)
				count++
				if count%100000 == 0 {
					duration := time.Since(startTime)
					avgTimePerRecord := duration.Nanoseconds() / int64(count)
					log.Info().Msgf("worker%d read %d media; avgTimePerRecord(ns): %d", i, count, avgTimePerRecord)
				}
			}
		}()
	}
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error().Err(err)
		}
		records <- record
	}
	close(records)
	wg.Wait()
	// TODO probably do this a better way
	dataLoaded = true
	log.Info().Msg("done reading media")
}

func handleRecord(record []string, builder *strings.Builder) {
	var creators []MediaCreator
	err := json.Unmarshal([]byte(record[2]), &creators)
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
	allMedia.Add(&media)
	mediaMap.Set(uint32(mediaId), &media)
	builder.Reset()
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
}
