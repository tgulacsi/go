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
	"os"
	"runtime"
	"syscall"
)

// MmapFile returns the mmap of the given path.
func MmapFile(fn string) ([]byte, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	p, err := Mmap(f)
	f.Close()
	return p, err
}

// Mmap returns a mmap of the given File
func Mmap(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxInt {
		return nil, errors.New("file too big to Mmap")
	}
	p, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(&p[0], func(_ interface{}) error { return syscall.Munmap(p) })
	return p, nil

}
