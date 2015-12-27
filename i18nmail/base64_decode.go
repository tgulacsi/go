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
	n, err := fr.Reader.Read(p)
	if n == 0 {
		return n, err
	}
	p2 := make([]byte, 0, n)
	for _, b := range p {
		if fr.okBytes[b] {
			p2 = append(p2, b)
		}
	}
	copy(p, p2)
	return len(p2), err
}
