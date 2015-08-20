/*
Copyright 2015 Tamás Gulácsi

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

package kithlp

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/inconshreveable/log15.v2/term"
	"gopkg.in/kit.v0/log"
	"gopkg.in/logfmt.v0"
)

// NewTerminalLogger returns a log.Logger which prouces nice colored logs,
// but only if the Writer is a tty.
// It is a copy of http://godoc.org/gopkg.in/inconshreveable/log15.v2/#TerminalFormat
//
// Otherwise it will return the alternate logger.
func NewTerminalLogger(w io.Writer, alternate log.Logger) log.Logger {
	var isTTY bool
	if std, ok := w.(fder); ok {
		isTTY = term.IsTty(std.Fd())
	}
	if !isTTY {
		return alternate
	}
	return &terminalLogger{
		w:          w,
		timeFormat: time.RFC3339,
		msgKey:     "msg",
		tsKey:      "ts",
		levelKey:   "level",

		debugValue: "debug",
		infoValue:  "info",
		warnValue:  "warn",
		errorValue: "error",
		critValue:  "crit",
	}
}

type terminalLogger struct {
	w io.Writer

	msgKey     string
	tsKey      string
	levelKey   string
	timeFormat string

	debugValue string
	infoValue  string
	warnValue  string
	errorValue string
	critValue  string
}

func (l terminalLogger) Log(keyvals ...interface{}) error {
	var ts, msg, level string

	for i := 0; i < len(keyvals); i += 2 {
		var found bool
		switch keyvals[i] {
		case l.msgKey:
			if msg == "" {
				msg = asString(keyvals[i+1])
				found = true
			}
		case l.levelKey:
			if level == "" {
				level = asString(keyvals[i+1])
				found = true
			}
		case l.tsKey:
			if ts == "" {
				ts = asTimeString(keyvals[i+1], l.timeFormat)
				found = true
			}
		}
		if found { // delete
			if len(keyvals) == i-2 {
				keyvals = keyvals[:i]
			} else {
				keyvals = append(keyvals[:i], keyvals[i+2:]...)
			}
			i -= 2
		}
	}

	// copied from gopkg.in/inconshreveable/log15.v2/format.go
	// ---8<---
	var color = 0
	switch level {
	case l.critValue:
		color = 35
	case l.errorValue:
		color = 31
	case l.warnValue:
		color = 33
	case l.infoValue:
		color = 32
	case l.debugValue:
		color = 36
	}
	if level == "" {
		level = l.infoValue
	}
	lvl := strings.ToUpper(level)
	if color > 0 {
		fmt.Fprintf(l.w, "\x1b[%dm%s\x1b[0m[%s] %s ", color, lvl, ts, msg)
	} else {
		fmt.Fprintf(l.w, "[%s] [%s] %s ", lvl, ts, msg)
	}
	// --->8---

	// copied from gopkg.in/kit.v0/log/logfmt_logger.go
	// ---8<---
	b, err := logfmt.MarshalKeyvals(keyvals...)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := l.w.Write(b); err != nil {
		return err
	}
	return nil
	// --->8---
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case fmt.Formatter:
		return fmt.Sprint(x)
	default:
		return fmt.Sprintf("%s", x)
	}
	return ""
}
func asTimeString(v interface{}, timeFormat string) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case time.Time:
		return x.Format(timeFormat)
	default:
		return asString(x)
	}
	return ""
}

type fder interface {
	Fd() uintptr
}
