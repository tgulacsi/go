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

import (
	"io"
	"io/ioutil"
	"net/http"
)

var _ = io.ReadCloser((*CountingReadCloser)(nil))

type CountingReadCloser struct {
	io.ReadCloser
	Size int64
}

func NewCountingReader(r io.Reader) *CountingReadCloser {
	if rc, ok := r.(io.ReadCloser); ok {
		return &CountingReadCloser{ReadCloser: rc}
	}
	return &CountingReadCloser{ReadCloser: struct {
		io.Reader
		io.Closer
	}{r, ioutil.NopCloser(nil)}}
}

func (cr *CountingReadCloser) Read(p []byte) (n int, err error) {
	n, err = cr.ReadCloser.Read(p)
	cr.Size += int64(n)
	return
}

var _ = http.ResponseWriter((*CountingResponseWriter)(nil))

type CountingResponseWriter struct {
	http.ResponseWriter
	Code int
	Size int64
}

func NewCountingRW(w http.ResponseWriter) *CountingResponseWriter {
	return &CountingResponseWriter{ResponseWriter: w}
}

func (cw *CountingResponseWriter) Write(p []byte) (n int, err error) {
	n, err = cw.ResponseWriter.Write(p)
	cw.Size += int64(n)
	return
}

func (rw *CountingResponseWriter) WriteHeader(code int) {
	rw.Code = code
	rw.ResponseWriter.WriteHeader(code)
}
