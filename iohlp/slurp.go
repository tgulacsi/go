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
)

// MakeSectionReader reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func MakeSectionReader(r io.Reader, threshold int) (*io.SectionReader, error) {
	if rat, ok := r.(*io.SectionReader); ok {
		return rat, nil
	}
	lr := io.LimitedReader{R: r, N: int64(threshold) + 1}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, &lr)
	bsr := io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()))
	if err != nil || buf.Len() <= threshold {
		return bsr, err
	}
	fh, err := ioutil.TempFile("", "iohlp-readall-")
	if err != nil {
		return bsr, err
	}
	defer os.Remove(fh.Name())
	defer fh.Close()
	if _, err = fh.Write(buf.Bytes()); err != nil {
		return bsr, err
	}
	buf.Truncate(0)
	if _, err = io.Copy(fh, r); err != nil {
		return nil, err
	}
	rat, err := Mmap(fh)
	if closeErr := fh.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return io.NewSectionReader(rat, 0, int64(rat.Len())), err
}
