// Copyright 2011-2015, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"gopkg.in/rana/ora.v4"
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

	flagSheet := flag.Int("sheet", 0, "Index of sheet to convert, zero based")
	flagConnect := flag.String("connect", "$BRUNO_ID", "database connection string")
	flagFunc := flag.String("call", "DBMS_OUTPUT.PUT_LINE", "function name to be called with each line")
	flagFixParams := flag.String("fix", "p_file_name=>{{.FileName}}", "fix parameters to add; uses text/template")
	flagFuncRetOk := flag.Int("call-ret-ok", 0, "OK return value")
	flagOneTx := flag.Bool("one-tx", true, "one transaction, or commit after each row")
	flagDelim := flag.String("d", "", "Delimiter to use between fields")
	flagCharset := flag.String("charset", "utf-8", "input charset")
	flagSkip := flag.Int("skip", 1, "skip first N rows")
	flagColumns := flag.String("columns", "", "column numbers to use, separated by comma, in param order, starts with 1")
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

	var columns []int
	if *flagColumns != "" {
		for _, x := range strings.Split(*flagColumns, ",") {
			i, err := strconv.Atoi(x)
			if err != nil {
				log.Fatalf("column %q: %v", x, err)
			}
			columns = append(columns, i-1)
		}
	}

	ctxData := struct {
		FileName string
	}{FileName: flag.Arg(0)}
	var fixParams [][2]string
	var buf bytes.Buffer
	for _, tup := range strings.Split(*flagFixParams, ",") {
		parts := strings.SplitN(tup, "=>", 2)
		tpl := template.Must(template.New(parts[0]).Parse(parts[1]))
		buf.Reset()
		if err := tpl.Execute(&buf, ctxData); err != nil {
			log.Fatal(err)
		}
		fixParams = append(fixParams, [2]string{parts[0], buf.String()})
	}

	inp := os.Stdin
	if flag.Arg(0) != "" && flag.Arg(0) != "-" {
		var err error
		if inp, err = os.Open(flag.Arg(0)); err != nil {
			log.Fatal("open %q: %v", flag.Arg(0), err)
		}
	}
	defer inp.Close()

	rows := make(chan Row, 8)
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

	// detect file type
	var (
		b       [4]byte
		R       func(rows chan<- Row, inp io.Reader, inpfn string) error
		rdrName string
	)
	if _, err := io.ReadFull(inp, b[:]); err != nil {
		log.Fatal("read %q: %v", inp, err)
	}
	if bytes.Equal(b[:], []byte{0xd0, 0xcf, 0x11, 0xe0}) { // OLE2
		rdrName = "xls"
	} else if bytes.Equal(b[:], []byte{0x50, 0x4b, 0x03, 0x04}) { //PKZip, so xlsx
		rdrName = "xlsx"
	} else if bytes.Equal(b[:1], []byte{'"'}) { // CSV
		rdrName = "csv"
	} else {
		switch filepath.Ext(inp.Name()) {
		case ".xls":
			rdrName = "xls"
		case ".xlsx":
			rdrName = "xlsx"
		default:
			rdrName = "csv"
		}
	}
	log.Printf("File starts with %q (% x), so using %s reader.", b, b, rdrName)

	switch rdrName {
	case "xls":
		R = func(rows chan<- Row, _ io.Reader, fn string) error {
			return readXLSFile(rows, fn, *flagCharset, *flagSheet)
		}
	case "xlsx":
		R = func(rows chan<- Row, _ io.Reader, fn string) error {
			return readXLSXFile(rows, fn, *flagSheet)
		}
	default:
		enc, err := htmlindex.Get(*flagCharset)
		if err != nil {
			log.Fatalf("Get encoding for name %q: %v", *flagCharset, err)
		}
		R = func(rows chan<- Row, inp io.Reader, _ string) error {
			r := transform.NewReader(
				io.MultiReader(bytes.NewReader(b[:]), inp),
				enc.NewDecoder())
			return readCSV(rows, r, *flagDelim)
		}
	}
	go func(rows chan<- Row) {
		defer close(rows)
		errch <- R(rows, inp, flag.Arg(0))
	}(rows)

	// filter out empty rows
	{
		filtered := make(chan Row, 8)
		go func(filtered chan<- Row, rows <-chan Row) {
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

	for i := 0; i < *flagSkip; i++ {
		<-rows
	}
	if len(columns) > 0 {
		// change column order
		filtered := make(chan Row, 8)
		go func(filtered chan<- Row, rows <-chan Row) {
			defer close(filtered)
			for row := range rows {
				row2 := Row{Line: row.Line, Values: make([]string, len(columns))}
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
	sesPool, err := ora.NewPool(dsn, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer sesPool.Close()
	ses, err := sesPool.Get()
	if err != nil {
		log.Fatal(err)
	}
	defer sesPool.Put(ses)

	var n int
	start := time.Now()
	n, err = dbExec(ses, *flagFunc, fixParams, int64(*flagFuncRetOk), rows, *flagOneTx)
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
