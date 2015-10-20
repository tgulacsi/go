/*
Copyright 2015 Tamás Gulácsi

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
	"encoding/json"
	"net/url"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"gopkg.in/errgo.v1"
)

const gmapsURL = `https://maps.googleapis.com/maps/api/geocode/json?sensors=false&address={{.Address}}`

var (
	ErrNotFound       = errgo.Newf("not found")
	ErrTooManyResults = errgo.Newf("too many results")
)

type Location struct {
	Address string
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

func Get(ctx context.Context, address string) (Location, error) {
	var loc Location
	select {
	case <-ctx.Done():
		return loc, ctx.Err()
	default:
	}
	aURL := strings.Replace(gmapsURL, "{{.Address}}", url.QueryEscape(address), 1)
	resp, err := ctxhttp.Get(ctx, nil, aURL)
	if err != nil {
		return loc, errgo.Notef(err, aURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return loc, errgo.Notef(err, aURL)
	}

	var data mapsResponse
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return loc, errgo.Notef(err, "decode")
	}
	switch data.Status {
	case "OK":
	case "ZERO_RESULTS":
		return loc, ErrNotFound
	default:
		return loc, errgo.Notef(err, "status=%q", data.Status)
	}
	switch len(data.Results) {
	case 0:
		return loc, ErrNotFound
	case 1:
	default:
		return loc, ErrTooManyResults
	}
	result := data.Results[0]
	loc.Address = result.FormattedAddress
	loc.Lat, loc.Lng = result.Geometry.Location.Lat, result.Geometry.Location.Lng
	return loc, nil
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
