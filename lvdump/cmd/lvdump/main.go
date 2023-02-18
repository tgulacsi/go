// Copyright 2017, 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of lvdump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"
	"os"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/lvdump"
)

var logger = zlog.New(os.Stderr)

func main() {
	lvdump.Log = logger.WithGroup("lvdump").Log

	flag.Parse()
	if err := lvdump.Dump(os.Stdout, flag.Arg(0)); err != nil {
		logger.Log("msg", "Dump", "src", flag.Arg(0), "error", err)
	}
}
