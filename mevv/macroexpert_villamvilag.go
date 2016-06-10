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

// Package mevv: a MacroExpert VillámVilág szolgáltatásának elérése.
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

var Log = func(...interface{}) error { return nil }

/*
GetPDF visszaad egy PDF-et a megadott címen és koordinátákon

    address M varchar(45) Keresett cím házszámmal
    lat M float(8,5) Szélesség pl.: ‘47.17451’
    lng M float(8,5) Hosszúság pl.: ‘17.04234’
    from_date M date(YYYY-MM-DD) Kezdő datum pl.: ‘2014-11-25’
    to_date M date(YYYY-MM-DD) Záró datum pl.: ‘2014-11-29’
    contr_id O varchar(25) Kárszám pl.: ‘KSZ-112233’
    needThunders O varchar(1) Villám adatokat kérek ‘1’–kérem, ‘0’-nem
    needRains O varchar(1) Csapadék adatokat kérek ‘1’–kérem, ‘0’-nem
    needWinds O varchar(1) Szél adatokat kérek ‘1’ – kérem, ‘0’-nem
    needRainsInt O varchar(1) Fix - ‘0’
    language O varchar(2) Fix - ‘hu’
*/
type Options struct {
	Address                                                string
	Lat, Lng                                               float64
	Since, Till                                            time.Time
	ContractID                                             string
	NeedThunders, NeedRains, NeedWinds, NeedRainsIntensity bool
}

var client = &http.Client{}

func init() {
	tr := new(http.Transport)
	*tr = *(http.DefaultTransport.(*http.Transport))
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client.Transport = tr
}

func GetPDF(
	ctx context.Context,
	username, password string,
	opt Options,
) (rc io.ReadCloser, fileName, mimeType string, err error) {
	params := url.Values(map[string][]string{
		"address":   {opt.Address},
		"lat":       {fmt.Sprintf("%.5f", opt.Lat)},
		"lng":       {fmt.Sprintf("%.5f", opt.Lng)},
		"from_date": {fmtDate(opt.Since)}, "to_date": {fmtDate(opt.Till)},
		"contr_id":     {opt.ContractID},
		"needThunders": {fmtBool(opt.NeedThunders)},
		"needRains":    {fmtBool(opt.NeedRains)},
		"needWinds":    {fmtBool(opt.NeedWinds)},
		"needRainsInt": {fmtBool(opt.NeedRainsIntensity)},
		"language":     {"hu"},
	})

	meURL := macroExpertURL + "?" + params.Encode()
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
		return nil, "", "", errors.Wrapf(err, "Do %#v", req)
	}
	if resp.StatusCode > 299 {
		resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, "", "", errors.New("Authentication error: " + resp.Status)
		}
		return nil, "", "", errors.New(fmt.Sprintf("%s: egyéb hiba (%s)", resp.Status, req.URL))
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
		return nil, "", "", errors.New(fmt.Sprintf("998: %s", buf))
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

func fmtDate(t time.Time) string {
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

var macroExpertUserPassw string

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
