// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package iohlp

import (
	"bytes"
	"io"
)

// NewStreamReplacer returns an io.Reader in which all non-overlapping
// patterns are replaced to their replacement pairs (such as a strings.Replacer on strings).
func NewStreamReplacer(r io.Reader, patternReplacementPairs ...[]byte) io.Reader {
	repl := NewBytesReplacer(patternReplacementPairs...)
	maxLen := repl.MaxPatternLen() - 1
	if maxLen == 0 {
		maxLen = 1
	}
	pr, pw := io.Pipe()
	go func() {
		n := 4096
		for n < maxLen {
			n <<= 1
		}
		scratch := make([]byte, n)
		for {
			n, readErr := io.ReadAtLeast(r, scratch, maxLen)
			if n == 0 && readErr == nil {
				break
			}
			var writeErr error
			if n > 0 {
				scratch = repl.Replace(scratch[:n])
				if readErr != nil {
					n, writeErr = pw.Write(scratch)
				} else {
					n, writeErr = pw.Write(scratch[:len(scratch)-maxLen])
				}
				scratch = scratch[n:]
			}
			if readErr != nil {
				if len(scratch) > 0 {
					_, _ = pw.Write(scratch)
				}
				pw.CloseWithError(readErr)
				break
			}
			if writeErr != nil {
				pw.CloseWithError(writeErr)
				break
			}
		}
	}()
	return pr
}

// NewBytesReplacer returns a Replacer, such as strings.Replacer, but for []byte.
func NewBytesReplacer(patternReplacementPairs ...[]byte) BytesReplacer {
	pairs := make([][2][]byte, len(patternReplacementPairs)/2)
	for i := 0; i < len(patternReplacementPairs); i += 2 {
		j := i / 2
		pairs[j] = [2][]byte{patternReplacementPairs[i], patternReplacementPairs[i+1]}
	}
	return BytesReplacer(pairs)
}

// BytesReplacer is a Replacer for bytes.
type BytesReplacer [][2][]byte

// Replace as strings.Replacer would do.
func (br BytesReplacer) Replace(p []byte) []byte {
	for _, pair := range br {
		p = bytes.Replace(p, pair[0], pair[1], -1)
	}
	return p
}

func (br BytesReplacer) MaxPatternLen() int {
	var maxLen int
	for _, pair := range br {
		if n := len(pair[0]); n > maxLen {
			maxLen = n
		}
	}
	return maxLen
}
