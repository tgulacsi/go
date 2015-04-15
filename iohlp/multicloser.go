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
func (mc multiCloser) Insert(c ...io.Closer) {
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
