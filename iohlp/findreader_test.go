// Copyright 2019, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tgulacsi/go/iohlp"
)

func TestFindReader(t *testing.T) {
	var buf bytes.Buffer
	buf.Grow(1 << 20)
	var n int
	for n = 0; buf.Len() < (1<<20)-10; n++ {
		fmt.Fprintf(&buf, "%09d\n", n)
	}
	haystack := buf.Bytes()
	if i, err := iohlp.FindReader(bytes.NewReader(haystack), []byte("not-in-there")); err != nil {
		t.Fatal(err)
	} else if i >= 0 {
		t.Fatalf("found what is not in there at %d", i)
	}
	for _, j := range []int{0, 1, 100, n / 2, n - 1, n} {
		needle := fmt.Sprintf("%09d", j)
		t.Log("needle:", needle)
		i, err := iohlp.FindReaderSize(bytes.NewReader(haystack), []byte(needle), 64)
		if err != nil {
			t.Fatal(err)
		}
		if want := bytes.Index(haystack, []byte(needle)); i != want {
			t.Errorf("got %d for %q (%q), wanted %d", i, needle, string(haystack[i-1:i+len(needle)+1]), want)
		}
	}
}
