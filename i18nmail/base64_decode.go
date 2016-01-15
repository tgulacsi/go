// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"bufio"
	"bytes"
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
	return NewFilterReader(r, []byte(b64chars))
}

// NewFilterReader returns a reader which silently throws away bytes not in
// the okBytes slice.
func NewFilterReader(r io.Reader, okBytes []byte) io.Reader {
	var okMap [256]bool
	for _, b := range okBytes {
		okMap[b] = true
	}
	pr, pw := io.Pipe()
	go func() {
		var length int64
		raw := make([]byte, 16<<10)
		filtered := make([]byte, cap(raw))
		bw := bufio.NewWriter(pw)
		finish := func(err error) {
			bw.Flush()
			pw.CloseWithError(err)
		}
		for {
			n, readErr := r.Read(raw)
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
			i, err := bw.Write(filtered[:i])
			if err != nil {
				finish(err)
				return
			}
			length += int64(i)
			if readErr == nil {
				continue
			}
			if readErr != io.EOF {
				finish(err)
				return
			}
			if padding := int(length % 4); padding > 0 {
				if _, err := bw.Write(bytes.Repeat([]byte{'='}, 4-padding)); err != nil {
					finish(err)
					return
				}
			}
			finish(readErr)
			return
		}
	}()
	return pr
}
