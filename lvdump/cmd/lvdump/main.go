// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of lvdump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
	"github.com/tgulacsi/go/lvdump"
)

var logger = kitloghlp.Stringify{Logger: log.NewLogfmtLogger(os.Stderr)}

func main() {
	lvdump.Log = log.With(logger, "lib", "lvdump").Log

	flag.Parse()
	if err := lvdump.Dump(flag.Arg(0)); err != nil {
		logger.Log("msg", "Dump", "src", flag.Arg(0), "error", err)
	}
}
