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

package loghlp

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// Lazy returns an fmt.Stringer which returns the string returned by the given
// function.
//
// As this function is called only when the Stringer's String() function is
// called, this can be used in loggers to postpone expensive tasks.
func Lazy(f func() string) fmt.Stringer {
	return StringerFunc(f)
}

// LazyW returns an fmt.Stringer which returns the string written by the given
// function.
//
// The io.Writer given to the function is a *bytes.Buffer cached by a sync.Pool.
//
// As this function is called only when the Stringer's String() function is
// called, this can be used in loggers to postpone expensive tasks.
func LazyW(f func(io.Writer)) fmt.Stringer {
	return StringerFunc(func() string {
		buf := bufPool.Get().(*bytes.Buffer)
		defer func() { buf.Reset(); bufPool.Put(buf) }()
		buf.Reset()
		f(buf)
		return buf.String()
	})
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 1024)) }}

type StringerFunc func() string

func (f StringerFunc) String() string { return f() }
