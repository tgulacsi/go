/*
Copyright 2014 Tamás Gulácsi

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

package text

import (
	"io"
	"log"
	"unicode/utf8"
)

// NewStringReader wraps an io.Reader which reads UTF-8, and splits reads
// between valid runes, never inside a rune.
func NewStringReader(r io.Reader) io.Reader {
	return &stringReader{r: r}
}

// stringReader is an io.Reader which reads UTF-8, and splits reads
// between valid runes, never inside a rune.
type stringReader struct {
	r      io.Reader
	rem    [utf8.UTFMax]byte
	remLen uint8
}

func (sr *stringReader) Read(p []byte) (int, error) {
	if sr.remLen > 0 {
		copy(p, sr.rem[:sr.remLen])
	}
	n, err := sr.r.Read(p[sr.remLen:])
	n += int(sr.remLen)
	sr.remLen = 0
	if err != nil {
		return n, err
	}
	// find the last full rune
	var i int
	for i = n; i >= 0; {
		r, size := utf8.DecodeLastRune(p[:i])
		if !(size == 1 && r == utf8.RuneError) {
			break
		}
		i -= size
	}
	if i == n {
		return n, err
	}
	sr.remLen = uint8(n - i)
	log.Printf("remlLen=%d i=%d n=%d", sr.remLen, i, n)
	copy(sr.rem[:sr.remLen], p[i:n])
	return i, err
}
