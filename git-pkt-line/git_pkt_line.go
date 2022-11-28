// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// https://git-scm.com/docs/pack-protocol
package gitpktline

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Reader reads git-pkt-line formatted slices.
type Reader struct {
	br     *bufio.Reader
	length [4]byte
	buf    []byte
	err    error
}

// NewReader returns a new git-pkt-line reader.
func NewReader(r io.Reader) *Reader { return &Reader{br: bufio.NewReaderSize(r, 65520)} }

// https://git-scm.com/docs/protocol-common
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, io.ErrShortBuffer
	}
	if r.err != nil {
		return 0, r.err
	}
	if len(r.buf) == 0 {
		if err := r.fill(); err != nil {
			r.err = err
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, r.err
}

func (r *Reader) fill() error {
	r.buf = r.buf[:0]
	n, err := io.ReadFull(r.br, r.length[:4])
	if n == 0 && (err == nil || errors.Is(err, io.EOF)) {
		return io.EOF
	}
	if n < 4 {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	if _, err = hex.Decode(r.length[:2], r.length[:4]); err != nil {
		return err
	}
	length := int(r.length[0]<<8) + int(r.length[1])
	length -= 4
	if length < 0 || length > 65516 {
		return fmt.Errorf("invalid length: %d", length)
	}
	if length == 0 {
		return nil
	}

	if cap(r.buf) < length {
		r.buf = make([]byte, length)
	}

	// A pkt-line is a variable length binary string. The first four bytes of the line, the pkt-len, indicates the total length of the line, in hexadecimal. The pkt-len includes the 4 bytes used to contain the length’s hexadecimal representation.
	n, err = io.ReadFull(r.br, r.buf[:length])
	r.buf = r.buf[:n]
	if n == length {
		return err
	}
	return fmt.Errorf("read %d, wanted %d: %w", n, length, io.ErrUnexpectedEOF)
}

// Writer writes in git-pkt-line format (prefix each line with hex-encoded line length, including the first 4 bytes.
type Writer struct{ io.Writer }

func NewWriter(w io.Writer) Writer { return Writer{Writer: w} }

func (w Writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		_, err := w.Writer.Write([]byte("0004"))
		return 0, err
	}
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
