// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package secret

import (
	"bytes"
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
	return bytes.Repeat([]byte("*"), len(passw)), nil
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
	return enc.WriteToken(jsontext.String(strings.Repeat("*", len(passw))))
}

// MarshalJSON returns ***.
func (passw Password) MarshalJSON() ([]byte, error) {
	p := append(make([]byte, 0, 1+len(passw)+1), '"')
	if MarshalPassword.Load() {
		p = append(p, []byte(passw)...)
	} else {
		for range len(passw) {
			p = append(p, '*')
		}
	}
	return append(p, '"'), nil
}

// UnmarshalJSON reads the JSON.
// Watch out that this fails for itself (*** -marshaled JSON)!
func (passw *Password) UnmarshalJSON(p []byte) error {
	var s string
	err := json.Unmarshal(p, &s)
	if strings.IndexFunc(s, func(r rune) bool { return r != '*' }) < 0 {
		return nil
	}
	*passw = Password(s)
	return err
}

// String returns the real password.
func (passw Password) String() string { return string(passw) }

// Set the password (from a flag).
func (passw *Password) Set(s string) error {
	*passw = Password(s)
	return nil
}
