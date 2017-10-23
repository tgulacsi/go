// Copyright 2017 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package stream_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tgulacsi/go/stream"
)

func TestTrimSpace(t *testing.T) {
	var buf bytes.Buffer
	for tN, tC := range []string{
		"abc",
		"  \n\t\va b c\n\t\v ",
	} {
		buf.Reset()
		w := stream.NewTrimSpace(&buf)
		for i := range []byte(tC) {
			if _, err := w.Write([]byte{tC[i]}); err != nil {
				t.Fatal(tC, err)
			}
		}
		w.Close()
		want := strings.TrimSpace(tC)
		got := buf.String()
		if want != got {
			t.Errorf("%d. got %q, wanted %q.", tN, got, want)
		}
	}
}

func TestTrimFix(t *testing.T) {
	var buf bytes.Buffer
	for tN, tC := range []string{
		"abc",
		" [a b \"c\"] ",

		" [{\"row_num\":1,\"contract_number\":11504108,\"member_code\":912944,\"modkod\":\"21211\",\"modrnev\":\"DEF/KATY\",\"bid_id\":55569182,\"contract_status\":\"26\",\"contract_status_name\":\"DÍJ SZEMPONTJÁBÓL ÁTDOLGOZOTT SZERZŐDÉS\",\"contract_status_short\":\"ÉLŐ\",\"contract_recording_date\":\"2015-11-18 22:39:09 +0200\",\"contract_btkezd\":\"2015-11-19 00:00:00 +0200\",\"contract_begin_date\":\"2015-11-18 00:00:00 +0200\",\"contract_balance_date\":\"2017-11-18 00:00:00 +0200\",\"contract_future_balance_date\":\"2017-11-18 00:00:00 +0200\",\"contract_yearly_price\":1825,\"contract_anniversary\":\"11-18\",\"client_name\":\"Horváth Ádám\",\"client_code\":2825227,\"car_plate\":\"MKN378\",\"dealer_code\":\"0001002110\",\"dealer_name\":\"Köbe-Cc2\",\"kockhely_cim\":\"   \",\"client_ppid\":\"10980\",\"client_city\":\"BUDAPEST\"},{\"row_num\":14,\"contract_number\":11321482,\"member_code\":912944,\"modkod\":\"23001\",\"modrnev\":\"UTAS/LM\",\"bid_id\":311003614,\"contract_status\":\"65\",\"contract_status_name\":\"LEJÁRAT MIATT TÖRÖLT SZERZŐDÉS\",\"contract_status_short\":\"TÖRÖLT\",\"contract_recording_date\":\"2015-04-01 12:43:46 +0200\",\"contract_btkezd\":\"2015-04-01 00:00:00 +0200\",\"contract_begin_date\":\"2015-04-01 00:00:00 +0200\",\"contract_deletion_valid_from\":\"2015-04-02 00:00:00 +0200\",\"contract_balance_date\":\"2015-03-31 00:00:00 +0200\",\"contract_future_balance_date\":\"2015-03-31 00:00:00 +0200\",\"contract_yearly_price\":900,\"contract_anniversary\":\"03-31\",\"elvi_dijhatralek\":900,\"client_name\":\"Horváth Ádám\",\"client_code\":2825227,\"dealer_code\":\"0000400001\",\"dealer_name\":\"KÖBE - Net\",\"kockhely_cim\":\"   \",\"client_ppid\":\"10980\",\"client_city\":\"BUDAPEST\"}] ",
	} {
		buf.Reset()
		w := stream.NewTrimFix(&buf, " [", "] ")
		for i := range []byte(tC) {
			if _, err := w.Write([]byte{tC[i]}); err != nil {
				t.Fatal(tC, err)
			}
		}
		w.Close()
		want := strings.TrimSuffix(strings.TrimPrefix(tC, " ["), "] ")
		got := buf.String()
		if want != got {
			t.Errorf("%d. got\n%q,\n\twanted\n%q.", tN, got, want)
			break
		}
	}
}
