// Copyright 2014, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"golang.org/x/exp/mmap"
)

// MmapFile returns the mmap of the given path.
func MmapFile(fn string) (io.ReaderAt, error) {
	return mmap.Open(fn)
}

// Mmap the file for read, return the bytes and the error.
// Will read the data directly if Mmap fails.
func Mmap(f *os.File) (*ReaderAt, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if int64(int(size)) != size {
		return nil, errors.New("file too big to Mmap")
	}
	if size < 0 {
		return nil, errors.New("file has negative size")
	}
	var r ReaderAt
	if size == 0 {
		return &r, nil
	}
	if err = r.mmap(f.Fd(), int(size)); err != nil {
		return nil, err
	}

	runtime.SetFinalizer(&r, func(r *ReaderAt) { r.munmap() })

	return &r, nil
}

// ReaderAt reads a memory-mapped file.
//
// Like any io.ReaderAt, clients can execute parallel ReadAt calls, but it is
// not safe to call Close and reading methods concurrently.
//
// Copied from https://github.com/golang/exp/blob/85be41e4509f/mmap/mmap_unix.go#L115
type ReaderAt struct {
	data []byte
}

// Close closes the reader.
func (r *ReaderAt) Close() error {
    _ = r.fh  // windows needs fh
	if r.data == nil {
		return nil
	}
	err := r.munmap()
	runtime.SetFinalizer(r, nil)
	return err
}

// Len returns the length of the underlying memory-mapped file.
func (r *ReaderAt) Len() int {
	return len(r.data)
}

// At returns the byte at index i.
func (r *ReaderAt) At(i int) byte {
	return r.data[i]
}

// ReadAt implements the io.ReaderAt interface.
func (r *ReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if r.data == nil {
		return 0, errors.New("mmap: closed")
	}
	if off < 0 || int64(len(r.data)) < off {
		return 0, fmt.Errorf("mmap: invalid ReadAt offset %d", off)
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
