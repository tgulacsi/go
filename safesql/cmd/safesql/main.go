// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/tgulacsi/go/safesql/inspectsql"

	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/txtar"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flags := ff.NewFlagSet("collect")
	flagCollectExecute := flags.StringLong("exec", "", "execute this JSON array with the SQL as {} argument, or stdin if not {} has given")
	collectCmd := ff.Command{Name: "collect", Flags: flags,
		Exec: func(ctx context.Context, args []string) error {
			initial, err := packages.Load(&packages.Config{
				Mode: packages.LoadAllSyntax,
			}, args...)
			if err != nil {
				return err
			}
			analyzers := []*analysis.Analyzer{inspectsql.Analyzer}
			graph, err := checker.Analyze(analyzers, initial, &checker.Options{FactLog: os.Stderr})
			if err != nil {
				return err
			}
			var ar txtar.Archive
			var todo func(q *inspectsql.SQLQuery) error
			if *flagCollectExecute == "" {
				todo = func(q *inspectsql.SQLQuery) error {
					ar.Files = append(ar.Files, txtar.File{
						Name: q.Position.String(),
						Data: []byte(q.Query),
					})
					return nil
				}
			} else {
				var args []string
				if (*flagCollectExecute)[0] != '[' {
					args = append(args, *flagCollectExecute)
				} else if err := json.Unmarshal([]byte(*flagCollectExecute), &args); err != nil {
					return err
				}
				prog, args := args[0], args[1:]
				argIdx := slices.Index(args, "{}")
				todo = func(q *inspectsql.SQLQuery) error {
					if argIdx >= 0 {
						args = append(make([]string, 0, len(args)), args...)
						args[argIdx] = q.Query
					}
					cmd := exec.CommandContext(ctx, prog, args...)
					if argIdx == -1 {
						cmd.Stdin = strings.NewReader(q.Query)
					}
					b, err := cmd.CombinedOutput()
					os.Stdout.Write(b)
					return err
				}
			}
			var grp errgroup.Group
			grp.SetLimit(runtime.GOMAXPROCS(-1))
			for a := range graph.All() {
				for _, f := range a.AllPackageFacts() {
					q := f.Fact.(*inspectsql.SQLQuery)
					grp.Go(func() error {
						if err := todo(q); err != nil {
							return fmt.Errorf("%s: %w", q.Position.String(), err)
						}
						return nil
					})
				}
			}
			if len(ar.Files) != 0 {
				_, err = os.Stdout.Write(txtar.Format(&ar))
				return err
			}
			return grp.Wait()
		},
	}
	inspectCmd := ff.Command{Name: "inspect",
		Exec: func(ctx context.Context, args []string) error {
			copy(os.Args[1:], args)
			singlechecker.Main(inspectsql.Analyzer)
			return nil
		},
	}

	app := ff.Command{Name: "safesql", Subcommands: []*ff.Command{
		&collectCmd, &inspectCmd,
	}}
	if err := app.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			ffhelp.Command(&app).WriteTo(os.Stderr)
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
