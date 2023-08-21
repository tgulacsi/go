// Copyright 2019, 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/tgulacsi/go/bufpool"
)

var srBufPool = bufpool.New(1 << 20)

// MakeSectionReader reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func MakeSectionReader(r io.Reader, threshold int) (*io.SectionReader, error) {
	if rat, ok := r.(*io.SectionReader); ok {
		return rat, nil
	}
	rat, cleanup, err := slurp(r, threshold)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	if cleanup != nil {
		rdr := bytes.NewReader(rat.data)
		runtime.SetFinalizer(rdr, func(_ *bytes.Reader) { cleanup() })
		return io.NewSectionReader(rdr, 0, int64(rat.Len())), nil
	}
	return io.NewSectionReader(rat, 0, int64(rat.Len())), nil
}

// Slurp reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func Slurp(r io.Reader, threshold int) (data []byte, cleanup func(), err error) {
	rat, cleanup, err := slurp(r, threshold)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, err
	}
	if cleanup == nil {
		cleanup = func() { rat.Close() }
	}
	return rat.data, cleanup, nil
}

func slurp(r io.Reader, threshold int) (*ReaderAt, func(), error) {
	buf := srBufPool.Get()
	if _, err := io.CopyN(buf, r, int64(threshold)+1); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			srBufPool.Put(buf)
			return nil, nil, fmt.Errorf("read below threshold: %w", err)
		}
		return &ReaderAt{data: buf.Bytes()}, func() { srBufPool.Put(buf) }, nil
	}
	defer srBufPool.Put(buf)

	fh, err := os.CreateTemp("", "iohlp-readall-")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(fh.Name())
	defer fh.Close()
	if _, err = fh.Write(buf.Bytes()); err != nil {
		return nil, nil, fmt.Errorf("write temp file: %w", err)
	}
	buf.Truncate(0)
	_, err = io.Copy(fh, r)
	if err != nil {
		err = fmt.Errorf("copy to temp file: %w", err)
	}
	rat, mmapErr := Mmap(fh)
	if mmapErr != nil && err == nil {
		err = mmapErr
	}
	if closeErr := fh.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if mmapErr != nil {
		return nil, nil, mmapErr
	}
	return rat, nil, err
}
