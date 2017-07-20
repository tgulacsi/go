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

package mevv_test

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tgulacsi/go/mevv"
)

func TestMacroExpertVillamVilagPDF(t *testing.T) {
	username, password := os.Getenv("MEVV_USERNAME"), os.Getenv("MEVV_PASSWORD")
	if username == "" && password == "" {
		t.Logf("Environment variables MEVV_USERNAME and MEVV_PASSWORD are empty, reading from .password")
		var err error
		username, password, err = mevv.ReadUserPassw(".password")
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, tc := range []struct {
		mevv.Options
		ErrOK bool
	}{
		{mevv.Options{
			Address:    "Budapest, Venyige utca 3",
			Since:      time.Date(2015, 01, 27, 0, 0, 0, 0, time.Local),
			Till:       time.Date(2015, 01, 30, 0, 0, 0, 0, time.Local),
			Lat:        47.47809,
			Lng:        19.16839,
			ContractID: "TESZT",
		}, true},
		{mevv.Options{
			Address:      "Budapest, Venyige utca 3",
			Since:        time.Date(2015, 01, 27, 0, 0, 0, 0, time.Local),
			Till:         time.Date(2015, 01, 30, 0, 0, 0, 0, time.Local),
			Lat:          47.47809,
			Lng:          19.16839,
			ContractID:   "TESZT",
			NeedThunders: true,
			NeedWinds:    true,
			NeedRains:    true, NeedRainsIntensity: true,
		}, false},
	} {

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		r, _, ct, err := mevv.V2.GetPDF(ctx, username, password, tc.Options)
		cancel()
		t.Logf("%d. ct=%q err=%v", i, ct, err)
		if r != nil {
			defer r.Close()
		}
		if err == nil && tc.ErrOK {
			b, _ := ioutil.ReadAll(&io.LimitedReader{R: r, N: 1024})
			t.Errorf("%d. wanted error, got [%s] %q.", i, ct, b)
			continue
		}
		if err != nil && !tc.ErrOK {
			t.Errorf("%d. got [%s] %v.", i, ct, err)
			continue
		}
	}
}

// vim: set noet fileencoding=utf-8:
