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
	return base64.NewDecoder(enc.WithPadding(0), NewB64FilterReader(NewB64FilterReader(r)))
}

// NewB64FilterReader returns a base64 filtering reader.
func NewB64FilterReader(r io.Reader) io.Reader {
	return NewFilterReader(r, []byte(b64chars))
}

type filterReader struct {
	io.Reader
	okBytes [256]bool
	scratch []byte
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
func NewFilterReader(r io.Reader, okBytes []byte) *filterReader {
	fr := filterReader{Reader: r}
	for _, b := range okBytes {
		fr.okBytes[b] = true
	}
	return &fr
}
func (fr *filterReader) Read(p []byte) (int, error) {
	if cap(fr.scratch) < len(p) {
		n := 1024
		for n < len(p) {
			n <<= 1
		}
		fr.scratch = make([]byte, n)
	}
	n, err := fr.Reader.Read(fr.scratch[:len(p)])
	if n == 0 {
		return n, err
	}
	i := 0
	for _, b := range fr.scratch[:n] {
		if fr.okBytes[b] {
			p[i] = b
			i++
		}
	}
	return i, err
}
