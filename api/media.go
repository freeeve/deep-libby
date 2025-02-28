package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/RoaringBitmap/roaring"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dgraph-io/badger/v4"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/rs/zerolog/log"
	"io"
	"math"
	"net/http"
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

type Identifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type overdriveBulkResponse struct {
	ReserveId string `json:"reserveId"`
	Subjects  []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"subjects"`
	BisacCodes []string `json:"bisacCodes"`
	Bisac      []struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"bisac"`
	Levels   []interface{} `json:"levels"`
	Creators []struct {
		Id       int    `json:"id"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		SortName string `json:"sortName"`
	} `json:"creators"`
	Languages []struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"languages"`
	Imprint struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"imprint"`
	IsBundledChild bool `json:"isBundledChild"`
	Ratings        struct {
		MaturityLevel struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"maturityLevel"`
		NaughtyScore struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"naughtyScore"`
	} `json:"ratings"`
	Constraints struct {
		IsDisneyEulaRequired bool `json:"isDisneyEulaRequired"`
	} `json:"constraints"`
	ReviewCounts struct {
		Premium           int `json:"premium"`
		PublisherSupplier int `json:"publisherSupplier"`
	} `json:"reviewCounts"`
	Subtitle                   string `json:"subtitle"`
	IsPublicDomain             bool   `json:"isPublicDomain"`
	IsPublicPerformanceAllowed bool   `json:"isPublicPerformanceAllowed"`
	Publisher                  struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"publisher"`
	Popularity       int      `json:"popularity"`
	ShortDescription string   `json:"shortDescription"`
	FullDescription  string   `json:"fullDescription"`
	Description      string   `json:"description"`
	Keywords         []string `json:"keywords"`
	UnitsSold        int      `json:"unitsSold"`
	IsBundleChild    bool     `json:"isBundleChild"`
	Sample           struct {
		Href string `json:"href"`
	} `json:"sample"`
	IsPreReleaseTitle              bool          `json:"isPreReleaseTitle"`
	EstimatedReleaseDate           time.Time     `json:"estimatedReleaseDate"`
	VisitorEligible                bool          `json:"visitorEligible"`
	JuvenileEligible               bool          `json:"juvenileEligible"`
	YoungAdultEligible             bool          `json:"youngAdultEligible"`
	BundledContentChildrenTitleIds []interface{} `json:"bundledContentChildrenTitleIds"`
	Classifications                struct {
	} `json:"classifications"`
	Type struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"type"`
	Covers struct {
		Cover150Wide struct {
			Href         string `json:"href"`
			Height       int    `json:"height"`
			Width        int    `json:"width"`
			PrimaryColor struct {
				Hex string `json:"hex"`
				Rgb struct {
					Red   int `json:"red"`
					Green int `json:"green"`
					Blue  int `json:"blue"`
				} `json:"rgb"`
			} `json:"primaryColor"`
			IsPlaceholderImage bool `json:"isPlaceholderImage"`
		} `json:"cover150Wide"`
		Cover300Wide struct {
			Href         string `json:"href"`
			Height       int    `json:"height"`
			Width        int    `json:"width"`
			PrimaryColor struct {
				Hex string `json:"hex"`
				Rgb struct {
					Red   int `json:"red"`
					Green int `json:"green"`
					Blue  int `json:"blue"`
				} `json:"rgb"`
			} `json:"primaryColor"`
			IsPlaceholderImage bool `json:"isPlaceholderImage"`
		} `json:"cover300Wide"`
		Cover510Wide struct {
			Href         string `json:"href"`
			Height       int    `json:"height"`
			Width        int    `json:"width"`
			PrimaryColor struct {
				Hex string `json:"hex"`
				Rgb struct {
					Red   int `json:"red"`
					Green int `json:"green"`
					Blue  int `json:"blue"`
				} `json:"rgb"`
			} `json:"primaryColor"`
			IsPlaceholderImage bool `json:"isPlaceholderImage"`
		} `json:"cover510Wide"`
	} `json:"covers"`
	Id                   string    `json:"id"`
	FirstCreatorName     string    `json:"firstCreatorName"`
	FirstCreatorId       int       `json:"firstCreatorId"`
	FirstCreatorSortName string    `json:"firstCreatorSortName"`
	Title                string    `json:"title"`
	SortTitle            string    `json:"sortTitle"`
	StarRating           float64   `json:"starRating"`
	StarRatingCount      int       `json:"starRatingCount"`
	PublishDate          time.Time `json:"publishDate"`
	PublishDateText      string    `json:"publishDateText"`
	Formats              []struct {
		Identifiers              []Identifier  `json:"identifiers"`
		Rights                   []interface{} `json:"rights"`
		OnSaleDateUtc            time.Time     `json:"onSaleDateUtc"`
		HasAudioSynchronizedText bool          `json:"hasAudioSynchronizedText"`
		IsBundleParent           bool          `json:"isBundleParent"`
		BundledContent           []interface{} `json:"bundledContent"`
		FulfillmentType          string        `json:"fulfillmentType"`
		Id                       string        `json:"id"`
		Name                     string        `json:"name"`
		Isbn                     string        `json:"isbn,omitempty"`
		Sample                   struct {
			Href string `json:"href"`
		} `json:"sample,omitempty"`
		FileSize int `json:"fileSize,omitempty"`
	} `json:"formats"`
	PublisherAccount struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	} `json:"publisherAccount"`
	DetailedSeries struct {
		SeriesId     int    `json:"seriesId"`
		SeriesName   string `json:"seriesName"`
		ReadingOrder string `json:"readingOrder"`
		Rank         int    `json:"rank"`
	} `json:"detailedSeries"`
	Series string `json:"series"`
}

type Media struct {
	Id              uint32
	Title           string
	Languages       []string
	Creators        []MediaCreator
	Publisher       string
	PublisherId     uint32
	Formats         []string
	CoverUrl        string
	Subtitle        string
	Description     string
	Series          string
	SeriesReadOrder uint16
	Ids             []string
}

func getMediaKey(mediaId uint32) []byte {
	return append([]byte("mk"), []byte(strconv.Itoa(int(mediaId)))...)
}

func getAllMedia() {
	log.Info().Msg("starting getAllMedia")
	idsPerQuery := 100
	db, err := sql.Open("duckdb", "media.db")
	if err != nil {
		log.Error().Err(err).Msg("failed to open database")
		return
	}
	defer db.Close()

	_, err = db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS media (
        id INTEGER primary key,
        title STRING,
        creators JSON,
        publisher STRING,
        publisherId INTEGER,
        publishDate DATE,
        languages STRING,
        coverUrl STRING,
        formats JSON,
        subtitle STRING,
        description STRING,
        series STRING,
        seriesReadOrder STRING)`)
	if err != nil {
		log.Error().Err(err).Msg("failed to create table")
		return
	}

	for i := 1; i < 100000000; i += idsPerQuery {
		time.Sleep(1 * time.Second)
		mediaIds := strings.Builder{}
		for j := 0; j < idsPerQuery; j++ {
			mediaIds.WriteString(strconv.Itoa(i + j))
			mediaIds.WriteString(",")
		}
		log.Debug().Int("idStart", i).Int("idEnd", i+idsPerQuery).Msg("getting ids getAllMedia")
		url := fmt.Sprintf("https://thunder.api.overdrive.com/v2/media/bulk?titleIds=%s&x-client-id=dewey", mediaIds.String())
		resp, err := http.Get(url)
		if err != nil {
			log.Error().Err(err).Msg("failed to get response from API")
			i -= idsPerQuery
			continue
		}
		defer resp.Body.Close()
		var overdriveBulkResponses []overdriveBulkResponse
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msg("failed to read response")
			i -= idsPerQuery
			continue
		}
		if strings.Contains(string(buf), "Media not found.") || strings.Contains(string(buf), "An unexpected error has occurred") {
			log.Info().Msg("media not found for range")
			continue
		}
		err = json.Unmarshal(buf, &overdriveBulkResponses)
		if err != nil {
			log.Error().Err(err).Str("body", string(buf)).Msg("failed to decode response")
			i -= idsPerQuery
			continue
		}

		insertStmt, err := db.PrepareContext(context.Background(), `INSERT INTO media
            (id, title, creators, publisher, publisherId, languages, publishDate, coverUrl, formats, subtitle, description, series, seriesReadOrder)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			log.Error().Err(err).Msg("failed to prepare insert statement")
			return
		}
		if len(overdriveBulkResponses) == 0 {
			log.Info().Msg("no media found")
			break
		}
		for _, overdriveBulkResponse := range overdriveBulkResponses {
			languages := ""
			for _, language := range overdriveBulkResponse.Languages {
				languages += language.Name + ";"
			}
			creatorsJSON, err := json.Marshal(overdriveBulkResponse.Creators)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal creators")
				continue
			}
			formatsJSON, err := json.Marshal(overdriveBulkResponse.Formats)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal formats")
				continue
			}

			_, err = insertStmt.ExecContext(context.Background(),
				overdriveBulkResponse.Id,
				overdriveBulkResponse.Title,
				string(creatorsJSON),
				overdriveBulkResponse.Publisher.Name,
				overdriveBulkResponse.Publisher.Id,
				languages,
				overdriveBulkResponse.PublishDate,
				overdriveBulkResponse.Covers.Cover150Wide.Href,
				string(formatsJSON),
				overdriveBulkResponse.Subtitle,
				overdriveBulkResponse.Description,
				overdriveBulkResponse.Series,
				overdriveBulkResponse.DetailedSeries.ReadingOrder,
			)
			if err != nil {
				log.Error().Err(err).Msg("failed to insert record")
			} else {
				log.Trace().Msgf("successfully inserted record with id: %d", overdriveBulkResponse.Id)
			}
		}
	}
	log.Info().Msg("done getAllMedia")
}

func getAllMediaIndividually() {
	log.Info().Msg("starting getAllMediaIndividually")
	db, err := sql.Open("duckdb", "media.db")
	if err != nil {
		log.Error().Err(err).Msg("failed to open database")
		return
	}
	defer db.Close()

	fetched := false
	for i := 7686277; i < 100000000; i += 1 {
		if fetched {
			time.Sleep(100 * time.Millisecond)
			fetched = false
		}
		res, err := db.QueryContext(context.Background(), `select * from media where id = ?`, i)
		if err != nil {
			log.Error().Err(err).Msg("failed to query db")
			continue
		}
		if res.Next() {
			log.Trace().Msgf("media with id %d already exists", i)
			continue
		}
		url := fmt.Sprintf("https://thunder.api.overdrive.com/v2/media/bulk?titleIds=%d&x-client-id=dewey", i)
		log.Debug().Int("id", i).Msg("getting id getAllMediaIndividually")
		resp, err := http.Get(url)
		fetched = true
		if err != nil {
			log.Error().Err(err).Msg("failed to get response from API")
			i -= 1
			continue
		}
		var overdriveBulkResponses []overdriveBulkResponse
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Err(err).Msg("failed to read response")
			i -= 1
			continue
		}
		resp.Body.Close()

		if strings.Contains(string(buf), "Media not found.") {
			log.Debug().Msg("media not found")
			continue
		}
		if strings.Contains(string(buf), "An unexpected error has occurred") {
			log.Info().Msg("An unexpected error has occurred")
			continue
		}
		err = json.Unmarshal(buf, &overdriveBulkResponses)
		if err != nil {
			log.Error().Err(err).Str("body", string(buf)).Msg("failed to decode response")
			i -= 1
			continue
		}

		insertStmt, err := db.PrepareContext(context.Background(), `INSERT INTO media
            (id, title, creators, publisher, publisherId, languages, publishDate, coverUrl, formats, subtitle, description, series, seriesReadOrder)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			log.Error().Err(err).Msg("failed to prepare insert statement")
			return
		}
		if len(overdriveBulkResponses) == 0 {
			log.Info().Msg("no media found")
			insertStmt.Close()
			break
		}
		for _, overdriveBulkResponse := range overdriveBulkResponses {
			languages := ""
			for _, language := range overdriveBulkResponse.Languages {
				languages += language.Name + ";"
			}
			creatorsJSON, err := json.Marshal(overdriveBulkResponse.Creators)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal creators")
				continue
			}
			formatsJSON, err := json.Marshal(overdriveBulkResponse.Formats)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal formats")
				continue
			}

			_, err = insertStmt.ExecContext(context.Background(),
				overdriveBulkResponse.Id,
				overdriveBulkResponse.Title,
				string(creatorsJSON),
				overdriveBulkResponse.Publisher.Name,
				overdriveBulkResponse.Publisher.Id,
				languages,
				overdriveBulkResponse.PublishDate,
				overdriveBulkResponse.Covers.Cover150Wide.Href,
				string(formatsJSON),
				overdriveBulkResponse.Subtitle,
				overdriveBulkResponse.Description,
				overdriveBulkResponse.Series,
				overdriveBulkResponse.DetailedSeries.ReadingOrder,
			)
			if err != nil {
				log.Error().Err(err).Msg("failed to insert record")
			} else {
				log.Trace().Msgf("successfully inserted record with id: %d", overdriveBulkResponse.Id)
			}
		}
		insertStmt.Close()
	}
	log.Info().Msg("done getAllMedia")
}

func readMedia() {
	loadDone := false
	if onDiskSize, _ := db.EstimateSize(nil); onDiskSize > 10000 {
		log.Info().Msg("media already loaded")
		if os.Getenv("LOAD_ONLY") == "true" {
			log.Info().Msg("load only mode, running load anyway")
		} else {
			loadDone = true
		}
	}
	var err error
	if err != nil {
		log.Error().Err(err)
	}
	languageMap = sync.Map{}
	formatMap = sync.Map{}
	startTime := time.Now()
	if !loadDone {
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
				duration := time.Since(startTime)
				avgTimePerRecord := duration.Nanoseconds() / int64(count)
				log.Info().Msgf("worker%d read %d media; avgTimePerRecord(ns): %d", 0, count, avgTimePerRecord)
			}
		}
		gzr.Close()
	}

	// index media
	log.Info().Msg("indexing media")
	db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("mk")
		iter := txn.NewIterator(opts)
		defer iter.Close()
		count := 0
		for iter.Rewind(); iter.ValidForPrefix([]byte("mk")); iter.Next() {
			item := iter.Item()
			err := item.Value(func(val []byte) error {
				media := &Media{}
				err := gob.NewDecoder(bytes.NewReader(val)).Decode(media)
				if err != nil {
					log.Error().Err(err)
				}
				indexMedia(media)
				return nil
			})
			if err != nil {
				return err
			}
			count++
			if count%1000000 == 0 {
				log.Info().Msgf("indexed %d media", count)
			}
		}
		return nil
	})
	search.Finalize()
	log.Info().Msg("done reading media")
}

func handleRecord(record []string) *Media {
	var creators []MediaCreator
	err := json.Unmarshal([]byte(record[2]), &creators)
	if err != nil {
		log.Error().Err(err)
	}
	languages := strings.Split(record[3], ";")
	formats := strings.Split(record[5], ";")
	identifiers := strings.Split(record[10], ";")
	mediaId, err := strconv.ParseUint(record[0], 10, 32)
	if mediaId > math.MaxUint32 {
		log.Warn().Msgf("mediaId too large %d", mediaId)
	}
	if err != nil {
		panic(err)
	}
	publisherId, err := strconv.ParseUint(record[12], 10, 32)
	if err != nil {
		panic(err)
	}
	seriesReadOrder, err := strconv.Atoi(record[9])
	if err != nil {
		log.Error().Err(err)
	}
	media := &Media{
		Id:              uint32(mediaId),
		Title:           record[1],
		Creators:        creators,
		Publisher:       record[11],
		PublisherId:     uint32(publisherId),
		Languages:       languages,
		CoverUrl:        record[4],
		Formats:         formats,
		Subtitle:        record[6],
		Description:     record[7],
		Series:          record[8],
		SeriesReadOrder: uint16(seriesReadOrder),
		Ids:             identifiers,
	}
	buf := bytes.Buffer{}
	err = gob.NewEncoder(&buf).Encode(media)
	if err != nil {
		panic(err)
	}
	// insert into db
	txn := db.NewTransaction(true)
	err = txn.Set(getMediaKey(media.Id), buf.Bytes())
	if err != nil {
		panic(err)
	}
	err = txn.Commit()
	if err != nil {
		panic(err)
	}
	//log.Debug().Msgf("mediaId: %d %v", media.Id, identifiers)
	return media
}

func getMedia(mediaId uint32) (*Media, error) {
	txn := db.NewTransaction(false)
	buf, err := txn.Get(getMediaKey(mediaId))
	if err != nil {
		return nil, err
	}
	media := &Media{}
	err = buf.Value(func(val []byte) error {
		return gob.NewDecoder(bytes.NewReader(val)).Decode(media)
	})
	txn.Discard()
	if err != nil {
		return nil, err
	}
	return media, nil
}

func indexMedia(media *Media) {
	indexStrings(media.Languages, &languageMap, media.Id)
	indexStrings(media.Formats, &formatMap, media.Id)
	search.Index(" "+media.Title+" ", media.Id)
	search.Index(" "+media.Subtitle+" ", media.Id)
	search.Index(" "+media.Publisher+" ", media.Id)
	search.Index(fmt.Sprintf(" %s-%d ", media.Publisher, media.PublisherId), media.Id)
	if media.Series != "" {
		search.Index(fmt.Sprintf("#%d", media.SeriesReadOrder), media.Id)
		search.Index(" "+media.Series+" ", media.Id)
	}
	for _, creator := range media.Creators {
		search.Index(" "+creator.Name+" ", media.Id)
	}
	for _, identifier := range media.Ids {
		search.Index(" "+identifier+" ", media.Id)
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
