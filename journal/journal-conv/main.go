// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: LGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
	"os"
	"os/signal"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/tgulacsi/go/journal"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flags := ff.NewFlagSet("journal-conv")
	flagFrom := flags.StringEnum('f', "from", "input format", "export", "json")
	flagTo := flags.StringEnum('t', "to", "output format", "json", "export")
	app := ff.Command{Name: "journal-conv", Flags: flags,
		Exec: func(ctx context.Context, args []string) error {
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
				print = func(w io.Writer, rec journal.Record) error { return json.MarshalWrite(w, rec, opt) }
			case "export":
				print = func(w io.Writer, rec journal.Record) error { _, err := rec.WriteTo(w); return err }
			}

			for rec, err := range inp {
				if err != nil {
					return err
				}
				if err := print(os.Stdout, rec); err != nil {
					return err
				}
			}
			return nil
		},
	}
	if err := app.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			ffhelp.Command(&app).WriteTo(os.Stderr)
			return nil
		}
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return app.Run(ctx)
}
