// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v4"

	"github.com/tgulacsi/go/safesql/inspectsql"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	collectCmd := ff.Command{Name: "collect",
		Exec: func(ctx context.Context, args []string) error {
			return nil
		},
	}
	inspectCmd := ff.Command{Name: "inspect",
		Exec: func(ctx context.Context, args []string) error {
			singlechecker.Main(inspectsql.Analyzer)
			return nil
		},
	}
	app := ff.Command{Name: "safesql", Subcommands: []*ff.Command{
		&collectCmd, &inspectCmd,
	}}
	if err := app.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			return nil
		}
		return err
	}
	copy(os.Args[1:], os.Args[2:])
	os.Args = os.Args[:len(os.Args)-1]
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return app.Run(ctx)
}
