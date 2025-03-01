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
	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	stdlog "log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type UiStatic struct {
	Body        []byte
	ContentType string
}

var uiCache *bigcache.BigCache
var s3Client *s3.Client

const port = "443"

var dataLoaded = false
var db *badger.DB

func main() {
	var err error
	stdlog.SetFlags(0)
	stdlog.SetOutput(log.Logger)
	badgerOpts := badger.DefaultOptions("deeplibby.badger").
		WithLogger(nil)
	db, err = badger.Open(badgerOpts)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open BadgerDB")
	}
	defer db.Close()
	// go getAllMedia()
	// go getAllMediaIndividually()
	/*
		go func() {
			fmt.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/
	if os.Getenv("LOCAL_TESTING") == "true" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"})
		log.Logger = log.Level(zerolog.DebugLevel)
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000", NoColor: true})
		log.Logger = log.Level(zerolog.InfoLevel)
	}

	// handle graceful shutdown on SIGINT and SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)
	go func() {
		<-c
		log.Info().Msg("shutting down on interrupt...")
		db.Close()
		os.Exit(0)
	}()

	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Info().Msg("reading initial data")
	// this one is fast and libraryIds need to be loaded before availability/media
	readLibraries()
	if os.Getenv("LOAD_ONLY") == "true" {
		readMedia()
		readAvailability()
		log.Info().Msg("shutting down")
		os.Exit(0)
	}
	readMedia()
	readAvailability()

	rootServeMux := http.NewServeMux()
	uiServeMux := http.NewServeMux()
	uiServeMux.Handle("GET /", http.HandlerFunc(uiHandler))

	apiServeMux := http.NewServeMux()
	apiServeMux.Handle("GET /api/search", gziphandler.GzipHandler(http.HandlerFunc(searchHandler)))
	apiServeMux.Handle("GET /api/libraries", gziphandler.GzipHandler(http.HandlerFunc(librariesHandler)))
	apiServeMux.Handle("GET /api/availability", gziphandler.GzipHandler(http.HandlerFunc(availabilityHandler)))
	apiServeMux.Handle("GET /api/diff", gziphandler.GzipHandler(http.HandlerFunc(diffHandler)))
	apiServeMux.Handle("GET /api/intersect", gziphandler.GzipHandler(http.HandlerFunc(intersectHandler)))
	apiServeMux.Handle("GET /api/unique", gziphandler.GzipHandler(http.HandlerFunc(uniqueHandler)))
	apiServeMux.Handle("GET /api/memory", gziphandler.GzipHandler(http.HandlerFunc(memoryHandler)))
	apiServeMux.Handle("GET /api/search-debug", gziphandler.GzipHandler(http.HandlerFunc(searchDebugHandler)))
	apiServeMux.Handle("GET /api/search-hardcover", gziphandler.GzipHandler(http.HandlerFunc(searchMediaByUsernameHandler)))

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
		log.Info().Str("port", port).Msg("starting server")
		privKey := "/etc/letsencrypt/live/deeplibby.com/privkey.pem"
		certFile := "/etc/letsencrypt/live/deeplibby.com/fullchain.pem"
		err := http.ListenAndServeTLS(fmt.Sprintf("0.0.0.0:%s", port), certFile, privKey, rootServeMux)
		if err != nil {
			log.Fatal().Err(err)
		}
	}
}

func calculateMemoryUsage(v interface{}) int {
	b := new(bytes.Buffer)
	if err := gob.NewEncoder(b).Encode(v); err != nil {
		return 0
	}
	return b.Len()
}

func memoryHandler(writer http.ResponseWriter, request *http.Request) {
	log.Info().Msgf("Memory usage of libraryMap: %d bytes\n", calculateMemoryUsage(libraryMap))
	sum := uint64(0)
	formatMap.Range(func(key, value interface{}) bool {
		size := value.(*ConcurrentBitmap).UnsafeBitmap().GetSizeInBytes()
		if size > 1024*1024 {
			log.Info().Msgf("memory usage of formatMap[%s]: %d bytes\n", key, size)
		}
		sum += size
		return true
	})
	log.Info().Msgf("Memory usage of formatMap values: %d bytes\n", sum)
	sum = 0
	languageMap.Range(func(key, value interface{}) bool {
		size := value.(*ConcurrentBitmap).UnsafeBitmap().GetSizeInBytes()
		if size > 128*1024 {
			log.Info().Msgf("memory usage of languageMap[%s]: %d bytes\n", key, size)
		}
		sum += size
		return true
	})
	log.Info().Msgf("Memory usage of languageMap values: %d bytes\n", sum)
	sum = 0
	for ngram, bitmap := range search.ngramMap {
		size := bitmap.UnsafeBitmap().GetSizeInBytes()
		if size > 1024*1024 {
			log.Info().Msgf("memory usage of ngram[%s]: %d bytes\n", ngram, size)
		}
		sum += size
	}
	log.Info().Msgf("Memory usage (total) of search index: %d bytes\n", int(sum)+calculateMemoryUsage(search.ngramMap))
	runtime.GC()
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
	log.Trace().Str("path", path).Msg("serving ui")
	if getFromUICache(w, path, acceptEncoding) {
		return
	}
	if s3Client == nil {
		getS3Client()
	}
	key := uiPrefix + path
	log.Trace().Str("key", key).Msg("reading s3")
	resp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("deep-libby"),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Error().Err(err)
	}
	if resp == nil {
		log.Trace().Msg("empty body")
		log.Trace().Msg(err.Error())
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
	// all this cache hackiness is to make load time faster
	// we cache both the gzip and non-gzip versions of the files
	// TODO for some reason it is not expiring cache after 30 minutes
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
			log.Trace().Int("bodyLength", len(cachedStatic.Body)).Msg("getFromUICache before write to response")
			_, err = w.Write(cachedStatic.Body)
			if err != nil {
				log.Error().Err(err)
			} else {
				// early return for cache hit
				log.Trace().Msg("returning early, cache hit")
				log.Trace().Str("path", path).Dur("duration(ms)", time.Duration(time.Since(start).Milliseconds())).Msg("getFromUICache")
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
