// Copyright 2019, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"bytes"
	"io"
)

const defaultFindReaderBufSize = 64 << 10

// FindReader finds the first occurrence of needle in the io.Reader and gives back its position.
// Returns -1 when needle is not found.
//
// Uses the default buffer size (64kB).
func FindReader(r io.Reader, needle []byte) (int, error) {
	return FindReaderSize(r, needle, defaultFindReaderBufSize)
}

// FindReaderSize finds the first occurrence of needle in the io.Reader and gives back its position.
// Returns -1 when needle is not found.
//
// Uses the specified amount of buffer (must be longer than needle!).
func FindReaderSize(r io.Reader, needle []byte, bufSize int) (int, error) {
	if bufSize < len(needle) {
		bufSize = 2 * len(needle)
	}
	needleLen := len(needle)
	buf := make([]byte, bufSize)
	var off, start int
	for {
		n, err := io.ReadAtLeast(r, buf[start:], needleLen)
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		if n == 0 && err == io.EOF {
			return -1, nil
		}
		if i := bytes.Index(buf[:start+n], needle); i >= 0 {
			return off + i, nil
		}
		if err != nil {
			return -1, err
		}
		// copy the end to the start
		copy(buf[0:], buf[start+n-needleLen-1:start+n])
		if start == 0 {
			off += n - needleLen - 1
		} else {
			off += n
		}
		start = needleLen - 1
	}
	return -1, nil
}
