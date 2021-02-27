// +build posix linux !windows

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
	"errors"
	"io/ioutil"
	"os"
	"runtime"
	"syscall"
)

// MaxInt is the maximum value an int can contain.
const MaxInt = int64(int(^uint(0) >> 1))

// Mmap the file for read, return the bytes and the error.
// Will read the data directly if Mmap fails.
func Mmap(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxInt {
		return nil, errors.New("file too big to Mmap")
	}
	if fi.Size() == 0 {
		return []byte{}, nil
	}
	p, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	if err != nil {
		p, _ = ioutil.ReadAll(f)
		return p, err
	}

	runtime.SetFinalizer(&p, func(p *[]byte) {
		if p != nil {
			syscall.Munmap(*p)
		}
	})

	return p, nil
}
