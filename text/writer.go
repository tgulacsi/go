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

	"code.google.com/p/go.text/encoding"
	"code.google.com/p/go.text/transform"
)

// NewEncodingWriter returns a writer which encodes to the given encoding, utf8.
//
// If enc is nil, then only an utf8-enforcing replacement writer
// (see http://godoc.org/code.google.com/p/go.text/encoding#pkg-variables)
// is used.
func NewEncodingWriter(w io.Writer, enc encoding.Encoding) io.Writer {
	if enc == nil || enc == encoding.Replacement {
		return transform.NewWriter(w, encoding.Replacement.NewEncoder())
	}
	return transform.NewWriter(w,
		transform.Chain(enc.NewEncoder(), encoding.Replacement.NewEncoder()))
}
