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
	//"log"
	"unicode"

	"golang.org/x/text/transform"
)

// NewTrimSpace returns an io.Writer which trims pre- and postfix space.
func NewTrimSpace(w io.Writer) io.WriteCloser {
	return transform.NewWriter(w, &trimSpacesTransform{})
}

type trimSpacesTransform struct {
	middle       bool
	buf, scratch []byte
}

func (ts *trimSpacesTransform) Reset() { ts.middle, ts.buf = false, ts.buf[:0] }
func (ts *trimSpacesTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	ts.scratch = append(append(ts.scratch[:0], ts.buf...), src...)
	x := ts.scratch
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
	return transform.NewWriter(w, &trimTransform{
		prefix: []byte(prefix), suffix: []byte(suffix),
		middle: prefix == "",
	})
}

type trimTransform struct {
	prefix, suffix []byte
	buf, scratch   []byte
	middle         bool
}

func (tw *trimTransform) Reset() { tw.middle, tw.buf = false, tw.buf[:0] }
func (tw *trimTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	//log.Printf("dst=%d src=%q atEOF=%t", len(dst), src, atEOF)
	if !atEOF {
		if tw.middle {
			if len(tw.buf)+len(src) < len(tw.suffix) {
				//log.Println("a", string(tw.buf), string(src))
				return 0, 0, transform.ErrShortSrc
			}
		} else {
			if len(tw.buf)+len(src) < len(tw.prefix) {
				//log.Println("b", string(tw.buf), string(src))
				return 0, 0, transform.ErrShortSrc
			}
		}
	}
	tw.scratch = append(append(tw.scratch[:0], tw.buf...), src...)
	x := tw.scratch
	//log.Printf("x=%q middle=%t", x, tw.middle)
	tw.buf = tw.buf[:0]
	if !tw.middle {
		tw.middle = true
		x = bytes.TrimPrefix(x, tw.prefix)
		if len(x) == 0 {
			//log.Println("c", string(src))
			return 0, len(src), nil
		}
	}
	if atEOF {
		x = bytes.TrimSuffix(x, tw.suffix)
		//log.Println("end", string(x))
		if len(dst) < len(x) {
			return 0, 0, transform.ErrShortDst
		}
		return copy(dst, x), len(src), nil
	}
	i := len(x) - len(tw.suffix)
	//log.Printf("e x[:%d]=%q buf=%q", i, x[:i], tw.buf)
	//log.Printf("%q", x)
	if i <= 0 {
		tw.buf = append(tw.buf, x...)
		//log.Println("F")
		return 0, len(src), nil
	}
	//log.Printf("%q", x)
	if len(dst) < i {
		//log.Println("shodt")
		return 0, 0, transform.ErrShortDst
	}
	//log.Printf("%q", x)
	tw.buf = append(tw.buf, x[i:]...)
	//log.Printf("f x[:%d]=%q buf=%q", i, x[:i], tw.buf)
	return copy(dst, x[:i]), len(src), nil
}
