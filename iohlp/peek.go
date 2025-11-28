// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"bytes"
	"io"
)

// Peek (ReadAtLeast) the io.Reader, and return it unmodified.
func Peek(r io.Reader, atLeast int) ([]byte, io.Reader, error) {
	length := 1024
	for length < atLeast {
		length *= 2
	}
	b := make([]byte, length)
	n, err := io.ReadAtLeast(r, b, atLeast)
	b = b[:n]
	return b, io.Reader(bytes.NewReader(b)), err
}
