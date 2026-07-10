/*
Copyright 2017, 2026 Tamás Gulácsi

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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/tgulacsi/go/mevv"
)

func TestMacroExpertVillamVilagPDF(t *testing.T) {
	username, password, cases := testCases(t)
	V := mevv.V3
	for nm, tc := range cases {
		nm, tc := nm, tc
		t.Run(nm, func(t *testing.T) {
			t.Parallel()
			ctx := zlog.NewSContext(t.Context(), zlog.NewT(t).SLog().
				With("version", V, "case", nm))
			tc.URL = V.URL() //mevv.MacroExpertURLv3Test
			ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			data, r, _, ct, err := mevv.Client{Version: V}.GetPDFData(
				ctx, username, password, tc.Options,
			)
			t.Logf("ct=%q err=%v", ct, err)
			if err != nil {
				t.Errorf("got [%s] %v.", ct, err)
				return
			}
			fh, err := os.Create(filepath.Join(t.ArtifactDir(), nm+".pdf"))
			if err != nil {
				t.Fatal(err)
			}
			defer fh.Close()
			var buf bytes.Buffer
			if r != nil {
				_, err = io.Copy(io.MultiWriter(&buf, fh), r)
				r.Close()
				if err != nil {
					t.Errorf("read response: %+v", err)
				}
				if err = fh.Close(); err != nil {
					t.Errorf("close artifact %s: %+v", fh.Name(), err)
				}
			}

			if tc.Check != nil {
				t.Log(string(data.Raw))
				tc.Check(t, data)
			}
			// t.Log("response:", buf.String())
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
			"13DaysPDF": {Options: mevv.Options{
				Address: "Érd, Fő u. 20.",
				Lat:     47.08219889999999, Lng: 18.9232321,
				At:         mevv.RoundMidnight(time.Now().AddDate(0, 0, -30)),
				Interval:   13,
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
						if found = x.Date != "" && x.MinValue != 0 && x.MaxValue != 0 && x.MinValue < x.MaxValue && x.Value != ""; found {
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
						if found = x.Date != "" && x.MinValue != 0 && x.MaxValue != 0 && x.MinValue < x.MaxValue && x.Value != ""; found {
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
						if found = x.Date != "" && x.Hour != "" && x.DistanceFromOrigin != 0 && x.Altitude != 0 && x.Settlement != ""; found {
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
						if found = x.MinValue != 0 && x.MaxValue != 0 && x.MinValue < x.MaxValue && x.Date != "" && x.DistanceFromOrigin != 0 && x.Altitude != 0 && x.Settlement != ""; found {
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

func TestPrepare(t *testing.T) {
	type Input struct {
		At, Since, Till   time.Time
		Interval          int
		Hourly, NeedRains bool
	}
	type Output struct {
		At       time.Time
		Interval int
	}
	now := time.Now().Truncate(0)
	midnight := mevv.RoundMidnight(now)
	for tName, tCase := range map[string]struct {
		In   Input
		Want Output
	}{
		"hourly": {
			In:   Input{At: now, Interval: 12, Hourly: true},
			Want: Output{At: now, Interval: 1},
		},
		"hourlyRains": {
			In:   Input{At: now, Interval: 12, Hourly: true, NeedRains: true},
			Want: Output{At: now, Interval: 3},
		},

		"5": {
			In:   Input{At: now, Interval: 5},
			Want: Output{At: midnight, Interval: 5},
		},
		"5-middle": {
			In:   Input{Since: now.AddDate(0, 0, -2), Interval: 5},
			Want: Output{At: midnight, Interval: 5},
		},

		"13": {
			In:   Input{At: now, Interval: 12},
			Want: Output{At: midnight, Interval: 13},
		},
		"13-middle": {
			In:   Input{Since: now, Interval: 6},
			Want: Output{At: midnight, Interval: 13},
		},

		"30": {
			In:   Input{At: now, Interval: 20},
			Want: Output{At: midnight, Interval: 30},
		},
		"30-middle": {
			In:   Input{Since: now, Interval: 22},
			Want: Output{At: midnight, Interval: 30},
		},

		"180": {
			In:   Input{At: now, Interval: 183},
			Want: Output{At: midnight, Interval: 180},
		},
		"180-middle": {
			In:   Input{Since: now, Interval: 159},
			Want: Output{At: midnight, Interval: 180},
		},
	} {
		t.Run(tName, func(t *testing.T) {
			opt := mevv.Options{
				At: tCase.In.At, Since: tCase.In.Since, Till: tCase.In.Till,
				Interval: tCase.In.Interval,
				Hourly:   tCase.In.Hourly, NeedRains: tCase.In.NeedRains,
			}.Prepare()
			got := Output{At: opt.At, Interval: opt.Interval}
			t.Log(got)
			if d := cmp.Diff(tCase.Want, got); d != "" {
				t.Errorf("%s: got %#v, wanted %#v", d, got, tCase.Want)
			}
		})
	}
}

func TestRoundMidnight(t *testing.T) {
	loc := time.FixedZone("XXX", 3600)
	now := time.Now().In(loc)
	if got, want := mevv.RoundMidnight(now).Format("15:04:05.999999999-0700"), "00:00:00+0100"; got != want {
		t.Errorf("got %s wanted %s", got, want)
	}
}
