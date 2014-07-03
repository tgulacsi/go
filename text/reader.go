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
	"bytes"
	"io"
	"io/ioutil"

	"code.google.com/p/go.text/encoding"
	"code.google.com/p/go.text/transform"
)

// NewReader returns a reader which decode from the given encoding, to utf8.
//
// If enc is nil, then only an utf8-enforcing replacement reader
// (see http://godoc.org/code.google.com/p/go.text/encoding#pkg-variables)
// is used.
func NewReader(r io.Reader, enc encoding.Encoding) io.Reader {
	if enc == nil || enc == encoding.Replacement {
		return transform.NewReader(r, encoding.Replacement.NewEncoder())
	}
	return transform.NewReader(r,
		transform.Chain(enc.NewDecoder(), encoding.Replacement.NewEncoder()))
}

// Decode decodes the bytes from enc to utf8 (an allocating, convenience version of transform.Transform).
func Decode(p []byte, enc encoding.Encoding) (string, error) {
	r := NewReader(bytes.NewReader(p), enc)
	q, err := ioutil.ReadAll(r)
	return string(q), err
}

// NewDecodingReader is a deprecated, it has been renamed to NewReader.
func NewDecodingReader(r io.Reader, enc encoding.Encoding) io.Reader {
	return NewReader(r, enc)
}
