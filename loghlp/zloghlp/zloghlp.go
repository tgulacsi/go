// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package zloghlp contains some very simple go-logr / zerologr helper functions.
// This sets the default timestamp format to time.RFC3339 with ms precision.
package zloghlp

import (
	"io"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.999Z07:00"
}

// NewZerolog returns a new zerolog.Logger writing to w.
func NewZerolog(w io.Writer) zerolog.Logger {
	return zerolog.New(w).With().Timestamp().Logger()
}

// New returns a new logr.Logger writing to w as a zerolog.Logger.
func New(w io.Writer) logr.Logger {
	zl := NewZerolog(w)
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
