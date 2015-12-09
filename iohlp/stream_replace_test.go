// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package iohlp

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestStreamReplace(t *testing.T) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, NewStreamReplacer(strings.NewReader("abc"), []byte("b"), []byte("ac"))); err != nil {
		t.Error(err)
	}
	if buf.String() != "aacc" {
		t.Errorf("got %q wanted aacc.", buf.String())
	}

}

func TestBytesReplacer(t *testing.T) {
	pairs := make([][]byte, 0, 4)
	for i, elt := range []struct {
		pairs   []string
		in, out string
	}{
		{[]string{"a", "A"}, "bbb", "bbb"},
		{[]string{"b", "B"}, "bbb", "BBB"},
		{[]string{"b", "ac"}, "abc", "aacc"},
	} {
		pairs = pairs[:len(elt.pairs)]
		for j, p := range elt.pairs {
			pairs[j] = []byte(p)
		}
		got := NewBytesReplacer(pairs...).Replace([]byte(elt.in))
		if !bytes.Equal(got, []byte(elt.out)) {
			t.Errorf("%d. got %q, awaited %q.", i, string(got), elt.out)
		}
	}
}
