// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: LGPL-3.0

package journal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"
)

var bufPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}

func (rec Record) WriteTo(w io.Writer) (int64, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	defer func() { buf.Reset(); bufPool.Put(buf) }()
	buf.Reset()
	for _, kv := range [][2]string{
		{"MESSAGE", rec.Message},
		{"SYSLOG_IDENTIFIER", rec.SyslogIdentifier},
		{"CODE_FILE", rec.CodeFile},
		{"CODE_FUNC", rec.CodeFunc},
		{"__CURSOR", rec.Cursor},
	} {
		if kv[1] == "" {
			continue
		}
		if err := WriteField(buf, kv[0], kv[1]); err != nil {
			return 0, err
		}
	}
	for _, kv := range []struct {
		Key string
		Num int64
	}{
		{"__REALTIME_TIMESTAMP", rec.Realtime.UnixMicro()}, // microseconds since the epoch UTC
		{"CODE_LINE", int64(rec.CodeLine)},
		{"PRIORITY", int64(rec.Priority)},
	} {
		if kv.Num == 0 {
			continue
		}
		if err := WriteNumField(buf, kv.Key, kv.Num); err != nil {
			return 0, err
		}
	}
	for k, v := range rec.Fields {
		if err := WriteField(buf, k, v); err != nil {
			return 0, err
		}
	}
	if err := WriteEndOfEntry(w); err != nil {
		return 0, err
	}
	n, err := w.Write(buf.Bytes())
	return int64(n), err
}

func WriteJournalEntry(w io.Writer, priority int, message []byte, vars map[string]string) error {
	if err := WriteNumField(w, "PRIORITY", priority); err != nil {
		return err
	}
	if err := WriteField(w, "MESSAGE", string(message)); err != nil {
		return err
	}
	for k, v := range vars {
		if err := WriteField(w, k, v); err != nil {
			return err
		}
	}
	return WriteEndOfEntry(w)
}

// Copied from https://github.com/helixml/moby/blob/884aa4f88d65/daemon/logger/journald/internal/export/export.go

// Package journal implements a serializer for the systemd Journal Export Format
// as documented at https://systemd.io/JOURNAL_EXPORT_FORMATS/

// Returns whether s can be serialized as a field value "as they are" without
// the special binary safe serialization.
func isSerializableAsIs(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if c < ' ' && c != '\t' {
			return false
		}
	}
	return true
}

// WriteNumField writes a number(int64) field
func WriteNumField[T ~int | ~int64 | ~float32 | ~float64 | ~uint | ~uint64](w io.Writer, variable string, value T) error {
	_, err := fmt.Fprintf(w, "%s=%v\n", variable, value)
	return err
}

// WriteField writes the field serialized to Journal Export format to w.
//
// The variable name must consist only of uppercase characters, numbers and
// underscores. No validation or sanitization is performed.
func WriteField(w io.Writer, variable, value string) error {
	if isSerializableAsIs(value) {
		_, err := fmt.Fprintf(w, "%s=%s\n", variable, value)
		return err
	}

	if _, err := fmt.Fprintln(w, variable); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(value))); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, value)
	return err
}

// WriteEndOfEntry terminates the journal entry.
func WriteEndOfEntry(w io.Writer) error {
	_, err := w.Write([]byte{'\n'})
	return err
}
