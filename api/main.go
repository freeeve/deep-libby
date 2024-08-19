package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/NYTimes/gziphandler"
	"github.com/allegro/bigcache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type UiStatic struct {
	Body        []byte
	ContentType string
}

var uiCache *bigcache.BigCache
var s3Client *s3.Client

func main() {
	if os.Getenv("LOCAL_TESTING") == "true" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"})
	}
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Info().Msg("reading initial data")
	go readLibraries()
	go readAvailability()
	// this is the slowest one, let it block the server start
	readMedia()

	rootServeMux := http.NewServeMux()
	uiServeMux := http.NewServeMux()
	// uiServeMux.Handle("GET /", gziphandler.GzipHandler(http.HandlerFunc(uiHandler)))
	uiServeMux.Handle("GET /", http.HandlerFunc(uiHandler))

	apiServeMux := http.NewServeMux()
	apiServeMux.Handle("GET /api/search", gziphandler.GzipHandler(http.HandlerFunc(searchHandler)))
	apiServeMux.Handle("GET /api/libraries", gziphandler.GzipHandler(http.HandlerFunc(librariesHandler)))
	apiServeMux.Handle("GET /api/availability", gziphandler.GzipHandler(http.HandlerFunc(availabilityHandler)))
	apiServeMux.Handle("GET /api/diff", gziphandler.GzipHandler(http.HandlerFunc(diffHandler)))
	apiServeMux.Handle("GET /api/intersect", gziphandler.GzipHandler(http.HandlerFunc(intersectHandler)))

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
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	uiPrefix := "ui"
	var err error
	if uiCache == nil {
		uiCache, err = bigcache.NewBigCache(bigcache.DefaultConfig(30 * time.Minute))
		if err != nil {
			log.Error().Err(err)
		}
	}
	path := r.URL.Path
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if strings.Contains(acceptEncoding, "gzip") {
		acceptEncoding = "gzip"
	} else {
		acceptEncoding = "none"
	}
	if path == "/" {
		path = "/index.html"
	}
	log.Info().Str("path", path).Msg("serving ui")
	if getFromUICache(w, path, acceptEncoding) {
		return
	}
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
		if strings.Contains(err.Error(), "NoSuchKey") {
			resp, err = s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
				Bucket: aws.String("deep-libby"),
				Key:    aws.String(uiPrefix + "/index.html"),
			})
		}
	}
	var buf *bytes.Buffer
	buf = new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		log.Error().Err(err)
	}
	contentType := *resp.ContentType
	addToUICache(contentType, buf, path)
	getFromUICache(w, path, acceptEncoding)
}

func addToUICache(contentType string, responseBody *bytes.Buffer, path string) {
	var cacheBuffer *bytes.Buffer
	cacheBuffer = new(bytes.Buffer)
	bytesBuf := responseBody.Bytes()
	var gzipBuffer *bytes.Buffer
	gzipBuffer = new(bytes.Buffer)
	gzw := gzip.NewWriter(gzipBuffer)
	_, err := gzw.Write(bytesBuf)
	if err != nil {
		return
	}
	err = gzw.Flush()
	if err != nil {
		log.Error().Err(err)
	}
	err = gob.NewEncoder(cacheBuffer).Encode(UiStatic{ContentType: contentType, Body: bytesBuf})
	if err != nil {
		log.Error().Err(err)
	}
	err = uiCache.Set(path+"~none", cacheBuffer.Bytes())
	if err != nil {
		log.Error().Err(err)
	}
	cacheBuffer.Reset()
	err = gob.NewEncoder(cacheBuffer).Encode(UiStatic{ContentType: contentType, Body: gzipBuffer.Bytes()})
	if err != nil {
		log.Error().Err(err)
	}
	err = uiCache.Set(path+"~gzip", cacheBuffer.Bytes())
	if err != nil {
		log.Error().Err(err)
	}
}

func getFromUICache(w http.ResponseWriter, path, acceptEncoding string) bool {
	start := time.Now()
	cached, err := uiCache.Get(path + "~" + acceptEncoding)
	if err == nil {
		var cachedStatic UiStatic
		err := gob.NewDecoder(bytes.NewReader(cached)).Decode(&cachedStatic)
		if err != nil {
			log.Error().Err(err)
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300")
			w.Header().Set("Content-Type", cachedStatic.ContentType)
			if acceptEncoding == "gzip" {
				w.Header().Set("Content-Encoding", "gzip")
			} else {
				w.Header().Set("Content-Encoding", "none")
			}
			log.Info().Int("bodyLength", len(cachedStatic.Body)).Msg("getFromUICache before write to response")
			_, err = w.Write(cachedStatic.Body)
			if err != nil {
				log.Error().Err(err)
			} else {
				// early return for cache hit
				log.Info().Msg("returning early, cache hit")
				log.Info().Str("path", path).Dur("duration(ms)", time.Duration(time.Since(start).Milliseconds())).Msg("getFromUICache")
				return true
			}
		}
	} else {
		log.Debug().Msg("cache miss?")
	}
	return false
}

func getS3Client() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Error().Err(err)
	}
	s3Client = s3.NewFromConfig(cfg)
}
