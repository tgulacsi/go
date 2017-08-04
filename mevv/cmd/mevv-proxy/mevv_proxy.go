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

package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/tgulacsi/go/mevv"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flagHTTP := flag.String("http", ":8383", "HTTP host:port to listen on")
	flag.Parse()

	tr := *(http.DefaultTransport.(*http.Transport))
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
		tr.TLSClientConfig.InsecureSkipVerify = true
	}

	hndl := &httputil.ReverseProxy{
		Director:  director(mevv.V2.URL()),
		Transport: &tr,
	}
	log.Println("Start listening on", *flagHTTP)
	return http.ListenAndServe(*flagHTTP, hndl)
}

func director(dest string) func(r *http.Request) {
	destURL, err := url.Parse(dest)
	if err != nil {
		panic(errors.Wrap(err, dest))
	}

	/*
		V0
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

		V2
		• address=[string]
		– káresemény címe
		• date=[string: YYYY-MM-DD]
		– káresemény dátuma, pl. 2017.05.04
		• interval=[number: 5 | 30 | 180]
		lekérdezendő napok száma. A tól-ig dátumok az alábbi táblázat szerint alakulnak:
		5 nap: tól = date - 2, ig = date + 2
		30 nap: tól = date, ig = date + 29
		180 nap: tól = date, ig = date + 179

		• lat=[number: XX.X+]
		– koordináta szélesség, pl. 47.8941234
		• lon=[number: XX.X+]
		– koordináta hosszúság, pl. 19.63477
		• language=[enum: hu_HU]
		– nyelv, csak a magyar támogatott: paraméter értéke hu_HU
		• operation=[enum: QUERY_LIGHTNING | QUERY_WIND | QUERY_ICE |
		| QUERY_PRECIPITATION_INTENSITY]
		QUERY_PRECIPITATION
		– lekérdezés típusa, többet is fel lehet sorolni, pl: operation=QUERY_WIND&operation=QUERY_PRECIPITATION
		Opcionális:
		• referenceNo=[string]
		– kárszám
		1
		• withStatistics=[number: 0 | 1]
		– statisztika lekérdezése a felsorolt lekérdezés típusokra
		• extended=[number: 0 | 1]
		– kiterjesztett villám sugár alkalmazása, 6km
	*/
	return func(r *http.Request) {
		u1 := r.URL
		q1 := u1.Query()
		q2 := make(url.Values, len(q1)+1)
		q2["language"] = []string{"hu_HU"}
		for _, k := range []string{"address", "lat", "withStatistics", "extended", "lon", "date", "interval", "operation", "referenceNo"} {
			q2[k] = q1[k]
		}
		if q2.Get("referenceNo") == "" {
			q2["referenceNo"] = q1["contr_id"]
		}
		if q2.Get("lon") == "" {
			q2["lon"] = q1["lng"]
		}
		if m := q2["operation"]; len(m) == 0 {
			for k, v := range map[string]string{
				"needThunders": "LIGHTNING",
				"needRains":    "PRECIPITATION",
				"needRainsInt": "PRECIPITATION_INTENSITY",
				"needWinds":    "WIND",
				"needIce":      "ICE",
			} {
				if q1.Get(k) == "1" {
					m = append(m, "QUERY_"+v)
				}
			}
			q2["operation"] = m
		}
		if q2.Get("date") == "" {
			t1, err := time.Parse("2006-01-02", q1.Get("from_date"))
			if err != nil {
				log.Println(errors.Wrap(err, "from_date="+q1.Get("from_date")))
			}
			t2, err := time.Parse("2006-01-02", q1.Get("to_date"))
			if err != nil {
				log.Println(errors.Wrap(err, "to_date="+q1.Get("to_date")))
			}
			if t1.IsZero() && t2.IsZero() {
				t2 = time.Now()
			}
			if t1.IsZero() {
				t1 = t2.Add(-5 * 24 * time.Hour)
			}
			if t2.IsZero() {
				t2 = t1.Add(5 * 24 * time.Hour)
			}

			if !t2.IsZero() && t1.After(t2) {
				t1, t2 = t2, t1
			}
			d := t2.Sub(t1)
			/*
				5 nap: tól = date - 2, ig = date + 2
				30 nap: tól = date, ig = date + 29
				180 nap: tól = date, ig = date + 179
			*/
			t, i := t1, 5
			if d <= 15*24*time.Hour {
				t = t1.Add(d / 2)
			} else if d <= 90*24*time.Hour {
				i = 30
			} else {
				i = 180
			}
			q2["date"] = []string{t.Format("2006-01-02")}
			q2["interval"] = []string{strconv.Itoa(i)}
		}

		u2 := *destURL
		u2.RawQuery = q2.Encode()

		r.URL = &u2
		log.Println(u1.String(), "->", u2.String())
	}
}
