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

package tracehlp

import (
	"fmt"
	"io"

	"github.com/opentracing/basictracer-go"
	"github.com/opentracing/opentracing-go"
	"github.com/tgulacsi/go/loghlp"
)

// NewLoggerTracer returns a new basictracer with the given processName and Log function.
func NewLoggerTracer(processName string, Log func(keyvals ...interface{}) error) opentracing.Tracer {
	return basictracer.New(NewLoggerRecorder(processName, Log))
}

// LoggerRecorder implements the basictracer.Recorder interface.
type LoggerRecorder struct {
	processName string
	tags        map[string]string
	Log         func(keyvals ...interface{}) error
}

// NewLoggerRecorder returns a LoggerRecorder for the given `processName`.
func NewLoggerRecorder(processName string, Log func(keyvals ...interface{}) error) *LoggerRecorder {
	return &LoggerRecorder{
		processName: processName,
		tags:        make(map[string]string),
		Log:         Log,
	}
}

// ProcessName returns the process name.
func (t *LoggerRecorder) ProcessName() string { return t.processName }

// SetTag sets a tag.
func (t *LoggerRecorder) SetTag(key string, val interface{}) *LoggerRecorder {
	t.tags[key] = fmt.Sprint(val)
	return t
}

// RecordSpan complies with the basictracer.Recorder interface.
func (t *LoggerRecorder) RecordSpan(span basictracer.RawSpan) {
	if t.Log == nil {
		return
	}
	t.Log(
		"msg", "record span",
		"operation", span.Operation,
		"start", span.Start,
		"duration", span.Duration,
		//"context", span.Context,
		"baggage", span.Context.Baggage,
		"logs", loghlp.LazyW(func(w io.Writer) {
			for i, l := range span.Logs {
				fmt.Fprintf(w, "%d:{%v @ %v} ",
					i, l.Timestamp, l.Fields)
			}
		},
		))
}
