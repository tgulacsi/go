// Copyright 2020, 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package main of bddump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package main

import (
	"flag"
	"os"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/tgulacsi/go/bddump"
)

var (
	zl     = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	logger = zerologr.New(&zl)
)

func main() {
	bddump.SetLogger(logger.WithName("bddump"))

	flag.Parse()
	if err := bddump.Dump(os.Stdout, flag.Arg(0)); err != nil {
		logger.Error(err, "Dump", "src", flag.Arg(0))
		os.Exit(1)
	}
}
