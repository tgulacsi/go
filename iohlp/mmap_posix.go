//go:build posix || linux || !windows
// +build posix linux !windows

// Copyright 2019, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"runtime"
	"syscall"
)

func (r *ReaderAt) mmap(fd uintptr, size int) error {
	var err error
	if r.data, err = syscall.Mmap(int(fd), 0, size,
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE,
	); err != nil {
		return err
	}
	r.cleanup = runtime.AddCleanup(
		r,
		func(data []byte) { syscall.Munmap(data) },
		r.data,
	)
	return nil
}

func (r *ReaderAt) munmap() error {
	if r == nil || r.data == nil {
		return nil
	}
	data := r.data
	r.data = nil
	return syscall.Munmap(data)
}
