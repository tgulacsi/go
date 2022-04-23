// Copyright 2022 Tamás Gulácsi. All rights reserved.

package zlog

import (
	"errors"
	"io"
	"os"
	"sync/atomic"

	"github.com/rs/zerolog"
)

var _ = zerolog.LevelWriter((*levelWriter)(nil))

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
