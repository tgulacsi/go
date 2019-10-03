/*
Copyright 2017 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package jsondiff is for diffing JSON data.
//
// It does it by pretty-printing the JSON data
// (with ordered keys and generous line feeds), and then diffing it
// line-by-line.
package jsondiff

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"

	"github.com/kylelemons/godebug/diff"
	errors "golang.org/x/xerrors"
)

func Pretty(w io.Writer, data map[string]interface{}, indent string) error {
	n := len(data)
	if n == 0 {
		_, err := io.WriteString(w, "{}")
		return err
	}
	keys := make([]string, n)
	i := 0
	for k := range data {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	n--
	for i, k := range keys {
		b, err := json.Marshal(k)
		if err != nil {
			return err
		}
		if i != 0 {
			if _, err = io.WriteString(w, indent+"  "); err != nil {
				return err
			}
		}
		if _, err = w.Write(b); err != nil {
			return err
		}
		if _, err = io.WriteString(w, ": "); err != nil {
			return err
		}
		v := data[k]
		if m, ok := v.(map[string]interface{}); ok {
			if err = Pretty(w, m, indent+"  "); err != nil {
				return err
			}
		} else {
			if b, err = json.Marshal(v); err != nil {
				return err
			}
			if _, err = w.Write(b); err != nil {
				return err
			}
		}
		if i != n {
			_, err = io.WriteString(w, ",\n")
		} else {
			_, err = w.Write([]byte{'\n'})
		}
		if err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, indent+"}")
	return err
}

// Diff returns the line diff of the two JSON map[string]interface{}s.
func Diff(a map[string]interface{}, b map[string]interface{}) string {
	var bA, bB bytes.Buffer
	if err := Pretty(&bA, a, ""); err != nil {
		return "ERROR(a): " + err.Error()
	}
	if err := Pretty(&bB, b, ""); err != nil {
		return "ERROR(b): " + err.Error()
	}
	return diff.Diff(bA.String(), bB.String())
}

// DiffStrings json.Unmarshals the strings and diffs that.
func DiffStrings(a, b string) (string, error) {
	mA := make(map[string]interface{})
	if err := json.Unmarshal([]byte(a), &mA); err != nil {
		return "", errors.Errorf("%s: %w", "unmarshal 1. arg", err)
	}
	var bA bytes.Buffer
	if err := Pretty(&bA, mA, ""); err != nil {
		return "", errors.Errorf("%s: %w", "pretty-print 1. arg", err)
	}

	mB := make(map[string]interface{})
	if err := json.Unmarshal([]byte(b), &mB); err != nil {
		return "", errors.Errorf("%s: %w", "unmarshal 2. arg", err)
	}
	var bB bytes.Buffer
	if err := Pretty(&bB, mB, ""); err != nil {
		return "", errors.Errorf("%s: %w", "pretty-print 2. arg", err)
	}

	return diff.Diff(bA.String(), bB.String()), nil
}
