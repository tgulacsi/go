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
	"net/http"
	"time"

	"golang.org/x/net/context"
	"gopkg.in/inconshreveable/log15.v2"

	"github.com/renstrom/shortuuid"
	"github.com/spkg/httpctx"
)

func AddLogger(Log log15.Logger, h httpctx.Handler) httpctx.Handler {
	return httpctx.HandlerFunc(
		func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			var id string
			idI := ctx.Value("reqid")
			if idI != nil {
				id, _ = idI.(string)
			}
			if id == "" {
				id = shortuuid.New()
				ctx = context.WithValue(ctx, "reqid", id)
			}
			Log := Log
			if lg := ctx.Value("logger"); lg != nil {
				Log = lg.(log15.Logger)
			} else {
				Log = Log.New("reqid", id)
				ctx = context.WithValue(ctx, "logger", Log)
			}
			w.Header().Set("X-Req-Id", id)
			start := time.Now()
			sr := &StatusRecorder{ResponseWriter: w}
			err := h.ServeHTTPContext(ctx, sr, r)
			d := time.Since(start)
			Log.Info("served", "path", r.URL.Path, "duration", d, "status", sr.StatusCode, "error", err)
			return err
		})
}

func GetLogger(Log log15.Logger, ctx context.Context) (log15.Logger, context.Context) {
	if lgI := ctx.Value("logger"); lgI != nil {
		return lgI.(log15.Logger), ctx
	}
	var id string
	idI := ctx.Value("reqid")
	if idI != nil {
		id, _ = idI.(string)
	}
	if id == "" {
		id = shortuuid.New()
		ctx = context.WithValue(ctx, "reqid", id)
	}
	Log = Log.New("reqid", id)
	return Log, context.WithValue(ctx, "logger", Log)
}

type StatusRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (sr *StatusRecorder) WriteHeader(code int) {
	sr.StatusCode = code
	sr.ResponseWriter.WriteHeader(code)
}
