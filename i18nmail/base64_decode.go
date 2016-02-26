// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
)

const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// NewB64Decoder returns a new filtering bae64 decoder.
func NewB64Decoder(enc *base64.Encoding, r io.Reader) io.Reader {
	return base64.NewDecoder(enc, NewB64FilterReader(NewB64FilterReader(r)))
}

// NewB64FilterReader returns a base64 filtering reader.
func NewB64FilterReader(r io.Reader) io.Reader {
	return NewFilterReader(r, []byte(b64chars))
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
var NewFilterReader = NewFilterReaderMem

// NewFilterReaderMem returns a reader which silently throws away bytes not in
// the okBytes slice.
//
// Reads the whole reader into memory.
func NewFilterReaderMem(r io.Reader, okBytes []byte) io.Reader {
	var okMap [256]bool
	for _, b := range okBytes {
		okMap[b] = true
	}
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return &rdr{Err: err}
	}
	filtered := make([]byte, 0, len(raw)+3)
	for _, b := range raw {
		if okMap[b] {
			filtered = append(filtered, b)
		}
	}
	if padding := int(len(filtered) % 4); padding > 0 {
		filtered = append(filtered, bytes.Repeat([]byte{'='}, 4-padding)...)
	}
	return bytes.NewReader(filtered)
}

type rdr struct {
	Err error
	io.Reader
}

func (r *rdr) Read(p []byte) (int, error) {
	if r.Err != nil {
		return 0, r.Err
	}
	return r.Reader.Read(p)
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
//
// Uses io.Pipe to avoid reading whole reader into memory.
func NewFilterReaderPipe(r io.Reader, okBytes []byte) io.Reader {
	var okMap [256]bool
	for _, b := range okBytes {
		okMap[b] = true
	}
	pr, pw := io.Pipe()
	go func() {
		var length int64
		raw := make([]byte, 16<<10)
		filtered := make([]byte, cap(raw))
		for {
			n, readErr := r.Read(raw[:cap(raw)])
			if n == 0 && readErr == nil {
				continue
			}
			filtered = filtered[:n]
			i := 0
			for _, b := range raw[:n] {
				if okMap[b] {
					filtered[i] = b
					i++
				}
			}
			i, err := pw.Write(filtered[:i])
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			length += int64(i)
			if readErr == nil {
				continue
			}
			if readErr != io.EOF {
				pw.CloseWithError(err)
				return
			}
			if padding := int(length % 4); padding > 0 {
				if _, err := pw.Write(bytes.Repeat([]byte{'='}, 4-padding)); err != nil {
					pw.CloseWithError(err)
					return
				}
			}
			pw.CloseWithError(readErr)
			return
		}
	}()
	return pr
}
