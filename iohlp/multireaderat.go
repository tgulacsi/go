// Copyright 2025 Tamás Gulácsi. All rights reserved
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"errors"
	"io"
	"slices"
)

// SizeReaderAt is a ReaderAt with Size method (like *io.SectionReader).
type SizeReaderAt interface {
	io.ReaderAt
	Size() int64
}
type multiReaderAt struct {
	ras  []io.ReaderAt
	ends []int64
}

var _ io.ReaderAt = multiReaderAt{}

// NewMultiReaderAt returns an io.ReaderAt that combines it's lower ReaderAts, and has a Size() method.
func NewMultiReaderAt(ras ...SizeReaderAt) SizeReaderAt {
	mra := multiReaderAt{
		ras:  make([]io.ReaderAt, 0, len(ras)),
		ends: make([]int64, 0, len(ras)),
	}
	var offset int64
	for _, ra := range ras {
		size := ra.Size()
		if size == 0 {
			continue
		}
		offset += size
		mra.ends = append(mra.ends, offset)
		mra.ras = append(mra.ras, ra)
	}
	return mra
}

func (mra multiReaderAt) ReadAt(p []byte, off int64) (int, error) {
	// 12 24
	i, _ := slices.BinarySearch(mra.ends, off+1)
	// slog.Debug("ReadAt", "off", off, "i", i, "p", len(p))
	if len(mra.ends) <= i {
		return 0, io.EOF
	}
	if i > 0 {
		off -= mra.ends[i-1]
	}
	n, err := mra.ras[i].ReadAt(p, off)
	if err != nil && i != len(mra.ends)-1 && errors.Is(err, io.EOF) {
		err = nil
	}
	return n, err
}
func (mra multiReaderAt) Size() int64 { return mra.ends[len(mra.ends)-1] }
