// Copyright 2017, 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of lvdump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/lvdump"
)

var logger = zlog.New(os.Stderr).SLog()

func main() {
	slog.SetDefault(logger)

	flag.Parse()
	if err := lvdump.Dump(os.Stdout, flag.Arg(0)); err != nil {
		logger.Error("Dump", "src", flag.Arg(0), "error", err)
	}
}
