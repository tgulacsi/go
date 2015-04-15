// build: !windows,linux,bsd,darwin
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

package iohlp

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"syscall"
)

// Mmap the file for read, return the bytes, an io.Closer and the error.
// Will read the data directly if Mmap fails.
func Mmap(f *os.File) ([]byte, io.Closer, error) {
	closer := ioutil.NopCloser(nil)
	fi, err := f.Stat()
	if err != nil {
		return nil, closer, err
	}
	if fi.Size() > MaxInt {
		return nil, closer, errors.New("file too big to Mmap")
	}
	p, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	if err != nil {
		Log.Error("Mmap", "f", f, "size", fi.Size(), "error", err)
		p, err = ioutil.ReadAll(f)
		return p, closer, err
	}
	Log.Debug("Mmap", "f", f, "len(p)", len(p))

	return p, &mmapCloser{p}, nil
}

type mmapCloser struct {
	p []byte
}

func (mc *mmapCloser) Close() error {
	if mc.p == nil {
		return nil
	}
	err := syscall.Munmap(mc.p)
	mc.p = nil
	return err
}
