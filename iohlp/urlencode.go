/*
Copyright 2017 Tamás Gulácsi

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

package iohlp

import (
	"bufio"
	"io"
)

type NamedReader struct {
	Name string
	io.Reader
}

// URLEncode encodes the Name:Reader pairs just as url.Values.Encode does.
func URLEncode(w io.Writer, keyvals ...NamedReader) error {
	if len(keyvals) == 0 {
		return nil
	}
	ew := escapeWriter{bw: bufio.NewWriter(w)}
	defer ew.bw.Flush()
	for i, kv := range keyvals {
		if i != 0 {
			if err := ew.bw.WriteByte('&'); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(ew, kv.Name); err != nil {
			return err
		}
		if err := ew.bw.WriteByte('='); err != nil {
			return err
		}
		if _, err := io.Copy(ew, kv.Reader); err != nil {
			return err
		}
	}
	return ew.bw.Flush()
}

type escapeWriter struct {
	bw *bufio.Writer
}

func (ew escapeWriter) Close() error {
	return ew.bw.Flush()
}
func (ew escapeWriter) Write(p []byte) (int, error) {
	spaceCount, hexCount := 0, 0
	for _, c := range p {
		if shouldEscape(c) {
			if c == ' ' {
				spaceCount++
			} else {
				hexCount++
			}
		}
	}

	if spaceCount == 0 && hexCount == 0 {
		return ew.bw.Write(p)
	}

	var a [3]byte
	a[0] = '%'
	for i, c := range p {
		if c == ' ' {
			if err := ew.bw.WriteByte('+'); err != nil {
				return i, err
			}
		} else if shouldEscape(c) {
			a[1] = "0123456789ABCDEF"[c>>4]
			a[2] = "0123456789ABCDEF"[c&15]
			if _, err := ew.bw.Write(a[:]); err != nil {
				return i, err
			}
		} else {
			if err := ew.bw.WriteByte(c); err != nil {
				return i, err
			}
		}
	}
	return len(p), nil
}

// Return true if the specified character should be escaped when
// appearing in a URL string, according to RFC 3986.
//
// Please be informed that for now shouldEscape does not check all
// reserved characters correctly. See golang.org/issue/5684.
func shouldEscape(c byte) bool {
	// §2.3 Unreserved characters (alphanum)
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}
	switch c {
	case '-', '_', '.', '~': // §2.3 Unreserved characters (mark)
		return false
	}
	// Everything else must be escaped.
	return true
}
