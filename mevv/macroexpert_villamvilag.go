/*
Copyright 2017, 2023 Tamás Gulácsi

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
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/httpinsecure"
	"github.com/tgulacsi/go/iohlp"
)

var ErrAuth = errors.New("authentication error")

const (
	macroExpertURLv0     = `https://www.macroexpert.hu/villamvilag_uj/interface_GetWeatherPdf.php`
	macroExpertURLv1     = `https://macrometeo.hu/meteo-api-app/api/pdf/query-kobe`
	macroExpertURLv2     = `https://macrometeo.hu/meteo-api-app/api/pdf/query`
	macroExpertURLv3     = `https://frontend.macrometeo.hu/webapi/query-civil`
	MacroExpertURLv3Test = `https://frontend-test.macrometeo.hu/webapi/query-civil`
)

// Options are the space/time coordinates and the required details.
type Options struct {
	Since, Till                      time.Time `json:"-"`
	At                               time.Time `json:"eventDate"`
	Address                          string    `json:"address"`
	ContractID                       string    `json:"referenceNo"`
	URL                              string    `json:"-"`
	Lat                              float64   `json:"locationLat"`
	Lng                              float64   `json:"locationLon"`
	Interval                         int       `json:"interval"`
	NeedThunders, NeedIce, NeedWinds bool      `json:"-"`
	NeedRains, NeedRainsIntensity    bool      `json:"-"`
	NeedTemperature                  bool      `json:"-"`
	ExtendedLightning                bool      `json:"extendedRange"`
	NeedPDF, NeedData                bool      `json:"-"`
	WithStatistics                   bool      `json:"withStatistic"`
	Hourly                           bool      `json:"-"`
}

func (opt Options) Prepare() Options {
	var d time.Duration
	wasZero := opt.At.IsZero() && !(opt.Since.IsZero() || opt.Till.IsZero())
	if wasZero {
		d = opt.Till.Sub(opt.Since) / 2
		opt.At = opt.Since.Add(d)
	}
	if opt.Hourly {
		if opt.NeedRains {
			opt.Interval = 3
		} else {
			opt.Interval = 1
		}
		return opt
	}

	if wasZero && opt.Interval == 0 {
		opt.Interval = int(d/(24*time.Hour)) + 1
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

	if opt.Interval <= 0 {
		opt.Interval = 5
	}
	return opt
}

type V3Request struct {
	Username string  `json:"userName"`
	Password string  `json:"password"`
	Query    V3Query `json:"query"`
}
type V3Query struct {
	ResultTypes        []string `json:"resultTypes"`
	SelectedOperations []string `json:"selectedOperations"`
	Options
}

func (req V3Query) Prepare() V3Query {
	req.Options = req.Options.Prepare()
	req.Options.At = req.Options.At.UTC()
	req.ResultTypes = req.ResultTypes[:0]
	if req.Options.NeedData {
		req.ResultTypes = append(req.ResultTypes, "DATA")
	}
	if req.Options.NeedPDF || len(req.ResultTypes) == 0 {
		req.ResultTypes = append(req.ResultTypes, "PDF")
	}
	if req.Options.Hourly {
		if req.Options.NeedRains {
			req.SelectedOperations = append(req.SelectedOperations, "QUERY_BY_STATION_PREC")
		} else if req.Options.NeedWinds {
			req.SelectedOperations = append(req.SelectedOperations, "QUERY_BY_STATION_WIND")
		} else {
			req.SelectedOperations = append(req.SelectedOperations, "QUERY_BY_STATION_TEMP")
		}
		return req
	}
	if req.Options.NeedRains {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_PRECIPITATION")
	}
	if req.Options.NeedRainsIntensity {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_PRECIPITATION_INTENSITY")
	}
	if req.Options.NeedWinds {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_WIND")
	}
	if req.Options.NeedIce {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_ICE")
	}
	if req.Options.NeedTemperature {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_TEMPERATURE")
	}
	if req.Options.NeedThunders || len(req.SelectedOperations) == 0 {
		req.SelectedOperations = append(req.SelectedOperations, "QUERY_LIGHTNING")
	}
	return req
}

var client = &http.Client{Transport: httpinsecure.InsecureTransport}

type Version string

const (
	V0 = Version("v0")
	V1 = Version("v1")
	V2 = Version("v2")
	V3 = Version("v3")
)

func (V Version) URL() string {
	switch V {
	case V3:
		return macroExpertURLv3
	case V2:
		return macroExpertURLv2
	case V1:
		return macroExpertURLv1
	case V0:
		return macroExpertURLv0

	default:
		return ""
	}
}

func (V Version) LatKey() string {
	switch V {
	case V3:
		return "locationLat"
	case V2:
		return "lat"
	}
	return "lng"
}

func (V Version) LngKey() string {
	switch V {
	case V3:
		return "locationLon"
	case V2:
		return "lon"
	}
	return "lng"
}

func (V Version) RefKey() string {
	switch V {
	case V3, V2:
		return "referenceNo"
	}
	return "contr_id"
}

type (
	V3Response struct {
		Data        V3ResultData `json:"resultData"`
		File        V3File       `json:"file"`
		Errors      []V3Error    `json:"errors"`
		OperationID int          `json:"operationId"`
		Successful  bool         `json:"isSuccessful"`
	}
	V3ResultData struct {
		DateFrom                        time.Time          `json:"dateFrom"`
		DateTo                          time.Time          `json:"dateTo"`
		EventDate                       time.Time          `json:"eventDate"`
		Address                         string             `json:"address"`
		ReferenceNo                     string             `json:"referenceNo"`
		DailyListWind                   []V3DailyWind      `json:"dailyListWind"`
		DailyListPrecipitation          []V3DailyPrecip    `json:"dailyListPrecipitation"`
		DailyListPrecipitationIntensity []V3Measurement    `json:"dailyListPrecipitationIntensity"`
		DailyListIce                    []V3DailyIce       `json:"dailyListIce"`
		DailyListTemperature            []V3DailyTemp      `json:"dailyListTemperature"`
		LightningList                   []V3DailyLightning `json:"lightingList"`
		ByStationList                   []V3Measurement    `json:"byStationList"`
		ByStationPrecList               []V3MeasurePrecip  `json:"byStationPrecList"`
		ByStationTempList               []V3MeasureTemp    `json:"byStationTempList"`
		ByStationWindList               []V3MeasureWind    `json:"byStationWindList"`
		AgroFrostList                   []V3Measurement    `json:"agroFrostList"`
		AgroExtendedList                []V3Measurement    `json:"agroFrostExtendedList"`
		Drought                         []V3Drought        `json:"agroDroughtList"`
		DroughtExtendedList             []V3Measurement    `json:"agroDroughtExtendedList"`
		Statistics                      []V3Statistic      `json:"statisticsList"`
		Raw                             json.RawMessage    `json:"-"`
		Lat                             float64            `json:"locationLat"`
		Lon                             float64            `json:"locationLon"`
		Interval                        int                `json:"interval"`
		LightningRadius                 int                `json:"lightningRadius"`
		Visibility                      V3Visibility       `json:"visibility"`
	}

	V3Visibility struct {
		Daily                       bool `json:"hasDailyData"`
		DailyWind                   bool `json:"hasDailyData_Wind"`
		DailyPrecipitation          bool `json:"hasDailyData_Precipitation"`
		DailyPrecipitationIntensity bool `json:"hasDailyData_PrecipitationIntensity"`
		DailyIce                    bool `json:"hasDailyData_Ice"`
		DailyTemperature            bool `json:"hasDailyData_Temperature"`
		Lightning                   bool `json:"hasLightingData"`
		ByStationTemperature        bool `json:"hasByStationTemperature"`
		ByStationPrecipitation      bool `json:"hasByStationPrecipitation"`
		ByStationWind               bool `json:"hasByStationWind"`
		AgroFrost                   bool `json:"hasAgroFrost"`
		AgroFrostExtended           bool `json:"hasAgroFrostExtended"`
		AgroDrought                 bool `json:"hasAgroDrought"`
		AgroDroughtExtended         bool `json:"hasAgroDroughtExtended"`
		Statistic                   bool `json:"hasStatistic"`
	}

	V3MeasurePrecip struct {
		V3Measurement
		Precipitation float64 `json:"precipitation"`
	}
	V3MeasureTemp struct {
		V3Measurement
	}
	V3MeasureWind struct {
		V3Measurement
		Direction  string  `json:"directionCode"`
		MaxGustKmH float64 `json:"maxGustKmH"`
	}

	V3DailyIce struct {
		Date  string `json:"dateString"`
		Value bool   `json:"value"`
	}
	V3DailyLightning struct {
		EventDateUTC time.Time   `json:"eventDateUtc"`
		EventDate    time.Time   `json:"eventDate"`
		Zone         json.Number `json:"zone"`
		// Type               json.Number `json:"lightningType"`
		Index              int     `json:"index"`
		Altitude           float64 `json:"altitude"`
		CurrentIntensity   float64 `json:"currentIntensity"`
		DistanceFromOrigin float64 `json:"distanceFromOrigin"`
		Inaccuracy         float64 `json:"inaccuracy"`
	}
	V3DailyPrecip struct {
		Date  string  `json:"dateString"`
		Value float64 `json:"value"`
	}
	V3DailyTemp struct {
		Date     string  `json:"dateString"`
		MinValue float64 `json:"minValue"`
		MaxValue float64 `json:"maxValue"`
		Value    string  `json:"string"`
	}
	V3DailyWind struct {
		Date     string  `json:"dateString"`
		MinValue float64 `json:"minValue"`
		MaxValue float64 `json:"maxValue"`
		Value    string  `json:"string"`
		Code     float64 `json:"code"`
	}

	V3Measurement struct {
		Date               string          `json:"dateString"`
		Hour               string          `json:"hour"`
		Code               json.Number     `json:"code"`
		Settlement         string          `json:"settlementText"`
		Value              json.RawMessage `json:"value"`
		Altitude           float64         `json:"altitude"`
		DistanceFromOrigin float64         `json:"distanceFromOrigin"`
		PrecipitationMax   float64         `json:"precipitationMax"`
		TemperatureMin     float64         `json:"temperatureMin"`
		TemperatureMax     float64         `json:"temperatureMax"`
		MinValue           float64         `json:"minValue"`
		MaxValue           float64         `json:"maxValue"`
		DroughtIndex1      bool            `json:"droughtIndex"`
		DroughtIndex2      bool            `json:"droughtIndex2"`
	}

	V3Drought struct {
		FromDate       string
		ToDate         string `json:"toDate"`
		Index1, Index2 bool
	}
	V3Statistic struct {
		OperationTypeName string  `json:"operationTypeName"`
		Sum               float64 `json:"sum"`
	}
	V3File struct {
		UUID        string `json:"uuid"`
		Name        string `json:"fileName"`
		ContentType string `json:"contentType"`
		Data        []byte `json:"data"`
	}
	V3Error struct {
		Field   string `json:"fieldName"`
		Code    string `json:"errorCode"`
		Message string `json:"errorMessage"`
		Serious bool   `json:"isSerious"`
	}
)

var _ error = (*V3Error)(nil)

func (ve V3Error) Error() string {
	if ve.Field != "" {
		return fmt.Sprintf("%q: %s: %s", ve.Field, ve.Code, ve.Message)
	}
	return fmt.Sprintf("%s: %s", ve.Code, ve.Message)
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
	_, rc, fileName, mimeType, err = V.GetPDFData(ctx, username, password, opt)
	return rc, fileName, mimeType, err
}
func (V Version) GetPDFData(
	ctx context.Context,
	username, password string,
	opt Options,
) (data V3ResultData, r io.ReadCloser, fileName, mimeType string, err error) {
	logger := zlog.SFromContext(ctx)
	meURL := opt.URL
	if meURL == "" {
		meURL = V.URL()
	}
	var body io.Reader
	if V == V3 {
		qry := V3Query{Options: opt}.Prepare()
		req := V3Request{Query: qry, Username: username, Password: password}
		b, marshalErr := json.Marshal(req)
		if marshalErr != nil {
			err = marshalErr
			return
		}
		logger.Debug("V3Request", "body", string(b))
		body = bytes.NewReader(b)
	} else {
		opt = opt.Prepare()
		params := url.Values(map[string][]string{
			"address":  {opt.Address},
			V.LatKey(): {fmt.Sprintf("%.5f", opt.Lat)},
			V.LngKey(): {fmt.Sprintf("%.5f", opt.Lng)},
			V.RefKey(): {opt.ContractID},
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
			params["date"] = []string{V.fmtDate(opt.At)}
			params["interval"] = []string{strconv.Itoa(opt.Interval)}

			switch V {
			case V2:
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

				if len(params["operation"]) == 0 {
					params["operation"] = append(params["operation"], "QUERY_LIGHTNING")
				}
			}
		}
		meURL += "?" + params.Encode()
	}

	method := "GET"
	var buf strings.Builder
	if body != nil {
		method = "POST"
		body = io.TeeReader(body, &buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, meURL, body)
	if err != nil {
		logger.Error("NewRequest", "method", method, "url", meURL, "body", buf.String(), "error", err)
		return V3ResultData{}, nil, "", "", fmt.Errorf("%s %q: %w\nbody: %s", method, meURL, err, buf.String())
	}
	// logger.Debug("MEVV", "username", username, "password", strings.Repeat("*", len(password)))
	if V == V3 {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
	} else if username != "" {
		req.SetBasicAuth(username, password)
	}
	logger.Debug(method, "url", req.URL, "headers", req.Header)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(method, "url", req.URL, "headers", req.Header, "body", buf.String(), "error", err)
		return V3ResultData{}, nil, "", "", fmt.Errorf("do %#v (%q): %w", req.URL.String(), buf.String(), err)
	}
	var sr *io.SectionReader
	if resp.Body != nil {
		if sr, err = iohlp.MakeSectionReader(resp.Body, 1<<20); err != nil {
			return V3ResultData{}, nil, "", "", err
		}
		resp.Body.Close()
	}

	if resp.StatusCode > 299 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return V3ResultData{}, nil, "", "", fmt.Errorf("%s: %w", resp.Status, ErrAuth)
		}
		var a [1024]byte
		n, _ := sr.ReadAt(a[:], 0)
		logger.Error(method, "url", req.URL, "headers", req.Header, "body", a[:n], "status", resp.Status)
		return V3ResultData{}, nil, "", "", fmt.Errorf("%s: egyĂŠb hiba (%s)", resp.Status, req.URL)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "application/xml" { // error
		var mr meResponse
		if err := xml.NewDecoder(io.NewSectionReader(sr, 0, sr.Size())).Decode(&mr); err != nil {
			b, _ := io.ReadAll(sr)
			return V3ResultData{}, nil, "", "", fmt.Errorf("parse %q: %w", string(b), err)
		}
		return V3ResultData{}, nil, "", "", mr
	}
	if V == V3 {
		if prefix, _, _ := strings.Cut(ct, ";"); prefix == "application/json" {
			if logger.Enabled(ctx, slog.LevelDebug) {
				var a [1024]byte
				n, _ := sr.ReadAt(a[:], 0)
				b := a[:n]
				logger.Debug("V3", "response", string(b))
			}
			var v3resp V3Response
			if err := json.NewDecoder(io.NewSectionReader(
				sr, 0, sr.Size(),
			)).Decode(&v3resp); err != nil {
				return v3resp.Data, nil, "", "", err
			}
			b, _ := io.ReadAll(sr)
			v3resp.Data.Raw = json.RawMessage(b)
			if len(v3resp.Errors) != 0 {
				return v3resp.Data, nil, "", "", &v3resp.Errors[0]
			}
			f := v3resp.File
			logger.Info("got", "file", f.Name, "length", len(f.Data), "ct", f.ContentType)
			return v3resp.Data, struct {
				io.Reader
				io.Closer
			}{bytes.NewReader(f.Data), io.NopCloser(nil)}, f.Name, f.ContentType, nil
		}
	}

	if !strings.HasPrefix(ct, "application/") && !strings.HasPrefix(ct, "image/") {
		b, _ := io.ReadAll(sr)
		return V3ResultData{}, nil, "", "", fmt.Errorf("998: %s", string(b))
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

	return V3ResultData{}, resp.Body, fn, ct, nil
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
	return t.Format("2006-01-02")
}
func fmtBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ReadUserPassw reads the user/passw from the given file.
func ReadUserPassw(filename string) (string, string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return "", "", fmt.Errorf("open %q: %w", filename, err)
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
