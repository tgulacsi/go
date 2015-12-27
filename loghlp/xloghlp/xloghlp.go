// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package xloghlp

import (
	"bytes"
	"sync"
	"testing"

	"github.com/rs/xlog"
)

func TestLogger(t testing.TB) xlog.Logger {
	return xlog.New(xlog.Config{Output: TestOutput(t)})
}

func TestOutput(t testing.TB) xlog.Output {
	return &tstout{t: t}
}

type tstout struct {
	t testing.TB
	sync.Mutex
	buf    bytes.Buffer
	logFmt xlog.Output
}

func (to *tstout) Write(fields map[string]interface{}) error {
	to.Lock()
	defer to.Unlock()
	if to.logFmt == nil {
		to.logFmt = xlog.NewLogfmtOutput(&to.buf)
	}
	to.buf.Reset()
	err := to.logFmt.Write(fields)
	to.t.Log(to.buf.String())
	return err
}
