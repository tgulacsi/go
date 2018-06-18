/*
Copyright 2015 Tamás Gulácsi
Copyright 2013 The Camlistore Authors

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

package byteutil

import "testing"

func TestByteIndexFold(t *testing.T) {
	for i, tc := range []struct {
		haystack, needle string
		want             int
	}{
		{"body", "body", 0},
	} {
		got := ByteIndexFold([]byte(tc.haystack), []byte(tc.needle))
		if got != tc.want {
			t.Errorf("%d. got %d, wanted %d.", i, got, tc.want)
		}
	}
}

func TestByteHasPrefixFold(t *testing.T) {
	for i, tc := range []struct {
		haystack, needle string
		want             bool
	}{
		{"body", "body", true},
	} {
		got := ByteHasPrefixFold([]byte(tc.haystack), []byte(tc.needle))
		if got != tc.want {
			t.Errorf("%d. got %t, wanted %t.", i, got, tc.want)
		}
	}
}
