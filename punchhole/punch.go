/*
Copyright 2014 Tamás Gulácsi.

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

package punchhole

import (
	"errors"
	"io"
	"os"
)

var errNoPunch = errors.New("punchHole not supported")

// punchHole, if non-nil, punches a hole in f from offset to offset+size.
var PunchHole func(file *os.File, offset, size int64) error

// punchHoleZeros zeroes the bytes of the file from offset at size length.
func punchHoleZeros(file *os.File, offset, size int64) error {
	_, err := file.Seek(offset, 0)
	if err != nil {
		return err
	}
	_, err = io.CopyN(file, &zeroReader{size}, size)
	return err
}

type zeroReader struct {
	n int64
}

func (r *zeroReader) Read(p []byte) (int, error) {
	if r.n == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) <= r.n {
		for i := range p {
			p[i] = 0
		}
		r.n -= int64(n)
		return n, nil
	}
	n = int(r.n) // possible as n > r.n and n is an int (len(p))
	for i := 0; i < n; i++ {
		p[i] = 0
	}
	r.n = 0
	return n, nil
}

const blockSize = 1 << 20

func (r *zeroReader) WriteTo(w io.Writer) (int64, error) {
	var written int64
	var buf []byte
	if r.n > blockSize {
		buf = make([]byte, blockSize)
		for r.n > blockSize {
			n, err := w.Write(buf)
			r.n -= int64(n)
			written += int64(n)
			if err != nil {
				return written, err
			}
		}
		buf = buf[:r.n]
	} else {
		buf = make([]byte, r.n)
	}
	r.n = 0
	n, err := w.Write(buf)
	written += int64(n)
	return written, err
}
