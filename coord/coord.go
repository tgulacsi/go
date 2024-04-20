/*
Copyright 2019, 2024 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package coord contains a function to get the coordinates of
// a human-readable address, using GMaps.
package coord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/rogpeppe/retry"
	"golang.org/x/time/rate"
)

type urlTemplate struct {
	Base string
	addressOrder
}

var rZipCity = regexp.MustCompile("^ *[0-9]{4,} +[^ ]+ +")

func (ut urlTemplate) WithAddress(address string) string {
	if ut.addressOrder == littleEndian {
		if m := rZipCity.FindString(address); m != "" {
			address = address[len(m):] + " " + strings.TrimSpace(m)
		}
	}
	return strings.Replace(ut.Base, "{{.Address}}", url.QueryEscape(address), 1)
}

type addressOrder uint8

const (
	bigEndian addressOrder = iota
	littleEndian
)

var (
	gmapsURL     = urlTemplate{Base: `https://maps.googleapis.com/maps/api/geocode/json?key={{.APIKey}}&sensors=false&address={{.Address}}`}
	nominatimURL = urlTemplate{Base: `https://nominatim.openstreetmap.org/search?format=json&q={{.Address}}`, addressOrder: littleEndian}
	mapsCoURL    = urlTemplate{Base: `https://geocode.maps.co/search?format=json&api_key={{.APIKey}}&q={{.Address}}`}
)

var (
	ErrNotFound       = errors.New("not found")
	ErrTooManyResults = errors.New("too many results")

	rateLimit = rate.NewLimiter(1, 1)

	// GmapsAPIKey is the API_KEY served to Google Maps services.
	// It is set by default to the contents of the GOOGLE_MAPS_API_KEY env var.
	GmapsAPIKey = os.Getenv("GOOGLE_MAPS_API_KEY")

	// MapsCoAPIKey isd the API_Key served to maps.co
	// It is set by default to the contents of the MAPSCO_API_KEY env var.
	MapsCoAPIKey = os.Getenv("MAPSCO_API_KEY")
)

type Location struct {
	Address string
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

var retryStrategy = retry.Strategy{
	Delay:       100 * time.Millisecond,
	MaxDelay:    5 * time.Second,
	MaxDuration: 30 * time.Second,
	Factor:      2,
}

func Get(ctx context.Context, address string) (Location, error) {
	select {
	case <-ctx.Done():
		return Location{}, ctx.Err()
	default:
	}
	F := func(aURL urlTemplate, key string) (Location, error) {
		aURL.Base = strings.Replace(aURL.Base, "{{.APIKey}}", url.QueryEscape(key), 1)
		return aURL.get(ctx, address)
	}
	if GmapsAPIKey != "" {
		if loc, err := F(gmapsURL, GmapsAPIKey); err == nil {
			return loc, err
		}
	}
	if MapsCoAPIKey != "" {
		if loc, err := F(mapsCoURL, MapsCoAPIKey); err != nil {
			return loc, err
		}
	}
	return nominatimURL.get(ctx, address)
}

func (ut urlTemplate) get(ctx context.Context, address string) (Location, error) {
	aURL := ut.WithAddress(address)

	logger := zlog.SFromContext(ctx)
	var loc Location
	var firstErr error
	var gData mapsResponse
	var nData []nominatimResult
	for iter := retryStrategy.Start(); ; {
		if err := rateLimit.Wait(ctx); err != nil {
			return loc, err
		}
		req, err := http.NewRequest("GET", aURL, nil)
		if err != nil {
			return loc, fmt.Errorf("%s: %w", aURL, err)
		}
		if err = func() error {
			resp, err := http.DefaultClient.Do(req.WithContext(ctx))
			if err != nil {
				logger.Error("coord request", "url", aURL, "error", err)
				return fmt.Errorf("%s: %w", aURL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode > 299 {
				logger.Error("coord request", "url", aURL, "status", resp.Status)
				return fmt.Errorf("%s: %w", aURL, errors.New(resp.Status))
			}
			logger.Info("coord request", "url", aURL, "status", resp.Status)
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil && len(b) == 0 {
				return fmt.Errorf("read response: %w", err)
			}
			var data any
			if bytes.HasPrefix(b, []byte("[")) {
				err = json.Unmarshal(b, &nData)
				data = nData
			} else {
				err = json.Unmarshal(b, &gData)
				data = gData
				if err != nil {
					if gData.Status != "OVER_QUERY_LIMIT" {
						rateLimit.SetLimit(rateLimit.Limit() * 1.1)
					} else {
						rateLimit.SetLimit(rateLimit.Limit() / 2)
					}
				}
			}
			if err != nil {
				logger.Error("decode", "data", string(b), "error", err)
				return fmt.Errorf("decode: %w", err)
			}
			logger.Info("response", "data", data, "body", string(b))
			return nil
		}(); err == nil {
			break
		}
		if firstErr == nil {
			firstErr = err
		}
		if !iter.Next(ctx.Done()) {
			return loc, firstErr
		}
	}

	if gData.Status == "" {
		if len(nData) == 0 {
			return loc, ErrNotFound
		}
		result := nData[0]
		loc.Address = result.DisplayName
		var err error
		if loc.Lat, err = strconv.ParseFloat(result.Lat, 64); err == nil {
			loc.Lng, err = strconv.ParseFloat(result.Lon, 64)
		}
		return loc, err

	}

	switch gData.Status {
	case "OK":
	case "ZERO_RESULTS":
		return loc, ErrNotFound
	default:
		return loc, errors.New(gData.Status)
	}
	switch len(gData.Results) {
	case 0:
		return loc, ErrNotFound
	case 1:
	default:
		return loc, ErrTooManyResults
	}
	result := gData.Results[0]
	loc.Address = result.FormattedAddress
	loc.Lat, loc.Lng = result.Geometry.Location.Lat, result.Geometry.Location.Lng
	return loc, nil
}

type nominatimResult struct {
	PlaceID     uint64   `json:"place_id"`
	Licence     string   `json:"licence"`
	OSMType     string   `json:"osm_type"`
	OSMID       uint64   `json:"osm_id"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	Class       string   `json:"class"`
	Type        string   `json:"type"`
	PlaceRank   uint32   `json:"place_rank"`
	Importance  float32  `json:"importance"`
	AddressType string   `json:"addresstype"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	BoundingBox []string `json:"boundingbox"`
}

type mapsResponse struct {
	Status  string       `json:"status"`
	Results []mapsResult `json:"results"`
}
type mapsResult struct {
	FormattedAddress string       `json:"formatted_address"`
	Geometry         mapsGeometry `json:"geometry"`
}
type mapsGeometry struct {
	Location mapsLocation `json:"location"`
}
type mapsLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}
