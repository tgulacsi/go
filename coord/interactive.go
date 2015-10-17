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
	//"crypto/hmac"
	//"crypto/rand"
	//"crypto/sha256"
	//"encoding/base64"
	"crypto/rand"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/inconshreveable/log15.v2"
)

var (
	Log = log15.New("lib", "coord")

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

	Locations map[string]Location // setted locations
	sync.Mutex
}
type staticParams struct {
	Title                      string
	MapCenterLat, MapCenterLng string
	LocLat, LocLng             string
	DefaultAddress             string
	CallbackPath               string
}

func (in *Interactive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in.Lock()
	if in.Locations == nil {
		in.Locations = make(map[string]Location)
	}
	in.Unlock()
	if strings.HasSuffix(r.URL.Path, "/_koord/set") {
		in.serveSet(w, r)
		return
	}
	if in.DefaultAddress == "" {
		in.DefaultAddress = DefaultAddress
	}
	if in.Title == "" {
		in.Title = DefaultTitle
	}
	sp := staticParams{
		Title:          in.Title,
		MapCenterLat:   fmt.Sprintf("%+f", in.MapCenter.Lat),
		MapCenterLng:   fmt.Sprintf("%+f", in.MapCenter.Lng),
		LocLat:         fmt.Sprintf("%+f", in.Location.Lat),
		LocLng:         fmt.Sprintf("%+f", in.Location.Lng),
		DefaultAddress: in.DefaultAddress,
		CallbackPath:   path.Join(in.BaseURL, "_koord", "set"),
	}
	if err := tmpl.Execute(w, sp); err != nil {
		Log.Error("template with %#v: %v", sp, err)
	}
}
func (in *Interactive) serveSet(w http.ResponseWriter, r *http.Request) {
	var mp macParams
	if err := mp.ParseQuery(r.URL.Query()); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !mp.Check() {
		http.Error(w, "bad mac", http.StatusBadRequest)
		return
	}

	in.Lock()
	in.Locations[mp.ID] = mp.Location
	in.Unlock()

	if in.Set == nil {
		return
	}
	if err := in.Set(mp.ID, mp.Location); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

const (
	keyLength   = 32
	nonceLength = 8
)

var (
	macKey []byte
	tmpl   *template.Template
)

func init() {
	Log.SetHandler(log15.DiscardHandler())

	macKey = make([]byte, keyLength)
	if _, err := rand.Read(macKey); err != nil {
		panic(err)
	}
	tmpl = template.Must(template.New("gmapsHTML").Parse(gmapsHTML))
}

type macParams struct {
	Location
	ID string
	//Nonce []byte
	//MAC   []byte
}

func (mp *macParams) ParseQuery(vals url.Values) error {
	mp.ID = vals.Get("id")
	latS, lngS := vals.Get("lat"), vals.Get("lng")
	var err error
	//mp.Nonce, err = base64.URLEncoding.DecodeString(vals.Get("nonce"))
	//if err != nil {
	//return err
	//}
	//mp.MAC, err = base64.URLEncoding.DecodeString(vals.Get("mac"))
	//if err != nil {
	//return err
	//}
	if mp.Lat, err = strconv.ParseFloat(latS, 64); err != nil {
		return err
	}
	if mp.Lng, err = strconv.ParseFloat(lngS, 64); err != nil {
		return err
	}
	return nil
}

func (mp macParams) Check() bool {
	//return hmac.Equal(mp.generate(nil), mp.MAC)
	return true
}

func (mp macParams) generate(mac []byte) []byte {
	//mc := hmac.New(sha256.New, macKey)
	//io.WriteString(mc,
	//"id="+url.QueryEscape(mp.ID)+
	//"&lat="+url.QueryEscape(fmt.Sprintf("%+f", mp.Lat))+
	//"&lng="+url.QueryEscape(fmt.Sprintf("%+f", mp.Lng))+
	//"&nonce="+base64.URLEncoding.EncodeToString(mp.Nonce),
	//)
	//return mc.Sum(mac)
	return nil
}

func (mp *macParams) Generate() {
	//if mp.Nonce == nil {
	//mp.Nonce = make([]byte, nonceLength)
	//_, _ = rand.Read(mp.Nonce)
	//}
	//mp.MAC = mp.generate(mp.MAC[:0])
}
