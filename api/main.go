package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/allegro/bigcache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"time"
)

var uiCache *bigcache.BigCache
var s3Client *s3.Client

func main() {
	log.Info().Msg("reading initial data")
	readMedia()
	//readLibraries()
	readAvailability()
	if os.Getenv("LOCAL_TESTING") == "true" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	rootServeMux := http.NewServeMux()
	uiServeMux := http.NewServeMux()
	uiServeMux.HandleFunc("GET /ui/", uiHandler)

	apiServeMux := http.NewServeMux()
	apiServeMux.HandleFunc("GET /api/search", searchHandler)
	apiServeMux.HandleFunc("GET /api/availability", availabilityHandler)

	rootServeMux.Handle("/ui/", uiServeMux)
	rootServeMux.Handle("/api/", apiServeMux)

	port := "8080"
	log.Info().Str("port", port).Msg("starting server")
	err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), rootServeMux)
	if err != nil {
		log.Fatal().Err(err)
	}
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	uiPrefix := "ui/"
	var err error
	if uiCache == nil {
		uiCache, err = bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))
		if err != nil {
			log.Error().Err(err)
		}
	}
	path := r.URL.Path
	cached, err := uiCache.Get(path)
	if err == nil {
		_, err = w.Write(cached)
		if err != nil {
			log.Error().Err(err)
		} else {
			// early return for cache hit
			return
		}
	} else {
		log.Debug().Msg("cache miss?")
	}
	if s3Client == nil {
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			log.Error().Err(err)
		}
		s3Client = s3.NewFromConfig(cfg)
	}
	resp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("deep-libby"),
		Key:    aws.String(uiPrefix + path),
	})
	var buf *bytes.Buffer
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		log.Error().Err(err)
	}
	_, err = io.Copy(w, buf)
	if err != nil {
		log.Error().Err(err)
	}
	err = uiCache.Set(path, buf.Bytes())
	if err != nil {
		log.Error().Err(err)
	}
}
