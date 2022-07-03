// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package zlog_test

import (
	"os"
	"testing"

	"github.com/tgulacsi/go/zlog"
)

func TestTerm(t *testing.T) {
	w := os.Stderr
	mw := zlog.MaybeConsoleWriter(w)
	t.Logf("%[1]v (%[1]T %[3]d) -> %[2]v (%[2]T)", w, mw, w.Fd())
	if mw == w {
		t.Log("stderr is not a terminal?")
	}
}
