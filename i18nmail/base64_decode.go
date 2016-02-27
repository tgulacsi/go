// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"encoding/base64"
	"io"
)

const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// NewB64Decoder returns a new filtering bae64 decoder.
func NewB64Decoder(enc *base64.Encoding, r io.Reader) io.Reader {
	return base64.NewDecoder(enc, NewB64FilterReader(NewB64FilterReader(r)))
}

// NewB64FilterReader returns a base64 filtering reader.
func NewB64FilterReader(r io.Reader) io.Reader {
	return &paddingReader{
		Reader: NewFilterReader(r, []byte(b64chars)),
		Pad:    '=',
		Modulo: 4,
	}
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
func NewFilterReader(r io.Reader, okBytes []byte) io.Reader {
	fr := &filterReader{Reader: r}
	for _, b := range okBytes {
		fr.okMap[b] = true
	}
	return fr
}

type filterReader struct {
	io.Reader
	scratch []byte
	okMap   [256]bool
}

func (r *filterReader) Read(p []byte) (int, error) {
	if cap(r.scratch) < len(p) {
		r.scratch = make([]byte, len(p))
	}
	scratch := r.scratch[:len(p)]
	for {
		n, err := r.Reader.Read(scratch)
		if n == 0 {
			return 0, err
		}
		p = p[:0]
		for _, b := range scratch[:n] {
			if r.okMap[b] {
				p = append(p, b)
			}
		}
		if len(p) > 0 || err != nil {
			return len(p), err
		}
	}
}

type paddingReader struct {
	io.Reader
	Pad    byte
	Modulo int
	length int64
	atEOF  bool
}

func (r *paddingReader) Read(p []byte) (int, error) {
	if r.atEOF {
		padding := int(r.length % int64(r.Modulo))
		if padding == 0 {
			return 0, io.EOF
		}
		padding = r.Modulo - padding
		n := 0
		for i := 0; i < padding && i < len(p); i++ {
			p[i] = r.Pad
			n++
		}
		r.length += int64(n)
		return n, nil
	}

	n, err := r.Reader.Read(p)
	r.length += int64(n)
	if err != io.EOF {
		return n, err
	}
	r.atEOF = true
	if n > 0 {
		return n, nil
	}
	return r.Read(p)
}
