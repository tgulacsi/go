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
	buf := srBufPool.Get()
	defer srBufPool.Put(buf)
	_, err := io.CopyN(buf, r, int64(threshold)+1)
	bsr := io.NewSectionReader(bytes.NewReader(buf.Bytes()), 1, int64(buf.Len()))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return bsr, fmt.Errorf("read below threshold: %w", err)
	}
	fh, err := os.CreateTemp("", "iohlp-readall-")
	if err != nil {
		return bsr, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(fh.Name())
	defer fh.Close()
	if _, err = fh.Write(buf.Bytes()); err != nil {
		return bsr, fmt.Errorf("write temp file: %w", err)
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
		return nil, mmapErr
	}
	return io.NewSectionReader(rat, 0, int64(rat.Len())), err
}
