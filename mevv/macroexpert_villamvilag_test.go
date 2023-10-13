/*
Copyright 2017, 2022 Tamás Gulácsi

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

package mevv_test

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/mevv"
)

const testHost = "40.68.241.196"

func TestMacroExpertVillamVilagPDF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	username, password, cases := testCases(t)
	U2, _ := url.Parse(mevv.V2.URL())
	U2.Scheme, U2.Host = "http", testHost
	for V, URL := range map[mevv.Version]string{
		mevv.V2: U2.String(),
		mevv.V3: mevv.MacroExpertURLv3Test,
	} {
		V, URL := V, URL
		t.Run(string(V), func(t *testing.T) {
			for nm, tc := range cases {
				nm, tc := nm, tc
				t.Run(nm, func(t *testing.T) {
					t.Parallel()
					ctx := zlog.NewSContext(ctx, zlog.NewT(t).SLog().
						With("version", V, "case", nm))
					tc.URL = URL
					ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
					data, r, _, ct, err := V.GetPDFData(ctx, username, password, tc.Options)
					cancel()
					t.Logf("ct=%q err=%v", ct, err)
					if err != nil {
						t.Errorf("got [%s] %v.", ct, err)
						return
					}
					var buf bytes.Buffer
					if r != nil {
						_, err = io.Copy(&buf, r)
						r.Close()
						if err != nil {
							t.Errorf("read response: %+v", err)
						}
					}

					if tc.Check != nil {
						t.Log(string(data.Raw))
						tc.Check(t, data)
					}
					// t.Log("response:", buf.String())
				})
			}
		})
	}
}

type testCase struct {
	mevv.Options
	Check func(*testing.T, mevv.V3ResultData)
}

func testCases(t *testing.T) (string, string, map[string]testCase) {
	username, password := os.Getenv("MEVV_USERNAME"), os.Getenv("MEVV_PASSWORD")
	if username == "" && password == "" {
		t.Logf("Environment variables MEVV_USERNAME and MEVV_PASSWORD are empty, reading from .password")
		var err error
		username, password, err = mevv.ReadUserPassw(".password")
		if err != nil {
			t.Fatal(err)
		}
	}

	return username, password,
		map[string]testCase{
			"dailyPDF": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				Since:      time.Date(2019, 01, 27, 0, 0, 0, 0, time.Local),
				Till:       time.Date(2019, 01, 30, 0, 0, 0, 0, time.Local),
				ContractID: "TESZT",
				NeedPDF:    true,
			}},
			"dailyData": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				Since:           time.Date(2019, 05, 27, 0, 0, 0, 0, time.Local),
				Till:            time.Date(2019, 05, 30, 0, 0, 0, 0, time.Local),
				ContractID:      "TESZT",
				NeedData:        true,
				NeedIce:         true,
				NeedRains:       true,
				NeedThunders:    true,
				NeedWinds:       true,
				NeedTemperature: true,
			},
				Check: func(t *testing.T, data mevv.V3ResultData) {
					v := data.Visibility
					if !v.DailyIce || len(data.DailyListIce) == 0 {
						t.Log(data.DailyListIce)
						t.Error("wanted ice")
					}
					if !v.DailyPrecipitation || len(data.DailyListPrecipitation) == 0 {
						t.Log(data.DailyListPrecipitation)
						t.Error("wanted precip")
					}
					if !v.DailyTemperature || len(data.DailyListTemperature) == 0 {
						t.Log(data.DailyListTemperature)
						t.Error("wanted temperature")
					}
					if !v.DailyWind || len(data.DailyListWind) == 0 {
						t.Log(data.DailyListWind)
						t.Error("wanted wind")
					}
				},
			},

			"hourlyRains": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				Since:      time.Date(2022, 01, 27, 0, 0, 0, 0, time.Local),
				Till:       time.Date(2022, 01, 30, 0, 0, 0, 0, time.Local),
				ContractID: "TESZT",
				NeedData:   true, NeedPDF: false,
				Hourly:    true,
				NeedRains: true,
			},
				Check: func(t *testing.T, data mevv.V3ResultData) {
					t.Log(data.ByStationPrecList)
					if !data.Visibility.ByStationPrecipitation || len(data.ByStationPrecList) == 0 {
						t.Errorf("wanted precipitation, got %#v", data.ByStationPrecList)
					}
				},
			},
			"hourlyTemp": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				Since:      time.Date(2022, 01, 27, 0, 0, 0, 0, time.Local),
				Till:       time.Date(2022, 01, 30, 0, 0, 0, 0, time.Local),
				ContractID: "TESZT",
				NeedData:   true, NeedPDF: false,
				Hourly:          true,
				NeedTemperature: true,
			},
				Check: func(t *testing.T, data mevv.V3ResultData) {
					t.Log(data.ByStationTempList)
					if !data.Visibility.ByStationTemperature || len(data.ByStationTempList) == 0 {
						t.Errorf("wanted temperature, got %#v", data.ByStationTempList)
					}
				},
			},
			"hourlyWind": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				Since:      time.Date(2022, 01, 27, 0, 0, 0, 0, time.Local),
				Till:       time.Date(2022, 01, 30, 0, 0, 0, 0, time.Local),
				ContractID: "TESZT",
				NeedData:   true, NeedPDF: false,
				Hourly:    true,
				NeedWinds: true,
			},
				Check: func(t *testing.T, data mevv.V3ResultData) {
					t.Log(data.ByStationWindList)
					if !data.Visibility.ByStationWind || len(data.ByStationWindList) == 0 {
						t.Errorf("wanted winds, got %#v", data.ByStationWindList)
					}
				},
			},
		}
}

// vim: set noet fileencoding=utf-8:
