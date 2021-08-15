//go:build posix || linux || !windows
// +build posix linux !windows

// Copyright 2019, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"syscall"
)

func (r *ReaderAt) mmap(fd uintptr, size int) error {
	var err error
	r.data, err = syscall.Mmap(int(fd), 0, size,
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	return err
}

func (r *ReaderAt) munmap() error {
	if r == nil || r.data == nil {
		return nil
	}
	data := r.data
	r.data = nil
	return syscall.Munmap(data)
}
