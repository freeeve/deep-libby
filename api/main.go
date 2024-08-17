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
	if os.Getenv("LOCAL_TESTING") == "true" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"})
	}
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Info().Msg("reading initial data")
	// this is the slowest one, let it block the server start

	rootServeMux := http.NewServeMux()
	uiServeMux := http.NewServeMux()
	uiServeMux.HandleFunc("GET /", uiHandler)

	apiServeMux := http.NewServeMux()
	apiServeMux.HandleFunc("GET /api/search", searchHandler)
	apiServeMux.HandleFunc("GET /api/libraries", librariesHandler)
	apiServeMux.HandleFunc("GET /api/availability", availabilityHandler)
	apiServeMux.HandleFunc("GET /api/diff", diffHandler)
	apiServeMux.HandleFunc("GET /api/intersect", intersectHandler)

	corsAPIMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// If it's a preflight request, respond immediately
		if r.Method == "OPTIONS" {
			return
		}

		// Otherwise, pass the request on to the original ServeMux
		apiServeMux.ServeHTTP(w, r)
	})

	rootServeMux.Handle("/", uiServeMux)
	rootServeMux.Handle("/api/", corsAPIMux)

	if os.Getenv("LOCAL_TESTING") == "true" {
		port := "8080"
		log.Info().Str("port", port).Msg("starting server")
		err := http.ListenAndServe(fmt.Sprintf("localhost:%s", port), rootServeMux)
		if err != nil {
			log.Fatal().Err(err)
		}
	} else {
		port := "443"
		log.Info().Str("port", port).Msg("starting server")
		privKey := "/etc/letsencrypt/live/deeplibby.com/privkey.pem"
		certFile := "/etc/letsencrypt/live/deeplibby.com/fullchain.pem"
		err := http.ListenAndServeTLS(fmt.Sprintf("0.0.0.0:%s", port), certFile, privKey, rootServeMux)
		if err != nil {
			log.Fatal().Err(err)
		}
	}

	go readLibraries()
	go readAvailability()
	readMedia()
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	uiPrefix := "ui"
	var err error
	if uiCache == nil {
		uiCache, err = bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))
		if err != nil {
			log.Error().Err(err)
		}
	}
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	log.Info().Str("path", path).Msg("serving ui")
	/*
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
	*/
	if s3Client == nil {
		getS3Client()
	}
	key := uiPrefix + path
	log.Info().Str("key", key).Msg("reading s3")
	resp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("deep-libby"),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Error().Err(err)
	}
	if resp == nil {
		log.Error().Msg("empty body")
		log.Error().Msg(err.Error())
	}
	var buf *bytes.Buffer
	buf = new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		log.Error().Err(err)
	}
	w.Header().Set("Content-Type", *resp.ContentType)
	_, err = io.Copy(w, buf)
	if err != nil {
		log.Error().Err(err)
	}
	err = uiCache.Set(path, buf.Bytes())
	if err != nil {
		log.Error().Err(err)
	}
}

func getS3Client() {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Error().Err(err)
	}
	s3Client = s3.NewFromConfig(cfg)
}
