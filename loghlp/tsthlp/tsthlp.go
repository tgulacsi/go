/*
Copyright 2014 Tamás Gulácsi

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

// Package loghlp collects some small log15 handlers
package tsthlp

import (
	"fmt"
	"testing"

	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/inconshreveable/log15.v2/stack"
)

// TestHandler returns a log15.Handler which logs using testing.T.Logf,
// thus pringing only if the tests are colled with -v.
func TestHandler(t *testing.T) log15.Handler {
	return CallerFileHandler(0, log15.LazyHandler(testLogHandler{t, log15.LogfmtFormat()}))
}

// CallerFileHandler returns a Handler that adds the line number and file of
// the calling function to the context with key "caller".
// The callOff parameter defines which caller will be printed.
func CallerFileHandler(callOff int, h log15.Handler) log15.Handler {
	return log15.FuncHandler(func(r *log15.Record) error {
		call := stack.Call(r.CallPC[callOff])
		r.Ctx = append(r.Ctx, "caller", fmt.Sprint(call))
		return h.Log(r)
	})
}

type testLogHandler struct {
	*testing.T
	fmt log15.Format
}

func (tl testLogHandler) Log(r *log15.Record) error {
	b := tl.fmt.Format(r)
	tl.T.Log(string(b[:len(b)-1])) // strip \n
	return nil
}
