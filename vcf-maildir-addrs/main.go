// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/mail"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"

	"github.com/tgulacsi/go/vcf-maildir-addrs/mailaddr"
	"github.com/tgulacsi/go/vcf-maildir-addrs/vcf"
)

func main() {
	if err := Main(); err != nil {
		slog.Error("main", "error", err)
		os.Exit(1)
	}
}

func Main() error {
	FS := flag.NewFlagSet("vcf-maildir-addrs", flag.ContinueOnError)
	flagContacts := FS.String("contacts", "~/.contacts", "contacts dir")
	flagConcurrency := FS.Int("concurrency", 8, "concurrency")
	flagMail := FS.String("maildir", "~/mail", "mail dir")
	app := ffcli.Command{Name: "vcf-maildir-addrs", FlagSet: FS,
		Exec: func(ctx context.Context, args []string) error {
			grp, grpCtx := errgroup.WithContext(ctx)
			if *flagConcurrency <= 0 {
				*flagConcurrency = 8
			}
			var (
				addrsMu sync.Mutex
				addrs   = make([]mail.Address, 0, 4096)
			)
			A := func(aa []mail.Address) {
				if len(aa) == 0 {
					return
				}
				for i, a := range aa {
					if a.Name != "" && a.Name[0] == '\'' {
						a.Name = strings.Trim(a.Name, "' ")
						aa[i] = a
					}
				}
				addrsMu.Lock()
				defer addrsMu.Unlock()
				if len(addrs) == 0 {
					addrs = append(addrs, aa[0])
					aa = aa[1:]
					if len(aa) == 0 {
						return
					}
				}
				for _, a := range aa {
					if i, found := slices.BinarySearchFunc(addrs, a, func(a, b mail.Address) int {
						return strings.Compare(strings.ToLower(a.Address), strings.ToLower(b.Address))
					}); !found {
						addrs = slices.Insert(addrs, i, a)
					}
				}
			}
			grp.SetLimit(*flagConcurrency)
			grp.Go(func() error {
				return filepath.WalkDir(
					strings.Replace(*flagContacts, "~/", os.Getenv("HOME")+"/", 1),
					func(path string, di fs.DirEntry, err error) error {
						if err != nil {
							slog.Warn("walk", "path", path, "error", err)
							return nil
						}
						if err := grpCtx.Err(); err != nil {
							return nil
						}
						if !(di.Type().IsRegular() && strings.HasSuffix(di.Name(), ".vcf")) {
							return nil
						}
						// slog.Info(path)
						grp.Go(func() error {
							fh, err := os.Open(path)
							if err != nil {
								return fmt.Errorf("open %q: %w", fh.Name(), err)
							}
							aa, err := vcf.ScanForAddrs(fh)
							// slog.Info("scan", "aa", aa, "error", err)
							fh.Close()
							if len(aa) == 0 {
								if err != nil {
									err = fmt.Errorf("%q: %w", fh.Name(), err)
								}
								return err
							}
							A(aa)
							return nil
						})
						return nil
					})
			})

			grp.Go(func() error {
				return filepath.WalkDir(
					strings.Replace(*flagMail, "~/", os.Getenv("HOME")+"/", 1),
					func(path string, di fs.DirEntry, err error) error {
						if err != nil {
							slog.Warn("walk", "path", path, "error", err)
							return nil
						}
						if err := grpCtx.Err(); err != nil {
							return nil
						}
						if dir := filepath.Base(filepath.Dir(path)); !(di.Type().IsRegular() && (dir == "cur" || dir == "tmp" || dir == "new")) {
							return nil
						}
						grp.Go(func() error {
							fh, err := os.Open(path)
							if err != nil {
								return err
							}
							aa, err := mailaddr.ScanForAddrs(fh)
							fh.Close()
							if len(aa) == 0 {
								if err != nil {
									err = fmt.Errorf("%q: %w", fh.Name(), err)
								}
								return err
							}
							A(aa)
							return nil
						})
						return nil
					})
			})
			err := grp.Wait()
			for _, a := range addrs {
				fmt.Printf("%s\t%s\n", a.Address, a.Name)
			}
			return err
		},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return app.ParseAndRun(ctx, os.Args[1:])
}
