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

package gloghlp

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"gopkg.in/inconshreveable/log15.v2"
)

// GLogHandler returns a log15.Handler which logs using glog.
func GLogHandler() log15.Handler {
	return glogHandler{GLogfmtFormat()}
}

func GLogfmtFormat() log15.Format {
	return log15.FormatFunc(func(r *log15.Record) []byte {
		_, file, line, ok := runtime.Caller(7) // It's always the same number of frames to the user's call.
		if !ok {
			file = "???"
			line = 1
		} else {
			slash := strings.LastIndex(file, "/")
			if slash >= 0 {
				file = file[slash+1:]
			}
		}
		if line < 0 {
			line = 0 // not a real line number, but acceptable to someDigits
		}
		var buf bytes.Buffer
		_, _ = fmt.Fprintf(&buf, "[%s:%d] %s", file, line, r.Msg)
		if len(r.Ctx) == 0 {
			return buf.Bytes()
		}
		_, _ = io.WriteString(&buf, "; ")
		for i := 0; i < len(r.Ctx)-1; i++ {
			if i > 0 {
				_ = buf.WriteByte(' ')
			}
			fmt.Fprintf(&buf, "%s=%q", r.Ctx[i], r.Ctx[i+1])
		}
		return buf.Bytes()
	})
}

type glogHandler struct {
	fmt log15.Format
}

func (gl glogHandler) Log(r *log15.Record) error {
	s := string(gl.fmt.Format(r)) // strip \n
	switch r.Lvl {
	case log15.LvlCrit:
		glog.Fatal(s)
	case log15.LvlError:
		glog.Error(s)
	case log15.LvlWarn:
		glog.Warning(s)
	case log15.LvlInfo:
		glog.Info(s)
	default:
		glog.V(1).Info(s)
	}
	return nil
}
