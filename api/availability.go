package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strconv"
)

type BookCounts struct {
	OwnedCount        uint32
	AvailableCount    uint32
	HoldsCount        uint16
	EstimatedWaitDays int32
}

var availabilityMap map[uint64]map[uint16]BookCounts

func readAvailability() {
	availabilityMap = make(map[uint64]map[uint16]BookCounts)
	s3Path := "availability.csv.gz"
	if s3Client == nil {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			log.Error().Err(err)
		}
		s3Client = s3.NewFromConfig(cfg)
	}
	resp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("deep-libby"),
		Key:    aws.String(s3Path),
	})
	if err != nil {
		log.Error().Err(err)
	}
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
		id, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			log.Error().Err(err)
		}
		websiteId, err := strconv.ParseUint(record[1], 10, 16)
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
			availabilityMap[id] = map[uint16]BookCounts{}
		}
		availabilityMap[id][uint16(websiteId)] = BookCounts{
			OwnedCount:        uint32(ownedCount),
			AvailableCount:    uint32(availableCount),
			HoldsCount:        uint16(holdsCount),
			EstimatedWaitDays: int32(estimatedWaitDays),
		}
	}
}

func availabilityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	websiteId, err := strconv.ParseUint(r.URL.Query().Get("websiteId"), 10, 16)
	if err != nil {
		http.Error(w, "invalid websiteId", http.StatusBadRequest)
		return
	}
	availability, exists := availabilityMap[id][uint16(websiteId)]
	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(availability)
	if err != nil {
		log.Error().Err(err)
	}
}
