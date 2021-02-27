/*
Copyright 2019, 2021 Tamás Gulácsi

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
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"unsafe"
)

// ReadAll reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func ReadAll(r io.Reader, threshold int) ([]byte, error) {
	lr := io.LimitedReader{R: r, N: int64(threshold) + 1}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, &lr)
	if err != nil || buf.Len() <= threshold {
		return buf.Bytes(), err
	}
	fh, err := ioutil.TempFile("", "iohlp-readall-")
	if err != nil {
		return buf.Bytes(), err
	}
	os.Remove(fh.Name())
	if _, err = fh.Write(buf.Bytes()); err != nil {
		fh.Close()
		return buf.Bytes(), err
	}
	buf.Truncate(0)
	if _, err = io.Copy(fh, r); err != nil {
		fh.Close()
		return nil, err
	}
	b, err := Mmap(fh)
	fh.Close()
	return b, err
}

// ReadAllString is like ReadAll, but returns a string.
func ReadAllString(r io.Reader, threshold int) (string, error) {
	b, err := ReadAll(r, threshold)
	return *((*string)(unsafe.Pointer(&b))), err
}
