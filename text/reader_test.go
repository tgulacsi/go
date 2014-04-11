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
	} {
		res, err := ioutil.ReadAll(
			NewDecodingReader(strings.NewReader(tup.encoded), GetEncoding(tup.charset)))
		if err != nil {
			t.Errorf("%d. error reading: %v", i, err)
		}
		if string(res) != tup.decoded {
			t.Errorf("%d. mismatch: got %q awaited %q", i, res, tup.decoded)
		}
	}
}

func TestReplacementReader(t *testing.T) {
	for i, tup := range []struct {
		encoded, decoded string
	}{
		{"\xf5\xf6abraka dabra", "\ufffd\ufffdabraka dabra"},
	} {
		res, err := ioutil.ReadAll(
			NewReplacementReader(strings.NewReader(tup.encoded)))
		if err != nil {
			t.Errorf("%d. error reading: %v", i, err)
		}
		if string(res) != tup.decoded {
			t.Errorf("%d. mismatch: got %q awaited %q", i, res, tup.decoded)
		}
	}
}
