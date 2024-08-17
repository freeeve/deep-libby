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
	"strconv"
)

type Library struct {
	Id           string `json:"id"`
	WebsiteId    int    `json:"websiteId"`
	Name         string `json:"name"`
	IsConsortium bool   `json:"isConsortium"`
}

type LibraryResponse struct {
	Libraries []Library `json:"libraries"`
}

var libraryMap map[int]Library

func readLibraries() {
	libraryMap = make(map[int]Library)
	s3Path := "libraries.csv.gz"
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
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Error().Err(err)
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
		websiteId, err := strconv.Atoi(record[1])
		if err != nil {
			log.Error().Err(err)
		}
		libraryMap[websiteId] = Library{
			Id:           record[0],
			WebsiteId:    websiteId,
			Name:         record[2],
			IsConsortium: record[3] == "true",
		}
	}
	log.Info().Msg("done reading libraries")
}

func librariesHandler(w http.ResponseWriter, r *http.Request) {
	if libraryMap == nil {
		readLibraries()
	}
	libraries := make([]Library, 0, len(libraryMap))
	for _, library := range libraryMap {
		libraries = append(libraries, library)
	}
	err := json.NewEncoder(w).Encode(LibraryResponse{libraries})
	if err != nil {
		log.Error().Err(err)
	}
}
