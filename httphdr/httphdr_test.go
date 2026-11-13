//  Copyright 2017, 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0

package httphdr

import (
	"testing"
	"time"
)

func TestAccept(t *testing.T) {
	a := Accept([]string{"application/pdf", "application/zip", "{*/pdf"})
	for k, want := range map[string]int{
		"application/pdf": 0,
		"application/zip": 1,
	} {
		t.Log(k+":", parseMediaRanges(k))
		if got := a.Match(k); got != want {
			t.Errorf("%q got %d, wanted %d.", k, got, want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	dt := time.Date(1994, time.November, 6, 8, 49, 37, 0, time.UTC)
	abs := func(d time.Duration) time.Duration { return max(d, -d) }
	for in, want := range map[string]time.Time{
		"":                               time.Time{},
		"120":                            time.Now().Add(2 * time.Minute),
		"Sun, 06 Nov 1994 08:49:37 GMT":  dt, // IMF-fixdate
		"Sunday, 06-Nov-94 08:49:37 GMT": dt, // obsolete RFC 850 format
		"Sun Nov  6 08:49:37 1994":       dt, // ANSI C's asctime() format
	} {
		if got := ParseRetryAfter(in); !got.Equal(want) && abs(got.Sub(want)) >= time.Second {
			t.Errorf("%s: got %s, wanted %s.", in, got, want)
		}
	}
}
