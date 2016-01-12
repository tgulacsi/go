/*
Copyright 2016 Tamás Gulácsi

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

package iohlp

import (
	"bufio"
	"io"
	"strings"

	"github.com/bthomson/wrap"
)

// WrappingReader returns an io.Reader which will wrap lines longer than the given width.
// All other lines (LF chars) will be preserved.
func WrappingReader(r io.Reader, width int) io.Reader {
	pr, pw := io.Pipe()

	go func() {
		uwidth := uint(width)
		scanner := bufio.NewScanner(r)
		ew := &ErrWriter{Writer: pw}
		for scanner.Scan() { // split lines
			if len(scanner.Bytes()) <= width {
				ew.Write(scanner.Bytes())
				ew.Write([]byte{'\n'})
				continue
			}
			for _, line := range strings.Split(wrap.String(scanner.Text(), uwidth), "\n") {
				io.WriteString(ew, line)
				ew.Write([]byte{'\n'})
			}
		}
		pw.CloseWithError(scanner.Err())
	}()

	return pr
}

type ErrWriter struct {
	io.Writer
	err error
}

func (w *ErrWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	var n int
	n, w.err = w.Writer.Write(p)
	return n, w.err
}
