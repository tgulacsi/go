// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of lvdump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"

	"github.com/tgulacsi/go/lvdump"
	"gopkg.in/inconshreveable/log15.v2"
)

var Log = lvdump.Log

func main() {
	Log.SetHandler(log15.CallerFileHandler(log15.StderrHandler))

	flag.Parse()
	if err := lvdump.Dump(flag.Arg(0)); err != nil {
		Log.Error("Dump", "src", flag.Arg(0), "error", err)
	}
}
