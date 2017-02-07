/*
Copyright 2016 Tamás Gulácsi

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
	"sort"

	"github.com/go-kit/kit/log"
)

// LogFunc is the Log function.
type LogFunc func(...interface{}) error

// New returns a LogContext, using Logfmt logger on w.
func New(w io.Writer) *LogContext {
	return NewContext(log.NewLogfmtLogger(w))
}

// NewContext wraps the given logger with Stringify, and adds a default ts timestamp.
func NewContext(logger log.Logger) *LogContext {
	return (&LogContext{Context: log.NewContext(Stringify{logger})}).
		With("ts", log.DefaultTimestamp)
}

// With appends the given plus keyvals to the LogFunc.
func With(oLog func(keyvals ...interface{}) error, plus ...interface{}) LogFunc {
	return LogFunc(func(keyvals ...interface{}) error {
		return oLog(append(keyvals, plus...)...)
	})
}

// NewTestLogger returns a Context wrapping a testing.TB.Log.
func NewTestLogger(t testLogger) *log.Context {
	return log.NewContext(
		Stringify{log.NewLogfmtLogger(testLog{t})},
	).With(
		"file", log.Caller(4),
	)
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

type LogContext struct {
	*log.Context
	keys []string
}

func (c *LogContext) With(keyvals ...interface{}) *LogContext {
	keys := c.keys[:len(c.keys):len(c.keys)]
	for i := 0; i < len(keyvals); i += 2 {
		var k string
		switch x := keyvals[i].(type) {
		case string:
			k = x
		case fmt.Stringer:
			k = x.String()
		default:
			k = fmt.Sprintf("%v", x)
		}
		j := sort.SearchStrings(keys, k)
		if !(j < len(keys) && keys[j] == k) {
			keys = append(append(keys[:j], k), keys[j:]...)
		}
	}
	return &LogContext{
		Context: c.Context.With(keyvals...),
		keys:    keys,
	}
}
func (c *LogContext) Keys() []string {
	return c.keys
}
func (c *LogContext) HasKey(k string) bool {
	j := sort.SearchStrings(c.keys, k)
	return j < len(c.keys) && c.keys[j] == k
}

// vim: set fileencoding=utf-8 noet:
