// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// https://git-scm.com/docs/pack-protocol
package gitpktline

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
)

// Reader reads git-pkt-line formatted slices.
type Reader struct {
	br     *bufio.Reader
	length [4]byte
	buf    []byte
}

// NewReader returns a new git-pkt-line reader.
func NewReader(r io.Reader) Reader { return Reader{br: bufio.NewReaderSize(r, 65520)} }

// https://git-scm.com/docs/protocol-common
func (r *Reader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		if err := r.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *Reader) fill() error {
	n, err := io.ReadFull(r.br, r.length[:])
	if err != nil {
		return err
	}
	if _, err = hex.Decode(r.length[:n/2], r.length[:n]); err != nil {
		return err
	}
	length := int(r.length[0]<<8) + int(r.length[1])
	if length < 4 || length > 65520 {
		return fmt.Errorf("invalid length: %d", length)
	}

	// A pkt-line is a variable length binary string. The first four bytes of the line, the pkt-len, indicates the total length of the line, in hexadecimal. The pkt-len includes the 4 bytes used to contain the length’s hexadecimal representation.
	n, err = io.ReadFull(r.br, r.buf[:length-4])
	r.buf = r.buf[:n]
	return err
}

// Writer writes in git-pkt-line format (prefix each line with hex-encoded line length, including the first 4 bytes.
type Writer struct{ io.Writer }

func (w Writer) Write(p []byte) (int, error) {
	var off int
	for off < len(p) {
		slice := p[off:]
		if len(slice) > 65516 {
			slice = slice[:65516]
		}
		length := len(slice) + 4
		var a [4]byte
		hex.Encode(a[:], []byte{byte(length >> 8), byte(length & 0xff)})
		if _, err := w.Writer.Write(a[:]); err != nil {
			return off, err
		}
		n, err := w.Writer.Write(slice)
		off += n
		if err != nil {
			return off, err
		}
	}
	return off, nil
}
