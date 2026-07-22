// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package secret

import (
	"bytes"
	"io"
	"strings"
	"sync/atomic"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// MarshalPassword allows overriding the default *** behaviour.
var MarshalPassword atomic.Bool

// Password is a string that renders to *** in XML/JSON (MarshalText/<arshalJSON)
type Password string

// MarshalText returns ***.
func (passw Password) MarshalText() ([]byte, error) {
	if MarshalPassword.Load() {
		return []byte(passw), nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(passw)))
	passw.marshalAppend(buf)
	return buf.Bytes(), nil
}

func (passw Password) marshalAppend(buf interface {
	io.StringWriter
	io.ByteWriter
}) {
	n := len(passw)
	m := prefixSuffixLength(n)
	if m != 0 {
		buf.WriteString(string(passw[:m]))
	}
	for i := m; i < n-m-m+1; i++ {
		buf.WriteByte('*')
	}
	if m != 0 {
		buf.WriteString(string(passw[len(passw)-m:]))
	}
}

// UnmarshalJSON reads the text.
// Watch out that this fails for itself (*** -marshaled text)!
func (passw *Password) UnmarshalText(p []byte) error {
	if bytes.IndexFunc(p, func(r rune) bool { return r != '*' }) < 0 {
		return nil
	}
	*passw = Password(p)
	return nil
}

// MarshalJSONTo encodes ***.
func (passw Password) MarshalJSONTo(enc *jsontext.Encoder) error {
	if MarshalPassword.Load() {
		return enc.WriteToken(jsontext.String(string(passw)))
	}
	buf := bytes.NewBuffer(make([]byte, 0, 1+len(passw)+1))
	passw.marshalAppend(buf)
	return enc.WriteToken(jsontext.String(buf.String()))
}

// MarshalJSON returns ***.
func (passw Password) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1+len(passw)+1))
	buf.WriteByte('"')
	if MarshalPassword.Load() {
		buf.WriteString(string(passw))
	} else {
		passw.marshalAppend(buf)
	}
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads the JSON.
// Watch out that this fails for itself (*** -marshaled JSON)!
func (passw *Password) UnmarshalJSON(p []byte) error {
	var s string
	err := json.Unmarshal(p, &s)
	m := prefixSuffixLength(len(s))
	if strings.IndexFunc(s[m:len(s)-m], func(r rune) bool { return r != '*' }) < 0 {
		return nil
	}
	*passw = Password(s)
	return err
}

// String returns the real password.
func (passw Password) String() string { return string(passw) }

// Text returns the garbled representation of Password.
func (passw Password) Text() string {
	if MarshalPassword.Load() {
		return string(passw)
	}
	var buf strings.Builder
	buf.Grow(len(passw))
	passw.marshalAppend(&buf)
	return buf.String()
}

// Set the password (from a flag).
func (passw *Password) Set(s string) error {
	*passw = Password(s)
	return nil
}

func prefixSuffixLength(n int) int {
	if n < 8 {
		return 0
	} else if n < 16 {
		return 1
	} else {
		return 2
	}
}
