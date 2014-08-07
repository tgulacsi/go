/*
Copyright 2014 Tamás Gulácsi

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

package text

import (
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStringReader(t *testing.T) {
	input := "a\u01234567b"
	N := len(input)
	p1 := make([]byte, N)
	p2 := make([]byte, N)
	for i := 1; i < len(input)-1; i++ {
		r := NewStringReader(io.MultiReader(strings.NewReader(input[:i]), strings.NewReader(input[i:])))
		n1, err := r.Read(p1[:N])
		if err != nil {
			t.Errorf("first part: %v", err)
			break
		}
		if !utf8.Valid(p1[:n1]) {
			t.Errorf("invalid runes in %+q", p1[:n1])
			break
		}

		n2, err := r.Read(p2[:N])
		if err != nil {
			t.Errorf("second part: %v", err)
		}
		if !utf8.Valid(p1[:n1]) {
			t.Errorf("invalid runes in %+q", p1[:n1])
			break
		}

		if n1+n2 != N {
			t.Errorf("length mismatch: got %d, awaited %d.", n1+n2, N)
			break
		}
		got := string(append(append([]byte{}, p1[:n1]...), p2[:n2]...))
		if got != input {
			t.Errorf("awaited %+q, got %+q.", input, got)
			break
		}
	}

}
