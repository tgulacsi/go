// Copyright 2025 Tamás Gulácsi. All rights reserved
//
// SPDX-License-Identifier: Apache-2.0

package iohlp_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/tgulacsi/go/iohlp"
)

func TestMultiReaderAt(t *testing.T) {
	slog.SetDefault(zlog.NewT(t).SLog())
	var b [16384]byte
	rand.Read(b[:])
	want := string(b[:])
	for i := 0; i < 10; i++ {
		parts := make([]iohlp.SizeReaderAt, max(1, i<<i))
		length := len(want) / len(parts)
		s := want
		for i := 0; i < len(parts)-1; i++ {
			parts[i], s = strings.NewReader(s[:length]), s[length:]
		}
		parts[len(parts)-1] = strings.NewReader(s)

		t.Logf("%d parts, length=%d", len(parts), length)
		mra := iohlp.NewMultiReaderAt(parts...)
		sr := io.NewSectionReader(mra, 0, mra.Size())
		var got bytes.Buffer
		got.Grow(len(want))
		n, err := io.CopyBuffer(&got, sr, make([]byte, max(1, length-1)))
		if err != nil {
			t.Fatal(err)
		}
		if n != int64(len(want)) {
			t.Errorf("got %d, wanted %d", n, len(want))
		}
		if d := cmp.Diff(want, got.String()); d != "" {
			t.Error(d)
		}
	}
}
