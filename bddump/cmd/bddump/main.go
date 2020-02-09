// Copyright 2020 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of bddump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/tgulacsi/go/bddump"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
)

var logger = kitloghlp.Stringify{Logger: log.NewLogfmtLogger(os.Stderr)}

func main() {
	bddump.Log = log.With(logger, "lib", "bddump").Log

	flag.Parse()
	if err := bddump.Dump(os.Stdout, flag.Arg(0)); err != nil {
		logger.Log("msg", "Dump", "src", flag.Arg(0), "error", err)
	}
}
