// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package checksum_test

import (
	"testing"

	"github.com/tgulacsi/go/checksum"
)

func TestCDV(t *testing.T) {
	for i, tc := range []struct {
		text string
		cs   uint8
	}{
		{"918100024", 4},
		// for i in range(min(len(text), 8), 0, -1):
		// ...   j =  {1: 19, 2: 17, 3: 13, 4: 11, 5: 7, 6: 5, 7: 3, 8: 1}[i]
		// ...   sum += j * int(text[i - 1])
		// ...   print "i=%d mul=%d r=%s => sum=%d (%d)" % (i, j, text[i-1], sum, sum % 97)
		// ...
		// i=8 mul=1 r=2 => i=2 (2)
		// i=7 mul=3 r=0 => i=2 (2)
		// i=6 mul=5 r=0 => i=2 (2)
		// i=5 mul=7 r=0 => i=2 (2)
		// i=4 mul=11 r=1 => i=13 (13)
		// i=3 mul=13 r=8 => i=117 (20)
		// i=2 mul=17 r=1 => i=134 (37)
		// i=1 mul=19 r=9 => i=305 (14)

	} {
		got := checksum.CDV.Calculate(tc.text)
		if got != tc.cs {
			t.Errorf("%d. got %d, want %d.", i, got, tc.cs)
		}
		await := tc.text + string([]byte{got + '0'})
		if !checksum.CDV.IsValid(await) {
			t.Errorf("%d. %q shall be valid.", i, await)
		}
	}
}

func TestEAN8(t *testing.T) {
	for i, tc := range []struct {
		text string
		cs   uint8
	}{
		{"7351353", 7},
	} {
		got := checksum.EAN8.Calculate(tc.text)
		if got != tc.cs {
			t.Errorf("%d. got %d, want %d.", i, got, tc.cs)
		}
		await := tc.text + string([]byte{got + '0'})
		if !checksum.EAN8.IsValid(await) {
			t.Errorf("%d. %q shall be valid.", i, await)
		}
	}
}

func TestCvtEAN8(t *testing.T) {
	for i, tc := range []struct {
		text string
		cs   uint8
	}{
		{"6528249", 7},
	} {
		got := checksum.CvtEAN8.Calculate(tc.text)
		if got != tc.cs {
			t.Errorf("%d. got %d, want %d.", i, got, tc.cs)
		}
		await := string([]byte{got + '0'}) + tc.text
		if !checksum.CvtEAN8.IsValid(await) {
			t.Errorf("%d. %q shall be valid.", i, await)
		}
	}
}
