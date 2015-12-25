// Copyright 2013 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package i18nmail

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// B64Filter is a decoding base64 filter
type B64Filter struct {
	n         int
	decodeMap [256]byte
	r         io.Reader
}

// B64FilterReader wraps the reader for decoding base64
func B64FilterReader(r io.Reader, decoder *base64.Encoding) io.Reader {
	f := B64Filter{r: r}
	for i := 0; i < len(f.decodeMap); i++ {
		f.decodeMap[i] = 0xFF
	}
	for i := 0; i < len(b64chars); i++ {
		f.decodeMap[b64chars[i]] = byte(i)
	}
	if decoder != nil {
		return base64.NewDecoder(decoder, &f)
	}
	return &f
}

// decodes Base64-encoded stream as reading
func (f *B64Filter) Read(b []byte) (int, error) {
	n, err := f.r.Read(b)
	if err != nil {
		if err == io.EOF && f.n%4 != 0 {
			miss := 4 - (f.n % 4)
			for i := 0; i < miss; i++ {
				b[n+i] = '='
			}
			f.n += miss
			return miss, nil
		}
		return n, err
	}
	for i := 0; i < n; i++ {
		if b[i] == '\r' || b[i] == '\n' || b[i] == '=' {
			continue
		}
		if c := f.decodeMap[b[i]]; c == 0xFF {
			logger.Warn().Log("msg", "invalid char: "+fmt.Sprintf("%c(%d) @ %d", b[i], b[i], f.n+i))
			b[i] = '\n'
		}
	}
	f.n += n
	return n, err
}

type b64ForceDecoder struct {
	*base64.Encoding
	r       io.Reader
	scratch []byte
}

func (d *b64ForceDecoder) Read(p []byte) (int, error) {
	es := d.Encoding.EncodedLen(len(p))
	if cap(d.scratch) < es {
		d.scratch = make([]byte, es)
	} else {
		d.scratch = d.scratch[:es]
	}
	raw := d.scratch
	n, err := d.r.Read(raw)
	//logger.Debug("msg", "read", "n", n, "error", err)
	if n == 0 {
		return n, err
	}
	raw = raw[:n]
	for len(raw) > 0 {
		dn, e := d.Encoding.Decode(p, raw)
		//logger.Debug("msg", "decode", "dn", dn, "error", e)
		if e == nil {
			return dn, err
		}
		bad := raw[:min(200, len(raw))]
		txt := e.Error()
		q := strings.LastIndex(txt, " ")
		if q < 0 {
			if err == nil {
				err = e
			}
			return dn, err
		}
		i, e2 := strconv.Atoi(txt[q+1:])
		if e2 != nil {
			if err == nil {
				err = e
			}
			return dn, err
		}
		bad = raw[max(0, i-20):min(i+4, len(raw))]
		logger.Error().Log("msg", "base64 decoding", "raw", string(bad), "error", e)
		if 0 <= i && i < len(raw) {
			raw = append(raw[:i], raw[i+1:]...)
		} else {
			raw = raw[:len(raw)-1]
		}

	}
	return 0, err
}
