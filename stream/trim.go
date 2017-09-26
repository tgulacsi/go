// Copyright 2017 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package stream

import (
	"bytes"
	"io"
	"unicode"

	"golang.org/x/text/transform"
)

// NewTrimSpace returns an io.Writer which trims pre- and postfix space.
func NewTrimSpace(w io.Writer) io.WriteCloser {
	return transform.NewWriter(w, &trimSpacesTransform{})
}

type trimSpacesTransform struct {
	middle bool
	buf    []byte
}

func (ts *trimSpacesTransform) Reset() { ts.middle, ts.buf = false, ts.buf[:0] }
func (ts *trimSpacesTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	x := append(ts.buf, src...)
	ts.buf = ts.buf[:0]
	if !ts.middle {
		x = bytes.TrimLeftFunc(x, unicode.IsSpace)
		if len(x) == 0 {
			return 0, len(src), nil
		}
		ts.middle = true
	}
	if atEOF {
		x = bytes.TrimRightFunc(x, unicode.IsSpace)
		return copy(dst, x), len(src), nil
	}
	y := bytes.TrimRightFunc(x, unicode.IsSpace)
	if len(y) < len(x) {
		ts.buf = append(ts.buf, x[len(y):]...)
		x = y
	}
	return copy(dst, x), len(src), nil
}

// NewTrimFix trims the given prefix, suffix on Write.
func NewTrimFix(w io.Writer, prefix, suffix string) io.WriteCloser {
	return transform.NewWriter(w, &trimTransform{prefix: []byte(prefix), suffix: []byte(suffix)})
}

type trimTransform struct {
	prefix, suffix []byte
	buf            []byte
	middle         bool
}

func (tw *trimTransform) Reset() { tw.middle, tw.buf = false, tw.buf[:0] }
func (tw *trimTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	if !tw.middle && len(tw.buf)+len(src) < len(tw.prefix) {
		return 0, 0, transform.ErrShortSrc
	}
	if !atEOF && tw.middle && len(tw.buf)+len(src)-1 < len(tw.suffix) {
		return 0, 0, transform.ErrShortSrc
	}
	x := append(tw.buf, src...)
	tw.buf = tw.buf[:0]
	if !tw.middle {
		x = bytes.TrimPrefix(x, tw.prefix)
		if len(x) == 0 {
			return 0, len(src), nil
		}
		tw.middle = true
	}
	if atEOF {
		x = bytes.TrimSuffix(x, tw.suffix)
		return copy(dst, x), len(src), nil
	}
	y := bytes.TrimSuffix(x, tw.suffix)
	if len(y) < len(x) {
		tw.buf = append(tw.buf, x[len(y):]...)
		x = y
	}
	return copy(dst, x), len(src), nil
}
