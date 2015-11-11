// Copyright 2011-2015, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/tgulacsi/go/orahlp"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"github.com/tealeg/xlsx"
	"gopkg.in/errgo.v1"
	"gopkg.in/rana/ora.v3"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)
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
	flagCommit := flag.Int("commit-each", 0, "commit on every Nth record")
	flagFunc := flag.String("call", "DBMS_OUTPUT.PUT_LINE", "function name to be called with each line")
	flagFixParams := flag.String("fix", "p_file_name=>{{.FileName}}", "fix parameters to add; uses text/template")
	flagFuncRetOk := flag.Int("call-ret-ok", 0, "OK return value")
	flagDelim := flag.String("d", ";", "Delimiter to use between fields")
	flagCharset := flag.String("charset", "utf-8", "input charset")
	flagSkip := flag.Int("skip", 1, "skip first N rows")
	flagColumns := flag.String("columns", "", "column numbers to use, separated by comma, in param order, starts with 1")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s

	The specified code will be called with the cells as (string) arguments
	(except dates, where DATE will be provided), for each row.

Usage:
	%s [flags] <xlsx-or-csv-to-be-read>
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
	var b [4]byte
	if _, err := io.ReadFull(inp, b[:]); err != nil {
		log.Fatal("read %q: %v", inp, err)
	}
	if bytes.Equal(b[:], []byte{0x50, 0x4b, 0x03, 0x04}) { //PKZip, so xlsx
		go func(rows chan<- Row) { defer close(rows); errch <- readXLSXFile(rows, flag.Arg(0), *flagSheet) }(rows)
	} else {
		enc, err := htmlindex.Get(*flagCharset)
		if err != nil {
			log.Fatalf("Get encoding for name %q: %v", *flagCharset, err)
		}
		r := transform.NewReader(inp, enc.NewDecoder())
		go func(rows chan<- Row) { defer close(rows); errch <- readCSV(rows, r, *flagDelim) }(rows)
	}

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

	env, err := ora.OpenEnv(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer env.Close()
	dsn := os.ExpandEnv(*flagConnect)
	username, password, sid := orahlp.SplitDSN(dsn)
	srv, err := env.OpenSrv(&ora.SrvCfg{Dblink: sid})
	if err != nil {
		log.Fatal(err)
	}
	defer srv.Close()

	ses, err := srv.OpenSes(&ora.SesCfg{Username: username, Password: password})
	if err != nil {
		log.Fatal(err)
	}
	defer ses.Close()

	var n int
	start := time.Now()
	n, err = dbExec(ses, *flagFunc, fixParams, int64(*flagFuncRetOk), rows, *flagCommit)
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

const (
	dateFormat     = "20060102"
	dateTimeFormat = "20060102150405"
)

var timeReplacer = strings.NewReplacer(
	"yyyy", "2006",
	"yy", "06",
	"dd", "02",
	"d", "2",
	"mmm", "Jan",
	"mmss", "0405",
	"ss", "05",
	"hh", "15",
	"h", "3",
	"mm:", "04:",
	":mm", ":04",
	"mm", "01",
	"am/pm", "pm",
	"m/", "1/",
	".0", ".9999",
)

func readXLSXFile(rows chan<- Row, filename string, sheetIndex int) error {
	xlFile, err := xlsx.OpenFile(filename)
	if err != nil {
		return errgo.Notef(err, "open %q", filename)
	}
	sheetLen := len(xlFile.Sheets)
	switch {
	case sheetLen == 0:
		return errgo.New("This XLSX file contains no sheets.")
	case sheetIndex >= sheetLen:
		return errgo.Newf("No sheet %d available, please select a sheet between 0 and %d\n", sheetIndex, sheetLen-1)
	}
	sheet := xlFile.Sheets[sheetIndex]
	n := 0
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		vals := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			numFmt := cell.GetNumberFormat()
			if strings.Contains(numFmt, "yy") || strings.Contains(numFmt, "mm") || strings.Contains(numFmt, "dd") {
				goFmt := timeReplacer.Replace(numFmt)
				dt, err := time.Parse(goFmt, cell.String())
				if err != nil {
					return errgo.Notef(err, "parse %q as %q (from %q)", cell.String(), goFmt, numFmt)
				}
				vals = append(vals, dt.Format(dateFormat))
			} else {
				vals = append(vals, cell.String())
			}
		}
		rows <- Row{Line: n, Values: vals}
		n++
	}
	return nil
}

func readCSVFile(rows chan<- Row, filename, delim string) error {
	fh, err := os.Open(filename)
	if err != nil {
		return errgo.Notef(err, "open %q", filename)
	}
	defer fh.Close()
	return readCSV(rows, fh, delim)
}

func readCSV(rows chan<- Row, r io.Reader, delim string) error {
	cr := csv.NewReader(r)
	cr.Comma = ([]rune(delim))[0]
	n := 0
	for {
		row, err := cr.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		rows <- Row{Line: n, Values: row}
		n++
	}
	return nil
}

type Row struct {
	Line   int
	Values []string
}
