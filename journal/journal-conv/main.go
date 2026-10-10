// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
	"os"
	"os/signal"

	"encoding/json/jsontext"
	"encoding/json/v2"

	"flag"
	"github.com/UNO-SOFT/cli"
	"github.com/tgulacsi/go/journal"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flags := flag.NewFlagSet("journal-conv", flag.ContinueOnError)
	flagFrom := flags.String("from", "export", "input format")
	flagTo := flags.String("to", "json", "output format")
	app := cli.Command{Name: "journal-conv", Flags: flags,
		Exec: func(ctx context.Context, state *cli.State) error {
			var inp iter.Seq2[journal.Record, error]
			switch *flagFrom {
			case "export":
				inp = journal.IterRecords(os.Stdin)
			case "json":
				inp = func(yield func(journal.Record, error) bool) {
					dec := jsontext.NewDecoder(os.Stdin)
					for {
						var rec journal.Record
						if err := json.UnmarshalDecode(dec, &rec); err != nil {
							if !errors.Is(err, io.EOF) {
								yield(rec, err)
							}
							return
						}
						if !yield(rec, nil) {
							return
						}
					}
				}
			default:
				return fmt.Errorf("unknown input formt %q", *flagFrom)
			}
			var print func(io.Writer, journal.Record) error
			switch *flagTo {
			case "json":
				opt := jsontext.AllowInvalidUTF8(true)
				print = func(w io.Writer, rec journal.Record) error {
					err := json.MarshalWrite(w, rec, opt)
					w.Write([]byte{'\n'})
					return err
				}
			case "export":
				print = func(w io.Writer, rec journal.Record) error { _, err := rec.WriteTo(w); return err }
			}

			bw := bufio.NewWriter(os.Stdout)
			for rec, err := range inp {
				if err != nil {
					return err
				}
				if err := print(bw, rec); err != nil {
					return err
				}
			}
			return bw.Flush()
		}, FlagConfigs: []cli.FlagConfig{cli.FlagConfig{Name: "from", Short: "f"}, cli.FlagConfig{Name: "to", Short: "t"}},
	}
	if err := cli.Parse(&app, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			cli.PrintHelp(os.Stderr, &app)
			return nil
		}
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return cli.Run(ctx, &app, nil)
}
