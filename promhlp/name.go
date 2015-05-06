/*
Copyright 2015 Tamás Gulácsi

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

package promhlp

import "strings"

// ClearName replaces non-allowed characters for Prometheus metrics.
//
// For metrics, colon is allowed, for tags, is is not.
func ClearName(txt string, allowColon bool, replaceRune rune) string {
	if replaceRune == 0 {
		replaceRune = -1
	}
	i := -1
	return strings.Map(
		func(r rune) rune {
			i++
			if r == '.' {
				return '_'
			}
			if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_' {
				return r
			}
			if allowColon && r == ':' {
				return r
			}
			if i > 0 && '0' <= r && r <= '9' {
				return r
			}
			return replaceRune
		},
		txt)
}
