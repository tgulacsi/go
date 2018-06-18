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

package coord

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template" // yes, no need for context-aware escapes

	"github.com/pkg/errors"
)

var (
	Log = func(...interface{}) error { return nil }

	DefaultTitle   = "Cím koordináták pontosítása"
	DefaultAddress = "Budapest"
)

type Interactive struct {
	Set            func(id string, loc Location) error
	Title          string
	MapCenter      Location
	Location       Location
	DefaultAddress string
	BaseURL        string
	NoDirect       bool

	inProgressMu sync.Mutex
	inProgress   map[string]struct{} // location set in progress
}
type staticParams struct {
	Address, Title             string
	MapCenterLat, MapCenterLng string
	LocLat, LocLng             string
	DefaultAddress             string
	CallbackPath               string
}

func (in *Interactive) RenderHTML(w io.Writer, address, callbackURL string) error {
	sp := staticParams{
		Address:        address,
		DefaultAddress: in.DefaultAddress,
		Title:          in.Title,
		MapCenterLat:   fmt.Sprintf("%+f", in.MapCenter.Lat),
		MapCenterLng:   fmt.Sprintf("%+f", in.MapCenter.Lng),
		LocLat:         fmt.Sprintf("%+f", in.Location.Lat),
		LocLng:         fmt.Sprintf("%+f", in.Location.Lng),
		CallbackPath:   callbackURL,
	}
	if err := tmpl.Execute(w, sp); err != nil {
		return errors.Wrapf(err, "with %#v", sp)
	}
	return nil
}

func (in *Interactive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vals := r.URL.Query()
	id := vals.Get("id")
	in.inProgressMu.Lock()
	if in.inProgress == nil {
		in.inProgress = make(map[string]struct{}, 8)
	}
	in.inProgress[id] = struct{}{}
	in.inProgressMu.Unlock()

	if strings.HasSuffix(r.URL.Path, "/set") {
		in.serveSet(w, r)
		return
	}
	if in.DefaultAddress == "" {
		in.DefaultAddress = DefaultAddress
	}
	if in.Title == "" {
		in.Title = DefaultTitle
	}
	if err := in.RenderHTML(w, vals.Get("address"), in.BaseURL+"/?id="+url.QueryEscape(id)); err != nil {
		Log("msg", "RenderHTML", "error", err)
	}
}
func (in *Interactive) serveSet(w http.ResponseWriter, r *http.Request) {
	vals := r.URL.Query()
	id := vals.Get("id")
	in.inProgressMu.Lock()
	_, ok := in.inProgress[id]
	in.inProgressMu.Unlock()
	if id == "" || !ok {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	lat, lng, err := parseLatLng(vals.Get("lat"), vals.Get("lng"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	in.inProgressMu.Lock()
	delete(in.inProgress, id)
	in.inProgressMu.Unlock()

	if in.Set == nil {
		return
	}
	if err := in.Set(id, Location{Lat: lat, Lng: lng}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("gmapsHTML").Parse(gmapsHTML))
}

func parseLatLng(latS, lngS string) (float64, float64, error) {
	lat, err := strconv.ParseFloat(latS, 64)
	if err != nil {
		return lat, 0, err
	}
	lng, err := strconv.ParseFloat(lngS, 64)
	return lat, lng, err
}
