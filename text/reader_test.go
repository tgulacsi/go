/*
Copyright 2014 Tamás Gulácsi

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

package text

import (
	"io/ioutil"
	"strings"
	"testing"
)

func TestDecodingReader(t *testing.T) {
	for i, tup := range []struct {
		charset, encoded, decoded string
	}{
		{"iso8859-2", "\xe1rv\xedzt\xfbr\xf5 t\xfck\xf6rf\xfar\xf3g\xe9p", "árvíztűrő tükörfúrógép"},
		{"utf-8", "\xf5\xf6abraka dabra", "\ufffd\ufffdabraka dabra"},
	} {
		enc := GetEncoding(tup.charset)
		res, err := ioutil.ReadAll(NewDecodingReader(strings.NewReader(tup.encoded), enc))
		if err != nil {
			t.Errorf("%d. error reading: %v", i, err)
		}
		if string(res) != tup.decoded {
			t.Errorf("%d. mismatch: got %q (% x) awaited %q", i, res, res, tup.decoded)
		}

		got, err := Decode([]byte(tup.encoded), enc)
		if err != nil {
			t.Errorf("%d. error decoding: %v", i, err)
		}
		if got != tup.decoded {
			t.Errorf("%d. mismatch: got %q awaited %q", i, res, tup.decoded)
		}
	}
}
