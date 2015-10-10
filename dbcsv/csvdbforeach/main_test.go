package main

import (
	"testing"
	"time"
)

func TestStrToDate(t *testing.T) {
	for i, tc := range []struct {
		in    string
		out   time.Time
		errOk bool
	}{
		{"1978-1225", time.Date(1978, 12, 25, 0, 0, 0, 0, time.UTC), false},
	} {
		gotI, err := strToDate(tc.in)
		if err != nil && !tc.errOk {
			t.Errorf("%d. error for %q: %v", i, tc.in, err)
			continue
		}
		got, ok := gotI.(time.Time)
		if !ok {
			t.Errorf("%d. got %v (%T), not time.Time for %q.", i, gotI, gotI, tc.in)
			continue
		}
		if !got.Equal(tc.out) {
			t.Errorf("%d. got %s, want %s.", i, got, tc.out)
		}
	}
}
