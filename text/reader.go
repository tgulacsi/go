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

// NewDecodingReader returns a reader which decode from the given encoding, to utf8.
func NewDecodingReader(r io.Reader, enc encoding.Encoding) io.Reader {
	return transform.NewReader(r,
		transform.Chain(enc.NewDecoder(),
			encoding.Replacement.NewEncoder()))
}

// NewReplacementReader returns an utf8-enforcing reader
// which will replace all non-utf8 sequences with the replacement character
// (see http://godoc.org/code.google.com/p/go.text/encoding#pkg-variables).
func NewReplacementReader(r io.Reader) io.Reader {
	return transform.NewReader(r, encoding.Replacement.NewEncoder())
}
