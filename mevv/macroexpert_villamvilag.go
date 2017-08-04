/*
Copyright 2017 Tamás Gulácsi

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

// Package mevv is for accessing "MacroExpert VillĂĄmVilĂĄg" service.
package mevv

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/pkg/errors"
)

const (
	macroExpertURLv0 = `https://www.macroexpert.hu/villamvilag_uj/interface_GetWeatherPdf.php`
	macroExpertURLv1 = `https://macrometeo.hu/meteo-api-app/api/pdf/query-kobe`
	macroExpertURLv2 = `https://macrometeo.hu/meteo-api-app/api/pdf/query`
)

// Log is used for logging.
var Log = func(...interface{}) error { return nil }

// Options are the space/time coordinates and the required details.
type Options struct {
	Address                          string
	Lat, Lng                         float64
	Since, Till                      time.Time
	At                               time.Time
	Interval                         int
	ContractID                       string
	NeedThunders, NeedIce, NeedWinds bool
	NeedRains, NeedRainsIntensity    bool
	ExtendedLightning                bool
	WithStatistics                   bool
	Host                             string
}

var client = &http.Client{}

func init() {
	tr := new(http.Transport)
	*tr = *(http.DefaultTransport.(*http.Transport))
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client.Transport = tr
}

type mevv struct {
	version Version
}
type Version string

const (
	V0 = Version("v0")
	V1 = Version("v1")
	V2 = Version("v2")
)

func (V Version) URL() string {
	switch V {
	case V0:
		return macroExpertURLv0
	case V1:
		return macroExpertURLv1
	case V2:
		return macroExpertURLv2
	default:
		return ""
	}
}

// GetPDF returns the meteorological data in PDF form.
/*
address M varchar(45) Keresett cĂ­m hĂĄzszĂĄmmal
lat M float(8,5) SzĂŠlessĂŠg pl.: â47.17451â
lng M float(8,5) HosszĂşsĂĄg pl.: â17.04234â
from_date M date(YYYY-MM-DD) KezdĹ datum pl.: â2014-11-25â
to_date M date(YYYY-MM-DD) ZĂĄrĂł datum pl.: â2014-11-29â
contr_id O varchar(25) KĂĄrszĂĄm pl.: âKSZ-112233â
needThunders O varchar(1) VillĂĄm adatokat kĂŠrek â1ââkĂŠrem, â0â-nem
needRains O varchar(1) CsapadĂŠk adatokat kĂŠrek â1ââkĂŠrem, â0â-nem
needWinds O varchar(1) SzĂŠl adatokat kĂŠrek â1â â kĂŠrem, â0â-nem
needRainsInt O varchar(1) Fix - â0â
language O varchar(2) Fix - âhuâ
*/
func (V Version) GetPDF(
	ctx context.Context,
	username, password string,
	opt Options,
) (rc io.ReadCloser, fileName, mimeType string, err error) {
	params := url.Values(map[string][]string{
		"address":  {opt.Address},
		"lat":      {fmt.Sprintf("%.5f", opt.Lat)},
		"lng":      {fmt.Sprintf("%.5f", opt.Lng)},
		"contr_id": {opt.ContractID},
	})

	if V == V0 || V == V1 {
		params["needThunders"] = []string{fmtBool(opt.NeedThunders)}
		params["needRains"] = []string{fmtBool(opt.NeedRains)}
		params["needWinds"] = []string{fmtBool(opt.NeedWinds)}
		params["needRainsInt"] = []string{fmtBool(opt.NeedRainsIntensity)}
	}

	if V == V0 {
		params["language"] = []string{"hu"}
		params["from_date"] = []string{V.fmtDate(opt.Since)}
		params["to_date"] = []string{V.fmtDate(opt.Till)}
	} else {
		params["language"] = []string{"hu_HU"}
		var d time.Duration
		if opt.At.IsZero() && !(opt.Since.IsZero() || opt.Till.IsZero()) {
			d = opt.Till.Sub(opt.Since) / 2
			opt.At = opt.Since.Add(d)
			if opt.Interval == 0 {
				opt.Interval = int(d/(24*time.Hour)) + 1
			}
		}
		switch {
		case opt.Interval < 15:
			opt.Interval = 5
		case opt.Interval < 90:
			opt.Interval = 30
			if d != 0 {
				opt.At = opt.Since
			}
		default:
			opt.Interval = 180
			if d != 0 {
				opt.At = opt.Since
			}
		}

		params["date"] = []string{V.fmtDate(opt.At)}
		if opt.Interval == 0 {
			opt.Interval = 5
		}
		params["interval"] = []string{strconv.Itoa(opt.Interval)}

		if V == V2 {
			params["lon"] = params["lng"]
			delete(params, "lon")
			params["referenceNo"] = params["contr_id"]
			delete(params, "contr_id")
			if opt.ExtendedLightning {
				params["extended"] = []string{"1"}
			}
			if opt.WithStatistics {
				params["withStatistics"] = []string{"1"}
			}
			if opt.NeedThunders {
				params["operation"] = append(params["operation"], "QUERY_LIGHTNING")
			}
			if opt.NeedWinds {
				params["operation"] = append(params["operation"], "QUERY_WIND")
			}
			if opt.NeedIce {
				params["operation"] = append(params["operation"], "QUERY_ICE")
			}
			if opt.NeedRains {
				params["operation"] = append(params["operation"], "QUERY_PRECIPITATION")
			}
			if opt.NeedRainsIntensity {
				params["operation"] = append(params["operation"], "QUERY_PRECIPITATION_INTENSITY")
			}
			if opt.NeedRainsIntensity {
				params["operation"] = append(params["operation"], "QUERY_PRECIPITATION_INTENSITY")
			}
		}
	}

	meURL := V.URL() + "?" + params.Encode()
	if opt.Host != "" {
		u, _ := url.Parse(meURL)
		u.Host = opt.Host
		meURL = u.String()
	}
	req, err := http.NewRequest("GET", meURL, nil)
	if err != nil {
		return nil, "", "", errors.Wrapf(err, "url=%q", meURL)
	}
	req.SetBasicAuth(username, password)
	select {
	case <-ctx.Done():
		return nil, "", "", ctx.Err()
	default:
	}
	Log("msg", "Get", "url", req.URL)
	resp, err := ctxhttp.Do(ctx, client, req)
	if err != nil {
		return nil, "", "", errors.Wrapf(err, "Do %#v", req.URL.String())
	}
	if resp.StatusCode > 299 {
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, "", "", errors.New("Authentication error: " + resp.Status)
		}
		return nil, "", "", errors.Errorf("%s: egyĂŠb hiba (%s)", resp.Status, req.URL)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "application/xml" { // error
		var mr meResponse
		var buf bytes.Buffer
		if err := xml.NewDecoder(io.TeeReader(resp.Body, &buf)).Decode(&mr); err != nil {
			_, _ = io.Copy(&buf, resp.Body)
			resp.Body.Close()
			return nil, "", "", errors.Wrapf(err, "parse %q", buf.String())
		}
		return nil, "", "", mr
	}
	if !strings.HasPrefix(ct, "application/") && !strings.HasPrefix(ct, "image/") {
		buf, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, "", "", errors.Errorf("998: %s", buf)
	}
	var fn string
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			fn = params["filename"]
		}
	}
	if fn == "" {
		fn = "macroexpert-villamvilag-" + url.QueryEscape(opt.Address) + ".pdf"
	}

	return resp.Body, fn, ct, nil
}

type meResponse struct {
	XMLName xml.Name `xml:"Response"`
	Code    string   `xml:"ResponseCode"`
	Text    string   `xml:"ResponseText"`
}

func (mr meResponse) ErrNum() int {
	n, err := strconv.Atoi(strings.TrimPrefix("ERR_", mr.Code))
	if err != nil {
		return 9999
	}
	return n
}

func (mr meResponse) Error() string { return fmt.Sprintf("%s: %s", mr.Code, mr.Text) }

func (V Version) fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if V == V0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006.01.02")
}
func fmtBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

var macroExpertUserPassw string

// ReadUserPassw reads the user/passw from the given file.
func ReadUserPassw(filename string) (string, string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return "", "", errors.Wrapf(err, "open %q", filename)
	}
	defer fh.Close()
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line = bytes.TrimSpace(line[:i])
		}
		if len(line) == 0 {
			continue
		}
		i := bytes.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		return string(line[:i]), string(line[i+1:]), nil
	}
	return "", "", io.EOF
}

// vim: set fileencoding=utf-8 noet:
