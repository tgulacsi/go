package term_test

import (
	"testing"

	"github.com/tgulacsi/go/term"
)

func TestGetLangEncodingName(t *testing.T) {
	for _, inOut := range [][2]string{
		{"iso885912", "iso8859-12"},
		{"iso8859-2", "iso8859-2"},
		{"ISO8859_2", "iso8859-2"},
		{"uTf-8", "utf-8"},
	} {
		if got := term.GetLangEncodingName(inOut[0]); got != inOut[1] {
			t.Errorf("%q got %q, wanted %q", inOut[0], got, inOut[1])
		}
	}
}
