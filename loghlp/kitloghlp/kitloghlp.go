/*
Copyright 2017 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package kitloghlp contains some helpers for go-kit/kit/log.
package kitloghlp

import (
	"fmt"
	"io"

	"github.com/go-kit/kit/log"
)

// LogFunc is the Log function.
type LogFunc func(...interface{}) error

// New returns a Logger, using Logfmt logger with Stringify on w.
func New(w io.Writer) Stringify {
	return Stringify{log.NewLogfmtLogger(w)}
}

// NewTestLogger returns a Context wrapping a testing.TB.Log.
func NewTestLogger(t testLogger) log.Logger {
	return log.With(New(testLog{t}), "file", log.Caller(4))
}

type testLogger interface {
	Log(args ...interface{})
}
type testLog struct {
	testLogger
}

func (t testLog) Write(p []byte) (int, error) {
	t.Log(string(p))
	return len(p), nil
}

// Stringify stringifies every value to make it printable by logfmt.
//
// Example:
//	Logger := log.LogfmtLogger(os.Stderr)
//	Logger = log.Stringify{Logger}
type Stringify struct {
	log.Logger
}

// Log with stringifying every value.
func (l Stringify) Log(keyvals ...interface{}) error {
	for i := 1; i < len(keyvals); i += 2 {
		switch keyvals[i].(type) {
		case string, fmt.Stringer, fmt.Formatter:
		case error:
		default:
			keyvals[i] = StringWrap{Value: keyvals[i]}
		}
	}
	return l.Logger.Log(keyvals...)
}

var _ = fmt.Stringer(StringWrap{})

// StringWrap wraps the Value as a fmt.Stringer.
type StringWrap struct {
	Value interface{}
}

// String returns a string representation (%v) of the underlying Value.
func (sw StringWrap) String() string {
	return fmt.Sprintf("%v", sw.Value)
}

type MultiLogger []log.Logger

func (m MultiLogger) Log(keyvals ...interface{}) error {
	var firstErr error
	for _, lgr := range m {
		if err := lgr.Log(keyvals...); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// vim: set fileencoding=utf-8 noet:
