// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package secret

import (
	"bytes"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Password is a string that renders to *** in XML/JSON (MarshalText/<arshalJSON)
type Password string

// MarshalText returns ***.
func (passw Password) MarshalText() ([]byte, error) {
	return bytes.Repeat([]byte("*"), len(passw)), nil
}

// MarshalJSONTo encodes ***.
func (passw Password) MarshalJSONTo(enc *jsontext.Encoder) error {
	return enc.WriteToken(jsontext.String(strings.Repeat("*", len(passw))))
}

// MarshalJSON returns ***.
func (passw Password) MarshalJSON() ([]byte, error) {
	p := append(make([]byte, 0, 1+len(passw)+1), '"')
	for range len(passw) {
		p = append(p, '*')
	}
	return append(p, '"'), nil
}

// UnmarshalJSON reads the JSON.
func (passw *Password) UnmarshalJSON(p []byte) error {
	var s string
	err := json.Unmarshal(p, &s)
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
