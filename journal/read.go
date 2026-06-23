// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: LGPL-3.0

package journal

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"iter"
	"strconv"
	"time"
)

type Record struct {
	Realtime         time.Time `json:"__REALTIME_TIMESTAMP"`
	Fields           map[string]string
	Message          string `json:"MESSAGE"`
	SyslogIdentifier string `json:"SYSLOG_IDENTIFIER"`
	Cursor           string `json:"__CURSOR"`
	CodeFile         string `json:"CODE_FILE"`
	CodeFunc         string `json:"CODE_FUNC"`
	CodeLine         uint32 `json:"CODE_LINE"`
	Priority         uint8  `json:"PRIORITY"` // 0=emerg 7=debug
}

func IterRecords(r io.Reader) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		br := bufio.NewReaderSize(r, 1<<20)
		for {
			var rec Record
			for kv, err := range iterKeyVals(br) {
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					yield(rec, err)
					return
				}
				switch string(kv.Key) {
				case "__REALTIME_TIMESTAMP":
					u, err := strconv.ParseInt(string(kv.Value), 10, 64)
					if err != nil {
						yield(rec, err)
						return
					}
					rec.Realtime = time.UnixMicro(u)
				case "MESSAGE":
					rec.Message = string(kv.Value)
				case "SYSLOG_IDENTIFIER":
					rec.SyslogIdentifier = string(kv.Value)
				case "__CURSOR":
					rec.Cursor = string(kv.Value)
				case "CODE_FILE":
					rec.CodeFile = string(kv.Value)
				case "CODE_LINE":
					u, err := strconv.ParseUint(string(kv.Value), 10, 32)
					if err != nil {
						yield(rec, err)
						return
					}
					rec.CodeLine = uint32(u)
				case "PRIORITY":
					u, err := strconv.ParseUint(string(kv.Value), 10, 8)
					if err != nil {
						yield(rec, err)
						return
					}
					rec.Priority = uint8(u)
				default:
					if rec.Fields == nil {
						rec.Fields = make(map[string]string)
					}
					rec.Fields[string(kv.Key)] = string(kv.Value)
				}
			}
			if !yield(rec, nil) {
				return
			}
		}
	}
}

type KeyVal struct {
	Key, Value, Size []byte
}

func (kv KeyVal) WriteTo(w io.Writer) (int64, error) {
	var written int64
	var err error
	W := func(p []byte) error {
		if err != nil {
			return err
		}
		var n int
		n, err = w.Write(p)
		written += int64(n)
		return err
	}
	W(kv.Key)
	if len(kv.Size) == 0 {
		W([]byte{'='})
	} else {
		W([]byte{'\n'})
		W(kv.Size)
	}
	W(kv.Value)
	W([]byte{'\n'})
	return written, err
}

// CopyJournalRecord copies JOURNAL_EXPORT format, one record a time.
func CopyJournalRecord(w io.Writer, br *bufio.Reader) (int64, error) {
	var written int64
	for kv, err := range iterKeyVals(br) {
		if err != nil {
			return written, err
		}
		n, err := kv.WriteTo(w)
		written += int64(n)
		if err != nil {
			return written, err
		}
	}
	_, err := w.Write([]byte{'\n'})
	return written, err
}

// iterExport returns an interator over the JOURNAL_EXPORT format - but only one entry
func iterKeyVals(br *bufio.Reader) iter.Seq2[KeyVal, error] {
	return func(yield func(KeyVal, error) bool) {
		for {
			line, err := br.ReadSlice('\n')
			if err != nil {
				yield(KeyVal{}, fmt.Errorf("read till EOL: %w", err))
				return
			}

			// End-of-entry: blank line
			if len(line) == 1 && bytes.Equal(line, []byte{'\n'}) ||
				len(line) == 2 && bytes.Equal(line, []byte("\r\n")) {
				return
			}

			trimmed := bytes.TrimRight(line, "\r\n")
			if bytes.HasPrefix(trimmed, []byte("-- cursor: ")) {
				continue
			}

			// Text form: FIELD=value
			if i := bytes.IndexByte(trimmed, '='); i > 0 {
				if !yield(KeyVal{Key: trimmed[:i], Value: trimmed[i+1:]}, nil) {
					return
				}
				continue
			}

			// Binary-safe form:
			// FIELD\n + 8-byte little-endian length + <data> + '\n'
			kv := KeyVal{Key: append([]byte(nil), trimmed...)}
			var szBuf [8]byte
			if _, err2 := io.ReadFull(br, szBuf[:]); err2 != nil {
				yield(kv, fmt.Errorf("read size for %q: %w", string(kv.Key), err2))
				return
			}
			size := binary.LittleEndian.Uint64(szBuf[:])

			// Guardrail: avoid absurd allocations on corrupted input
			// (tune this as you like; journald fields are usually small)
			if size > uint64(br.Size()) {
				yield(kv, fmt.Errorf("field %q too large: %d bytes", string(kv.Key), size))
				return
			}
			kv.Size = append([]byte(nil), szBuf[:]...)
			kv.Value = make([]byte, int(size))
			if _, err := io.ReadFull(br, kv.Value); err != nil {
				yield(kv, err)
				return
			}

			// Consume the trailing '\n' separator after the binary payload
			var b byte
			if b, err = br.ReadByte(); err != nil {
				err = fmt.Errorf("read newline after %q: %w", string(kv.Key), err)
			} else if b != '\n' {
				err = fmt.Errorf("expected newline after %q data, got 0x%02x", string(kv.Key), b)
			}
			if !yield(kv, err) {
				return
			}
			if err != nil {
				return
			}
		}
	}
}
