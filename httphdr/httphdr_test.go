//  Copyright 2017, 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0

package httphdr

import (
	"testing"
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
