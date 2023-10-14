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
				Since:              time.Date(2019, 05, 27, 0, 0, 0, 0, time.Local),
				Till:               time.Date(2019, 05, 30, 0, 0, 0, 0, time.Local),
				ContractID:         "TESZT",
				NeedData:           true,
				NeedIce:            true,
				NeedRains:          true,
				NeedRainsIntensity: true,
				NeedThunders:       true,
				NeedWinds:          true,
				NeedTemperature:    true,
				WithStatistics:     true,
			},
				Check: func(t *testing.T, data mevv.V3ResultData) {
					v := data.Visibility
					t.Log("ice:", data.DailyListIce)
					if !v.DailyIce || len(data.DailyListIce) == 0 {
						t.Error("wanted ice")
					}
					var found bool
					for _, x := range data.DailyListIce {
						if found = x.Date != "" && x.Value; found {
							break
						}
					}
					if !found {
						t.Errorf("not found ice")
					}

					t.Log("lightning:", data.LightningList)
					if !v.Lightning || len(data.LightningList) == 0 {
						t.Error("wanted ice")
					}
					found = false
					for _, x := range data.LightningList {
						zone, _ := x.Zone.Float64()
						if found = !x.EventDateUTC.IsZero() && zone != 0 && x.CurrentIntensity != 0 && x.DistanceFromOrigin != 0; found {
							break
						}
					}
					if !found {
						t.Error("lightning not found")
					}

					t.Log("precip:", data.DailyListPrecipitation)
					if !v.DailyPrecipitation || len(data.DailyListPrecipitation) == 0 {
						t.Error("wanted precip")
					}
					found = false
					for _, x := range data.DailyListPrecipitation {
						if found = x.Date != "" && x.Value != 0; found {
							break
						}
					}
					if !found {
						t.Error("precip not found")
					}

					t.Log("precipIntensity:", data.DailyListPrecipitationIntensity)
					if !v.DailyPrecipitationIntensity || len(data.DailyListPrecipitationIntensity) == 0 {
						t.Error("wanted precipIntensity")
					}
					found = false
					for _, x := range data.DailyListPrecipitationIntensity {
						if found = x.Date != ""; found {
							break
						}
					}
					if !found {
						t.Error("precipIntensity not found")
					}

					t.Logf("temp: %+v", data.DailyListTemperature)
					if !v.DailyTemperature || len(data.DailyListTemperature) == 0 {
						t.Error("wanted temperature")
					}
					found = false
					for _, x := range data.DailyListTemperature {
						if found = x.Date != "" && x.MinValue != 0 && x.MaxValue != 0 && x.Value != ""; found {
							break
						}
					}
					if !found {
						t.Error("temperature not found")
					}

					t.Logf("wind: %+v", data.DailyListWind)
					if !v.DailyWind || len(data.DailyListWind) == 0 {
						t.Error("wanted wind")
					}
					found = false
					for _, x := range data.DailyListWind {
						if found = x.Date != "" && x.MinValue != 0 && x.MaxValue != 0 && x.Value != ""; found {
							break
						}
					}
					if !found {
						t.Error("wind not found")
					}

					if !v.Statistic || len(data.Statistics) == 0 {
						t.Log(data.Statistics)
						t.Error("wanted statistics")
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
					t.Logf("precip: %+v", data.ByStationPrecipList)
					if !data.Visibility.ByStationPrecipitation || len(data.ByStationPrecipList) == 0 {
						t.Errorf("wanted precipitation, got %#v", data.ByStationPrecipList)
					}
					var found bool
					for _, x := range data.ByStationPrecipList {
						if found = x.Date != "" && x.DistanceFromOrigin != 0 && x.Altitude != 0 && x.Settlement != ""; found {
							break
						}
					}
					if !found {
						t.Error("precip not found")
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
					t.Logf("temp: %+v", data.ByStationTempList)
					if !data.Visibility.ByStationTemperature || len(data.ByStationTempList) == 0 {
						t.Errorf("wanted temperature, got %#v", data.ByStationTempList)
					}
					var found bool
					for _, x := range data.ByStationTempList {
						if found = x.MinValue != 0 && x.MaxValue != 0 && x.Date != "" && x.DistanceFromOrigin != 0 && x.Altitude != 0 && x.Settlement != ""; found {
							break
						}
					}
					if !found {
						t.Error("temp not found")
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
					t.Logf("wind: %+v", data.ByStationWindList)
					if !data.Visibility.ByStationWind || len(data.ByStationWindList) == 0 {
						t.Errorf("wanted winds, got %#v", data.ByStationWindList)
					}
					var found bool
					for _, x := range data.ByStationWindList {
						if found = x.Direction != "" && x.MaxGustKmH != 0 && x.Date != "" && x.DistanceFromOrigin != 0 && x.Altitude != 0 && x.Settlement != ""; found {
							break
						}
					}
					if !found {
						t.Error("wind not found")
					}
				},
			},
		}
}

// vim: set noet fileencoding=utf-8:
