// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package httpclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/klauspost/compress/gzhttp"
)

func MakeHTTPClient(cl *http.Client, options ...Option) *http.Client {
	if cl == nil {
		cl = http.DefaultClient
	}
	tr := cl.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	for _, o := range options {
		tr = o(tr)
	}
	cl.Transport = gzhttp.Transport(tr)
	return cl
}

type Option func(tr http.RoundTripper) http.RoundTripper

func WithExtraHeaders(extraHeaders map[string]map[string]string) Option {
	return func(tr http.RoundTripper) http.RoundTripper {
		return extraHeadersTransport{RoundTripper: tr, ExtraHeaders: extraHeaders}
	}
}

type extraHeadersTransport struct {
	http.RoundTripper
	ExtraHeaders map[string]map[string]string
}

func (tr extraHeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	var longestPrefix string
	for prefix := range tr.ExtraHeaders {
		if strings.HasPrefix(u, prefix) && len(prefix) > len(longestPrefix) {
			longestPrefix = prefix
		}
	}
	for k, v := range tr.ExtraHeaders[longestPrefix] {
		req.Header.Set(k, v)
	}

	return tr.RoundTripper.RoundTrip(req)
}

func WithLogger(
	getLogger func(context.Context) *slog.Logger,
	getReqID func(context.Context) (context.Context, string),
) Option {
	return func(tr http.RoundTripper) http.RoundTripper {
		return transportLogger{
			RoundTripper: tr,
			GetLogger:    getLogger,
			GetReqID:     getReqID,
		}
	}
}

type transportLogger struct {
	http.RoundTripper
	GetReqID  func(context.Context) (context.Context, string)
	GetLogger func(context.Context) *slog.Logger
}

func (tr transportLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqID string
	ctx := req.Context()
	if tr.GetReqID != nil {
		ctx, reqID = tr.GetReqID(ctx)
	}
	if reqID == "" {
		reqID = req.Header.Get("Request-ID")
	}
	var logger *slog.Logger
	if tr.GetLogger != nil {
		logger = tr.GetLogger(ctx)
	}
	if logger == nil {
		logger = zlog.SFromContext(ctx)
	}
	var start time.Time
	if logger != nil {
		var b []byte
		if logger.Enabled(ctx, slog.LevelDebug) {
			var err error
			if b, err = httputil.DumpRequestOut(req, true); err != nil {
				return nil, fmt.Errorf("dump request: %w", err)
			}
		}
		logger = logger.With(
			slog.String("url", req.URL.String()),
			slog.String("reqID", reqID),
			slog.Group("request",
				slog.String("method", req.Method),
				slog.Any("header", req.Header),
				slog.String("body", string(b)),
			),
		)
		start = time.Now()
	}
	resp, err := tr.RoundTripper.RoundTrip(req.WithContext(ctx))
	if logger == nil {
		return resp, err
	}
	logger = logger.With("dur", time.Since(start).String())
	lvl := slog.LevelInfo
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			lvl = slog.LevelWarn
		}
		lvl = slog.LevelError
	}
	if resp != nil {
		var b []byte
		if logger.Enabled(ctx, slog.LevelDebug) {
			if b, err = httputil.DumpResponse(resp, true); err != nil {
				return resp, fmt.Errorf("dump response: %w", err)
			}
		}
		logger = logger.With(slog.Group("response",
			slog.String("status", resp.Status),
			slog.Any("header", resp.Header),
			slog.String("body", string(b)),
		))
	}
	logger.Log(ctx, lvl, "response")
	return resp, err
}
