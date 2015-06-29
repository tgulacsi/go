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

package loghlp

import (
	"fmt"
	"io"
	"log"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/inconshreveable/log15.v2/stack"
)

// AsStdLog returns a *log.Logger from the given log15.Logger
func AsStdLog(lg log15.Logger, lvl log15.Lvl) *log.Logger {
	var w logWriter
	switch lvl {
	case log15.LvlCrit:
		w.Log = lg.Crit
	case log15.LvlError:
		w.Log = lg.Error
	case log15.LvlWarn:
		w.Log = lg.Warn
	case log15.LvlInfo:
		w.Log = lg.Info
	default:
		w.Log = lg.Debug
	}
	return log.New(w, "", 0)
}

type logWriter struct {
	Log func(msg string, ctx ...interface{})
}

var _ io.Writer = handlerWriter{}

func (w logWriter) Write(p []byte) (int, error) {
	// YYYY/MM/DD HH:MI:SS ...
	//var year, month,day,hour,minute,sec int
	//var msg string
	//fmt.Sscanf(string(p), "%d/%d/%d %d:%d:%d %s", &year, &month, &day, &hour, &minute, &sec, &msg)

	// strip \n
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\n' {
			continue
		}
		if i < len(p)-1 {
			p = p[:i+1]
		}
		break
	}
	w.Log(string(p))
	return len(p), nil
}

// WriterForStdLog returns an io.Writer suitable for use in log.New,
// and which will use the given log15.Handler
//
// This assumes that the log.Logger.Output will call the given io.Writer's
// Write method only once per log record, with the full output.
func WriterForStdLog(handler log15.Handler) io.Writer {
	return handlerWriter{handler}
}

var _ io.Writer = handlerWriter{}

type handlerWriter struct {
	handler log15.Handler
}

func (w handlerWriter) Write(p []byte) (int, error) {
	// [[YYYY/MM/DD ]HH:MI:SS[.ffffff] ][file:line: ]msg
	rec := log15.Record{Time: time.Now(), Lvl: log15.LvlInfo, Msg: string(p)}
	err := w.handler.Log(&rec)
	return len(p), err

}

// CallerFileHandler returns a Handler that adds the line number and file of
// the calling function to the context with key "caller".
//
// Skips skip number of lines from the top of the stack.
func CallerFileHandler(skip int, h log15.Handler) log15.Handler {
	return log15.FuncHandler(func(r *log15.Record) error {
		call := stack.Call(r.CallPC[0])
		if skip > 0 {
			callers := stack.Callers()
			if len(callers) > skip {
				call = callers[skip]
			} else {
				call = callers[len(callers)-1]
			}
		}
		r.Ctx = append(r.Ctx, "caller", fmt.Sprint(call))
		return h.Log(r)
	})
}
