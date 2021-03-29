// Copyright 2016, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"io"
	"sync"
)

// ConcurrentReader wraps the given io.Reader such that it can be called concurrently.
func ConcurrentReader(r io.Reader) io.Reader {
	return &concurrentReader{r: r}
}

type concurrentReader struct {
	r  io.Reader
	mu sync.Mutex
}

func (cr *concurrentReader) Read(p []byte) (int, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.r.Read(p)
}
