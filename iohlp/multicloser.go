// Copyright 2014, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package iohlp contains small io-related utility functions.
package iohlp

import (
	"io"
)

type multiCloser struct {
	closers []io.Closer
}

// NewMultiCloser returns an io.Closer which will close all contained io.Closer,
// in the given order.
func NewMultiCloser(c ...io.Closer) *multiCloser {
	return &multiCloser{c}
}

// Append appends new closers to the end (to be called later).
func (mc *multiCloser) Append(c ...io.Closer) {
	mc.closers = append(mc.closers, c...)
}

// Insert inserts new closers at the beginning (to be called first).
func (mc *multiCloser) Insert(c ...io.Closer) {
	mc.closers = append(append(make([]io.Closer, 0, len(c)+len(mc.closers)), c...), mc.closers...)
}

// Close which closes all contained Closers.
func (mc *multiCloser) Close() error {
	var err error
	for _, c := range mc.closers {
		if e := c.Close(); e != nil {
			if err == nil {
				err = e
			}
		}
	}
	mc.closers = mc.closers[:0]
	return err
}

var _ = io.Closer(CloserFunc(nil))

// CloserFunc implements the io.Closer.
type CloserFunc func() error

// Close calls the function.
func (cf CloserFunc) Close() error {
	return cf()
}
