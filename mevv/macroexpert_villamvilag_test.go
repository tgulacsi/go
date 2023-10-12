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
	"os"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/mevv"
)

func TestMacroExpertVillamVilagPDF(t *testing.T) {
	username, password := os.Getenv("MEVV_USERNAME"), os.Getenv("MEVV_PASSWORD")
	testHost := os.Getenv("MEVV_HOST")
	if username == "" && password == "" {
		t.Logf("Environment variables MEVV_USERNAME and MEVV_PASSWORD are empty, reading from .password")
		var err error
		username, password, err = mevv.ReadUserPassw(".password")
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx := zlog.NewSContext(context.Background(), zlog.NewT(t).SLog())
	var buf bytes.Buffer
	for i, tc := range []struct {
		mevv.Options
		ErrOK bool
	}{
		{mevv.Options{
			Address: "Érd, Fő u. 20.",
			Lat:     47.08219889999999, Lng: 18.9232321,
			Since:      time.Date(2019, 01, 27, 0, 0, 0, 0, time.Local),
			Till:       time.Date(2019, 01, 30, 0, 0, 0, 0, time.Local),
			ContractID: "TESZT",
			NeedData:   true, NeedPDF: true,
			Host: testHost,
		}, false},
		{mevv.Options{
			Address: "Érd, Fő u. 20.",
			Lat:     47.08219889999999, Lng: 18.9232321,
			Since:      time.Date(2019, 01, 27, 0, 0, 0, 0, time.Local),
			Till:       time.Date(2019, 01, 30, 0, 0, 0, 0, time.Local),
			ContractID: "TESZT",
			NeedData:   true, NeedPDF: true,
			NeedThunders: true,
			NeedWinds:    true,
			NeedRains:    true, NeedRainsIntensity: true,
			Host: testHost,
		}, false},
	} {

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		r, _, ct, err := mevv.V3test.GetPDF(ctx, username, password, tc.Options)
		cancel()
		t.Logf("%d. ct=%q err=%v", i, ct, err)
		if err != nil && !tc.ErrOK {
			t.Errorf("%d. got [%s] %v.", i, ct, err)
			continue
		}
		buf.Reset()
		if r != nil {
			_, err = io.Copy(&buf, r)
			r.Close()
			if err != nil {
				t.Errorf("read response: %+v", err)
			}
		}
		// t.Log("response:", buf.String())
		if err == nil && tc.ErrOK {
			t.Errorf("%d. wanted error, got [%s] %q.", i, ct, buf.String())
			continue
		}
	}
}

// vim: set noet fileencoding=utf-8:
