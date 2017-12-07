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

package iohlp

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

func TestURLEncode(t *testing.T) {
	var srcArr [256]byte
	for i := range srcArr[:] {
		srcArr[i] = byte(i)
	}
	var got bytes.Buffer
	err := URLEncode(&got,
		NamedReader{
			Name:   "abraka",
			Reader: strings.NewReader("dabra"),
		},
		NamedReader{
			Name:   "filename",
			Reader: bytes.NewReader(srcArr[:]),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := url.Values(map[string][]string{
		"abraka":   []string{"dabra"},
		"filename": []string{string(srcArr[:])},
	}).Encode()

	if got.String() != want {
		t.Errorf("got\n%q\nwanted\n%q", got.String(), want)
	}
}
