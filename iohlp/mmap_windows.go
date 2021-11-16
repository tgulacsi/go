//go:build windows
// +build windows

// Copyright 2015, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// MaxInt is the maximum value an int can contain.
const MaxInt = int64(1 << 49)

// mmap returns a mmap of the given file - just a copy of it.
func (r *ReaderAt) mmap(fd uintptr, size int) error {
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	handle, err := windows.CreateFileMapping(
		windows.Handle(fd), &sa, windows.PAGE_READONLY,
		0, 0, nil)
	if err != nil {
		return err
	}
	r.fh = uintptr(handle)
	addr, err := windows.MapViewOfFile(
		windows.Handle(handle), windows.FILE_MAP_READ,
		0, 0, 0)
	if err != nil {
		windows.CloseHandle(handle)
		return err
	}
	r.data = (*(*[MaxInt]byte)(unsafe.Pointer(addr)))[:size:size]
	return nil
}

func (r *ReaderAt) munmap() error {
	if r == nil || r.data == nil {
		return nil
	}
	data, fh := r.data, r.fh
	r.data, r.fh = nil, 0
	err := windows.UnmapViewOfFile(uintptr(unsafe.Pointer(&data[0])))
	if fh == 0 {
		return err
	}
	return windows.CloseHandle(windows.Handle(fh))
}
