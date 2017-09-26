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
		" [a b c] ",
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
			t.Errorf("%d. got %q, wanted %q.", tN, got, want)
		}
	}
}
