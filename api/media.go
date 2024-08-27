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
	"os"
	"strconv"
	"strings"
)

type MediaCreator struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	SortName string `json:"sortName"`
}

type Media struct {
	Id          uint64         `json:"id"`
	Title       string         `json:"title"`
	Creators    []MediaCreator `json:"creators"`
	Languages   []string       `json:"languages"`
	CoverUrl    string         `json:"coverUrl"`
	Formats     []string       `json:"formats"`
	Subtitle    string         `json:"subtitle"`
	Description string         `json:"description"`
}

var allMedia []Media
var mediaMap map[uint64]Media

func readMedia() {
	mediaMap = make(map[uint64]Media)
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
		mediaId, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			log.Error().Err(err)
		}
		media := Media{
			Id:          mediaId,
			Title:       record[1],
			Creators:    creators,
			Languages:   languages,
			CoverUrl:    record[4],
			Formats:     formats,
			Subtitle:    record[6],
			Description: record[7],
		}
		allMedia = append(allMedia, media)
		mediaMap[mediaId] = media
		var builder strings.Builder
		for _, creator := range creators {
			builder.WriteString(creator.Name)
		}
		for _, language := range languages {
			builder.WriteString(language)
		}
		builder.WriteString(media.Title)
		search.Index(builder.String(), mediaId)
	}
	log.Info().Msg("done reading media")
}
