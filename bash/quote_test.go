// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package bash_test

import (
	"strings"
	"testing"

	"github.com/tgulacsi/go/bash"
)

func TestQuote(t *testing.T) {
	var buf strings.Builder
	for out, in := range map[string][]string{
		"a":       {"a"},
		"a b":     {"a", "b"},
		"'c'":     {"'c'"},
		`"\"a\""`: {`"a"`},
	} {
		buf.Reset()
		if err := bash.QuoteWrite(&buf, in...); err != nil {
			t.Errorf("%s: quote: %+v", out, err)
		} else if got, want := buf.String(), out; got != want {
			t.Errorf("%s: quote got %q, wanted %q", out, got, want)
		}
	}

	for in, out := range map[string]string{
		"a":          "a",
		"a b":        "a b",
		"'c'":        "c",
		`"a"`:        `a`,
		`"a b \" c"`: "a b \" c",
	} {
		if got, want := bash.Unquote(in), out; got != want {
			t.Errorf("unquote %q: got %q, wanted %q", in, got, want)
		}
	}
}
