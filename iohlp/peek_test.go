// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"io"
	"strings"
	"testing"
)

func TestPeek(t *testing.T) {
	const data = "abcdefghijklmn"
	b, r, err := Peek(strings.NewReader(data), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b[:min(len(b), 3)]); got != data[:3] {
		t.Fatalf("got %s, wanted %s", got, data[:3])
	}
	if b, err = io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != data {
		t.Errorf("got %s, wanted %s", got, data)
	}
}
