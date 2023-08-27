// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// https://git-scm.com/docs/pack-protocol
package gitpktline

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// https://git-scm.com/docs/gitattributes

type LongRunningProcess struct {
	r *Reader
	w Writer
}

func NewLongRunningProcess(r io.Reader, w io.Writer, caps []string) *LongRunningProcess {
	return &LongRunningProcess{r: NewReader(r), w: NewWriter(w)}
}

// Handshake initates the handshake with the parent git process, advertises the given capabilities.
func (p *LongRunningProcess) Handshake(caps []string) error {
	first, err := p.r.ReadPacket()
	if err != nil {
		return err
	}

	if !(bytes.HasPrefix(first, []byte("git-")) && bytes.HasSuffix(first, []byte("-client"))) {
		return fmt.Errorf("got %q, wanted git-*-client", first)
	}
	if _, err = p.r.ReadPackets(); err != nil {
		return err
	}
	if err = p.w.WritePackets(
		append(first[:len(first)-6], []byte("server")...),
		[]byte("version=2"),
	); err != nil {
		return err
	}

	var pkts [][]byte
	lines, err := p.r.ReadPackets()
	if err != nil {
		return err
	}
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("capability=")) {
			for _, c := range caps {
				if c == string(line[len("capability="):]) {
					pkts = append(pkts, line)
				}
			}
		}
	}
	if err = p.w.WritePackets(pkts...); err != nil {
		return err
	}
	if len(pkts) == 0 {
		return fmt.Errorf("no common capabilities (got %q, have %q)", lines, pkts)
	}
	return nil
}

// Reader reads git-pkt-line formatted slices.
type Reader struct {
	br     *bufio.Reader
	length [4]byte
	buf    []byte
	err    error
}

// NewReader returns a new git-pkt-line reader.
func NewReader(r io.Reader) *Reader { return &Reader{br: bufio.NewReaderSize(r, 65520)} }

// ReadPackets reads the input till a flush packet.
func (r *Reader) ReadPackets() ([][]byte, error) {
	var lines [][]byte
	for {
		line, err := r.ReadPacket()
		if len(line) == 0 || bytes.Equal(line, []byte("0000")) {
			return lines, err
		}
		lines = append(lines, append([]byte(nil), line...))
		if err != nil {
			return lines, err
		}
	}
}

// ReadPacket reads and returns then next packet (line).
// The returned buffer is valid only till the next call to ReadPacket/Read.
func (r *Reader) ReadPacket() ([]byte, error) {
	var err error
	if len(r.buf) == 0 {
		err = r.fill()
	}
	p := r.buf
	r.buf = r.buf[:0]
	return p, err
}

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
	length := int(r.length[0])<<8 + int(r.length[1])
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

func (w Writer) WritePackets(pkts ...[]byte) error {
	for _, p := range pkts {
		if _, err := w.Write(p); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("0000"))
	return err
}
