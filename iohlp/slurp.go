/*
Copyright 2019 Tamás Gulácsi

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
	"io"
	"io/ioutil"
	"os"
)

// ReadAll reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func ReadAll(r io.Reader, threshold int) ([]byte, io.Closer, error) {
	lr := io.LimitedReader{R: r, N: int64(threshold) + 1}
	b, err := ioutil.ReadAll(&lr)
	if err != nil {
		return b, nilClose, err
	}
	if lr.N > 0 {
		return b, nilClose, nil
	}
	fh, err := ioutil.TempFile("", "iohlp-readall-")
	if err != nil {
		return b, nilClose, err
	}
	os.Remove(fh.Name())
	if _, err = fh.Write(b); err != nil {
		return b, nilClose, err
	}
	if _, err = io.Copy(fh, r); err != nil {
		fh.Close()
		return nil, nilClose, err
	}
	b, closer, err := Mmap(fh)
	fh.Close()
	if err != nil {
		if closer != nil {
			closer.Close()
		}
		return b, nil, err
	}
	return b, closer, nil
}

type nilCloser struct{}

var nilClose nilCloser

func (_ nilCloser) Close() error { return nil }
