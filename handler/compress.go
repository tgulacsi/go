// Copyright 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
	"net/http"
	"sync"

	"github.com/klauspost/compress/gzhttp"
	"github.com/klauspost/compress/gzip"
)

var (
	wrappersMu sync.Mutex
	wrappers   map[int]func(http.Handler) http.HandlerFunc
)

// CompressHandler gzip compresses HTTP responses for clients that support it
// via the 'Accept-Encoding' header.
func CompressHandler(h http.Handler) http.Handler {
	return CompressHandlerLevel(h, gzip.BestSpeed)
}

// CompressHandlerLevel gzip compresses HTTP responses with specified compression level
// for clients that support it via the 'Accept-Encoding' header.
//
// The compression level should be gzip.DefaultCompression, gzip.NoCompression,
// or any integer value between gzip.BestSpeed and gzip.BestCompression inclusive.
// gzip.DefaultCompression is used in case of invalid compression level.
func CompressHandlerLevel(h http.Handler, level int) http.Handler {
	if ch, ok := h.(compressHandler); ok {
		return ch
	}
	wrappersMu.Lock()
	if wrappers == nil {
		wrappers = make(map[int]func(http.Handler) http.HandlerFunc)
	}
	wrapper, ok := wrappers[level]
	if !ok {
		var err error
		if wrapper, err = gzhttp.NewWrapper(gzhttp.MinSize(1024), gzhttp.CompressionLevel(level)); err != nil {
			panic(err)
		}
		wrappers[level] = wrapper
	}
	wrappersMu.Unlock()

	return compressHandler{Handler: wrapper(h)}
}

type compressHandler struct{ http.Handler }
