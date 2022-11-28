package gitpktline_test

import (
	"io"
	"strings"
	"testing"

	gitpktline "github.com/tgulacsi/go/git-pkt-line"
)

func TestReadWrite(t *testing.T) {
	var a [1024]byte
	var buf strings.Builder
	for k, v := range rwTestCases {
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

func FuzzReadWrite(f *testing.F) {
	for _, v := range rwTestCases {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if len(in) < 4 {
			t.Skip(io.ErrShortBuffer)
		}
		r := gitpktline.NewReader(strings.NewReader(in))
		b, err := io.ReadAll(r)
		if err != nil {
			t.Skip(err.Error())
			return
		}
		t.Logf("Read %q: %q (%d)", in, b, len(b))
		var buf strings.Builder
		w := gitpktline.NewWriter(&buf)
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != in {
			t.Fatalf("%q: got %q wanted %q", b, got, in)
		}
	})
}

var rwTestCases = map[string]string{
	"a\n":      "0006a\n",
	"a":        "0005a",
	"foobar\n": "000bfoobar\n",
	"":         "0004",
	" ":        "0005 ",
	"0":        "00050",
}
