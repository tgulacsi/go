// +build windows

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
	"bytes"
	"errors"
	"unsafe"
	"runtime"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// MaxInt is the maximum value an int can contain.
const MaxInt = int64(1<<49)


// Mmap returns a mmap of the given file - just a copy of it.
func Mmap(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size > MaxInt {
		return nil, errors.New("file too big to Mmap")
	}
	if size== 0 {
		return []byte{}, nil
	}

	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	if handle, err := windows.CreateFileMapping(
		windows.Handle(f.Fd()), &sa, windows.PAGE_READONLY,  
		0, 0, nil,
	); err == nil {
		if addr, err := windows.MapViewOfFile(
			windows.Handle(handle), windows.FILE_MAP_READ, 
			0, 0, 0,
		); err != nil {
			windows.CloseHandle(handle)
		} else {
			b := (*(*[MaxInt]byte)(unsafe.Pointer(addr)))[:size:size]
			runtime.SetFinalizer(&b, func(p *[]byte) {
				if p != nil {
					windows.UnmapViewOfFile(addr)
					*p = nil
				}
				windows.CloseHandle(handle)
			})
			return b, nil
		}
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, f)
	return buf.Bytes(), err
}
