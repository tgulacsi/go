// Copyright 2011-2017, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/tgulacsi/go/dbcsv"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	_ "gopkg.in/goracle.v2"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)

	//Log = func(keyvals ...interface{}) error { return nil }
)

func main() {
	if lang := os.Getenv("LANG"); lang != "" {
		if i := strings.LastIndex(lang, "."); i >= 0 {
			lang = lang[i+1:]
			enc, err := htmlindex.Get(lang)
			if err != nil {
				log.Fatalf("Get encoding for %q: %v", lang, err)
			}
			stdout = transform.NewWriter(stdout, enc.NewEncoder())
			stderr = transform.NewWriter(stderr, enc.NewEncoder())
			log.SetOutput(stderr)
		}
	}
	bw := bufio.NewWriter(stdout)
	defer bw.Flush()
	stdout = bw

	var cfg dbcsv.Config
	flag.IntVar(&cfg.Sheet, "sheet", 0, "Index of sheet to convert, zero based")
	flagConnect := flag.String("connect", "$BRUNO_ID", "database connection string")
	flagFunc := flag.String("call", "DBMS_OUTPUT.PUT_LINE", "function name to be called with each line")
	flagFixParams := flag.String("fix", "p_file_name=>{{.FileName}}", "fix parameters to add; uses text/template")
	flagFuncRetOk := flag.Int("call-ret-ok", 0, "OK return value")
	flagOneTx := flag.Bool("one-tx", true, "one transaction, or commit after each row")
	flag.StringVar(&cfg.Delim, "d", "", "Delimiter to use between fields")
	flag.StringVar(&cfg.Charset, "charset", "utf-8", "input charset")
	flag.IntVar(&cfg.Skip, "skip", 1, "skip first N rows")
	flag.StringVar(&cfg.ColumnsString, "columns", "", "column numbers to use, separated by comma, in param order, starts with 1")
	//flagVerbose := flag.Bool("v", false, "verbose logging")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s

	The specified code will be called with the cells as (string) arguments
	(except dates, where DATE will be provided), for each row.

Usage:
	%s [flags] <xlsx/xls/csv-to-be-read>
`, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	ctxData := struct {
		FileName string
	}{FileName: flag.Arg(0)}
	var fixParams [][2]string
	var buf bytes.Buffer
	if strings.TrimSpace(*flagFixParams) != "" {
		for _, tup := range strings.Split(*flagFixParams, ",") {
			parts := strings.SplitN(tup, "=>", 2)
			tpl := template.Must(template.New(parts[0]).Parse(parts[1]))
			buf.Reset()
			if err := tpl.Execute(&buf, ctxData); err != nil {
				log.Fatal(err)
			}
			fixParams = append(fixParams, [2]string{parts[0], buf.String()})
		}
	}

	inp := os.Stdin
	if flag.Arg(0) != "" && flag.Arg(0) != "-" {
		var err error
		if inp, err = os.Open(flag.Arg(0)); err != nil {
			log.Fatal("open %q: %v", flag.Arg(0), err)
		}
	}
	defer inp.Close()

	rows := make(chan dbcsv.Row, 8)
	errch := make(chan error, 8)
	errs := make([]string, 0, 8)
	var errWg sync.WaitGroup
	errWg.Add(1)
	go func() {
		defer errWg.Done()
		for err := range errch {
			if err != nil {
				log.Printf("ERROR: %v", err)
				errs = append(errs, err.Error())
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(rows chan<- dbcsv.Row) {
		defer close(rows)
		errch <- cfg.ReadRows(ctx,
			func(_ string, row dbcsv.Row) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case rows <- row:
				}
				return nil
			},
			inp.Name(),
		)
	}(rows)

	// filter out empty rows
	{
		filtered := make(chan dbcsv.Row, 8)
		go func(filtered chan<- dbcsv.Row, rows <-chan dbcsv.Row) {
			defer close(filtered)
			for row := range rows {
				for _, s := range row.Values {
					if s != "" {
						filtered <- row
						break
					}
				}
			}
		}(filtered, rows)
		rows = filtered
	}

	columns, err := cfg.Columns()
	if err != nil {
		log.Fatal(err)
	}
	if len(columns) > 0 {
		// change column order
		filtered := make(chan dbcsv.Row, 8)
		go func(filtered chan<- dbcsv.Row, rows <-chan dbcsv.Row) {
			defer close(filtered)
			for row := range rows {
				row2 := dbcsv.Row{Line: row.Line, Values: make([]string, len(columns))}
				for i, j := range columns {
					if j < len(row.Values) {
						row2.Values[i] = row.Values[j]
					} else {
						row2.Values[i] = ""
					}
				}
				filtered <- row2
			}
		}(filtered, rows)
		rows = filtered
	}

	dsn := os.ExpandEnv(*flagConnect)
	db, err := sql.Open("goracle", dsn)
	if err != nil {
		log.Fatal(errors.Wrap(err, dsn))
	}
	defer db.Close()

	var n int
	start := time.Now()
	n, err = dbExec(db, *flagFunc, fixParams, int64(*flagFuncRetOk), rows, *flagOneTx)
	bw.Flush()
	if err != nil {
		log.Fatalf("exec %q: %v", *flagFunc, err)
	}
	d := time.Since(start)
	close(errch)
	if len(errs) > 0 {
		log.Fatal("ERRORS:\n\t%s", strings.Join(errs, "\n\t"))
	}
	log.Printf("Processed %d rows in %s.", n, d)
}

// vim: set fileencoding=utf-8 noet:
