/*
  Copyright 2013 Tamás Gulácsi

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

// Package httphdr provides some support for HTTP headers.
package httpctxhlp

import (
	"crypto/rand"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/oklog/ulid"
	"github.com/spkg/httpctx"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
)

func AddLogger(Log func(...interface{}) error, h httpctx.Handler) httpctx.Handler {
	return httpctx.HandlerFunc(
		func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			var id string
			idI := ctx.Value("reqid")
			if idI != nil {
				id, _ = idI.(string)
			}
			if id == "" {
				id = NewULID().String()
				ctx = context.WithValue(ctx, "reqid", id)
			}
			Log := Log
			if lg, _ := ctx.Value("Log").(func(...interface{}) error); lg != nil {
				Log = kitloghlp.With(Log, "reqid", id)
				ctx = context.WithValue(ctx, "Log", Log)
			}
			w.Header().Set("X-Req-Id", id)
			start := time.Now()
			sr := &StatusRecorder{ResponseWriter: w}
			err := h.ServeHTTPContext(ctx, sr, r)
			d := time.Since(start)
			Log("msg", "served", "path", r.URL.Path, "duration", d, "status", sr.StatusCode, "error", err)
			return err
		})
}

func NewULID() ulid.ULID {
	return ulid.MustNew(ulid.Now(), rand.Reader)
}

func GetLog(Log func(...interface{}) error, ctx context.Context) (func(...interface{}) error, context.Context) {
	if lgI, _ := ctx.Value("Log").(func(...interface{}) error); lgI != nil {
		Log = lgI
	}
	id, _ := ctx.Value("reqid").(string)
	if id == "" {
		id = NewULID().String()
		ctx = context.WithValue(ctx, "reqid", id)
		Log = kitloghlp.With(Log, "reqid", id)
	}
	return Log, context.WithValue(ctx, "Log", Log)
}

type StatusRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (sr *StatusRecorder) WriteHeader(code int) {
	sr.StatusCode = code
	sr.ResponseWriter.WriteHeader(code)
}
