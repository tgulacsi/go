// Copyright 2017, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

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
