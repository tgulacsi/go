/*
Copyright 2013 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpreq

import (
	"testing"
)

func TestParseMediaRange(t *testing.T) {
	var (
		err error
		mr  mediaRange
	)
	for i, s := range []struct {
		media   string
		awaited mediaRange
	}{
		{"application/xbel+xml", mediaRange{typ: "application", subtyp: "xbel+xml", q: 1}},
		{"text/xml", mediaRange{typ: "text", subtyp: "xml", q: 1}},
		{"application/xml;q=0.5", mediaRange{typ: "application", subtyp: "xml", q: 0.5}},
	} {
		if mr, err = parseMediaRange(s.media); err != nil {
			t.Errorf("%d. media=%q ERROR %s", i, s.media, err)
			t.Fail()
			continue
		}
		if !(mr.typ == s.awaited.typ && mr.subtyp == s.awaited.subtyp &&
			mr.q == s.awaited.q) {
			t.Errorf("%d. awaited %v got %v", i, s.awaited, mr)
		}
	}
}

func TestBestAcceptMatch(t *testing.T) {
	var (
		m   string
		err error
	)
	for i, s := range []struct{ supported, accepted, awaited string }{
		{"application/xbel+xml, text/xml",
			"text/*;q=0.5,*/*; q=0.1",
			"text/xml"},
	} {
		if m, err = BestAcceptMatch(s.supported, s.accepted); err != nil {
			t.Errorf("%d. supported=%q accepted=%q ERROR %s",
				i, s.supported, s.accepted, err)
			t.Fail()
			continue
		}
		if m != s.awaited {
			t.Errorf("%d. awaited %q got %q", i, s.awaited, m)
			t.Fail()
		}
	}
}
