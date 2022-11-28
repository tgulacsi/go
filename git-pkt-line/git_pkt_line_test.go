package gitpktline_test

import (
	"strings"
	"testing"

	gitpktline "github.com/tgulacsi/go/git-pkt-line"
)

func TestReadWrite(t *testing.T) {
	var a [1024]byte
	var buf strings.Builder
	for k, v := range map[string]string{
		"a\n":      "0006a\n",
		"a":        "0005a",
		"foobar\n": "000bfoobar\n",
		"":         "0004",
	} {
		r := gitpktline.NewReader(strings.NewReader(v))
		n, err := r.Read(a[:])
		if err != nil {
			t.Errorf("%q: read %+v", k, err)
		} else if got := string(a[:n]); got != k {
			t.Errorf("%q: got %q", k, got)
		}

		buf.Reset()
		w := gitpktline.Writer{Writer: &buf}
		if _, err = w.Write([]byte(k)); err != nil {
			t.Errorf("%q: write %+v", k, err)
		} else if got := buf.String(); got != v {
			t.Errorf("%q: got %q wanted %q", k, got, v)
		}
	}
}
