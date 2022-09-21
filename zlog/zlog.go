// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package zlog contains some very simple go-logr / zerologr helper functions.
// This sets the default timestamp format to time.RFC3339 with ms precision.
package zlog

import (
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"golang.org/x/term"
)

var _ = zerolog.LevelWriter((*levelWriter)(nil))

func init() {
	zerolog.TimestampFieldName = "ts"
	zerolog.LevelFieldName = "lvl"
}

const (
	TraceLevel = zerolog.TraceLevel
	InfoLevel  = zerolog.InfoLevel
)

type levelWriter struct {
	ws        atomic.Value
	threshold int32
}

// Write the bytes to all specified writers.
func (lw *levelWriter) Write(p []byte) (n int, err error) {
	var firstErr error
	var hasClosed bool
	ws := lw.ws.Load().([]io.Writer)
	for i := 0; i < len(ws); i++ {
		if _, err := ws[i].Write(p); err != nil && firstErr == nil && !errors.Is(err, os.ErrClosed) {
			firstErr = err
		} else if errors.Is(err, os.ErrClosed) {
			ws[i] = ws[0]
			ws = ws[1:]
			hasClosed = true
			i--
		}
	}
	if hasClosed {
		lw.ws.Store(ws)
	}
	return len(p), firstErr
}

// WriteLevel writes to the underlying writers iff level < the specified threshold.
func (lw *levelWriter) WriteLevel(level zerolog.Level, p []byte) (n int, err error) {
	if int32(level) < atomic.LoadInt32(&lw.threshold) {
		return len(p), nil
	}
	var firstErr error
	var hasClosed bool
	ws := lw.ws.Load().([]io.Writer)
	for i := 0; i < len(ws); i++ {
		var err error
		w := ws[i]
		if lw, ok := w.(zerolog.LevelWriter); ok {
			_, err = lw.WriteLevel(level, p)
		} else {
			_, err = w.Write(p)
		}
		if err != nil && firstErr == nil && !errors.Is(err, os.ErrClosed) {
			firstErr = err
		} else if errors.Is(err, os.ErrClosed) {
			ws[i] = ws[0]
			ws = ws[1:]
			hasClosed = true
			i--
		}
	}
	if hasClosed {
		lw.ws.Store(ws)
	}
	return len(p), firstErr
}

// NewLevelWriter returns a new zerolog.LevelWriter that discards messages under the given threshold,
// and writes to all the specified writers.
func NewLevelWriter(threshold zerolog.Level, ws ...io.Writer) *levelWriter {
	lw := levelWriter{threshold: int32(threshold)}
	lw.ws.Store(ws)
	return &lw
}

// Add an additional writer to the targets.
func (lw *levelWriter) Add(w io.Writer) { lw.ws.Store(append(lw.ws.Load().([]io.Writer), w)) }

// Swap the current writers with the defined.
func (lw *levelWriter) Swap(ws ...io.Writer) { lw.ws.Store(ws) }

// SetLevel sets the threshold.
func (lw *levelWriter) SetLevel(level zerolog.Level) { atomic.StoreInt32(&lw.threshold, int32(level)) }

func init() {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.999Z07:00"
}

// NewZerolog returns a new zerolog.Logger writing to w.
func NewZerolog(w io.Writer) zerolog.Logger {
	return zerolog.New(w).With().Timestamp().Logger()
}

// New returns a new logr.Logger writing to w as a zerolog.Logger,
// at InfoLevel.
func New(w io.Writer) logr.Logger {
	zl := NewZerolog(w).Level(zerolog.InfoLevel)
	return zerologr.New(&zl)
}

// SetLevel sets the level on the underlying zerolog.Logger, directly.
func SetLevel(lgr logr.Logger, level zerolog.Level) {
	if underlier, ok := lgr.GetSink().(zerologr.Underlier); ok {
		zl := underlier.GetUnderlying()
		*zl = zl.Level(level)
	}
}

// SetOutput sets the output on the underlying zerolog.Logger, directly.
func SetOutput(lgr logr.Logger, w io.Writer) {
	if underlier, ok := lgr.GetSink().(zerologr.Underlier); ok {
		zl := underlier.GetUnderlying()
		*zl = zl.Output(w)
	}
}

// MaybeConsoleWriter returns a zerolog.ConsoleWriter if w is a terminal, and w unchanged otherwise.
func MaybeConsoleWriter(w io.Writer) io.Writer {
	if fder, ok := w.(interface{ Fd() uintptr }); ok {
		if term.IsTerminal(int(fder.Fd())) {
			return zerolog.ConsoleWriter{Out: w, TimeFormat: "15:04:05"}
		}
	}
	return w
}
