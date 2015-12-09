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

	expected := "<methodCall><methodName>Some.Method</methodName><params><param><value><int>123</int></value></param><param><value><double>3.145926</double></value></param><param><value><string>Hello, World!</string></value></param><param><value><boolean>0</boolean></value></param><param><value><struct><member><name>Foo</name><value><int>42</int></value></member><member><name>Bar</name><value><string>I'm Bar</string></value></member><member><name>Data</name><value><array><data><value><int>1</int></value><value><int>2</int></value><value><int>3</int></value></data></array></value></member></struct></value></param><param><value><dateTime.iso8601>20120717T14:08:55</dateTime.iso8601></value></param><param><value><base64>eW91IGNhbid0IHJlYWQgdGhpcyE=</base64></value></param></params></methodCall>"
	buf.Reset()
	if _, err := io.Copy(&buf,
		NewStreamReplacer(
			strings.NewReader(
				strings.NewReplacer(
					"</name><value><string>", "</name><string>",
					"</string></value></param>", "</string></param>",
				).Replace(expected),
			),
			[]byte("</name><string>"), []byte("</name><value><string>"),
			[]byte("</string></param>"), []byte("</string></value></param>"),
		),
	); err != nil {
		t.Error(err)
	}

	if buf.String() != expected {
		t.Errorf("\tgot\n%q\n\twanted\n%q", buf.String(), expected)
	}
}

func TestBytesReplacer(t *testing.T) {
	var buf bytes.Buffer
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

		buf.Reset()
		if _, err := io.Copy(&buf,
			NewStreamReplacer(strings.NewReader(elt.in),
				pairs...),
		); err != nil {
			t.Error(err)
		}
		if buf.String() != elt.out {
			t.Errorf("got %q wanted %q.", buf.String(), elt.out)
		}
	}
}
