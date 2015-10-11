// Copyright 2011-2015, The xlsx2csv Authors.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
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
	"github.com/tgulacsi/go/text"

	"github.com/tealeg/xlsx"
	"gopkg.in/errgo.v1"
	"gopkg.in/rana/ora.v3"
)

func main() {
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

	The specified code will be called with the cells as (string) arguments,
	for each row.

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
	for _, x := range strings.Split(*flagColumns, ",") {
		i, err := strconv.Atoi(x)
		if err != nil {
			log.Fatal(err)
		}
		columns = append(columns, i)
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
		go func() { defer close(rows); errch <- readXLSXFile(rows, flag.Arg(0), *flagSheet) }()
	} else {
		r := text.NewReader(inp, text.GetEncoding(*flagCharset))
		go func() { defer close(rows); errch <- readCSV(rows, r, *flagDelim) }()
	}

	for i := 0; i < *flagSkip; i++ {
		<-rows
	}
	if len(columns) > 0 {
		filtered := make(chan Row, 8)
		go func() {
			defer close(filtered)
			for row := range rows {
				row2 := Row{Line: row.Line, Values: make([]string, len(columns))}
				for i, j := range columns {
					row2.Values[i] = row.Values[j]
				}
				filtered <- row2
			}
		}()
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

	if err = dbExec(ses, *flagFunc, fixParams, int64(*flagFuncRetOk), rows, *flagCommit); err != nil {
		log.Fatalf("exec %q: %v", *flagFunc, err)
	}
}

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
	var vals []string
	for n, row := range sheet.Rows {
		if row == nil {
			continue
		}
		vals = vals[:0]
		for _, cell := range row.Cells {
			vals = append(vals, cell.String())
		}
		rows <- Row{Line: n, Values: vals}
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

func dbExec(ses *ora.Ses, fun string, fixParams [][2]string, retOk int64, rows <-chan Row, commitEach int) error {
	st, err := getQuery(ses, fun, fixParams)
	if err != nil {
		return err
	}
	defer st.Close()
	var (
		tx       *ora.Tx
		values   []interface{}
		startIdx int
		ret      int64
		n        int
	)

	for row := range rows {
		if tx == nil {
			if tx, err = ses.StartTx(); err != nil {
				return err
			}
		}

		values = values[:startIdx]
		for i, s := range row.Values {
			conv := st.Converters[i]
			if conv == nil {
				values = append(values, s)
				continue
			}
			v, err := conv(s)
			if err != nil {
				log.Printf("row=%#v", row)
				return errgo.Notef(err, "convert %q (row %d, col %d)", s, row.Line, i+1)
			}
			values = append(values, v)
		}
		for _, s := range st.FixParams {
			values = append(values, s)
		}
		if _, err = st.Stmt.Exe(values...); err != nil {
			log.Printf("execute %q with row %d (%#v): %v", st.Qry, row.Line, row.Values, err)
			return err
		}
		if st.Returns && values[0] != nil {
			if ret != retOk {
				tx.Rollback()
				return errgo.Newf("function %q returned %v, wanted %v (line %d).", fun, ret, retOk, row.Line)
			}
		}
		n++
		if commitEach > 0 && n%commitEach == 0 {
			if err = tx.Commit(); err != nil {
				return err
			}
			tx = nil
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

type ConvFunc func(string) (interface{}, error)

type Statement struct {
	Qry string
	*ora.Stmt
	Returns    bool
	Converters []ConvFunc
	ParamCount int
	FixParams  []string
}

func getQuery(ses *ora.Ses, fun string, fixParams [][2]string) (Statement, error) {
	var st Statement
	parts := strings.Split(fun, ".")
	qry := "SELECT argument_name, data_type, in_out, data_length, data_precision, data_scale FROM "
	params := make([]interface{}, 0, 3)
	switch len(parts) {
	case 1:
		qry += "user_arguments WHERE object_name = UPPER(:1)"
		params = append(params, fun)
	case 2:
		qry += "user_arguments WHERE package_name = UPPER(:1) AND object_name = UPPER(:2)"
		params = append(params, parts[0], parts[1])
	case 3:
		qry += "all_arguments WHERE owner = UPPER(:1) AND package_name = UPPER(:2) AND object_name = UPPER(:3)"
		params = append(params, parts[0], parts[1], parts[2])
	default:
		return st, errgo.Newf("bad function name: %q", fun)
	}
	qry += " ORDER BY sequence"
	rset, err := ses.PrepAndQry(qry, params...)
	if err != nil {
		return st, errgo.Notef(err, qry)
	}

	type Arg struct {
		Name, Type, InOut        string
		Length, Precision, Scale int
	}
	args := make([]Arg, 0, 32)
	for rset.Next() {
		arg := Arg{Name: rset.Row[0].(string), Type: rset.Row[1].(string), InOut: rset.Row[2].(string)}
		if rset.Row[3] != nil {
			arg.Length = int(rset.Row[3].(float64))
			if rset.Row[4] != nil {
				arg.Precision = int(rset.Row[4].(float64))
				if rset.Row[5] != nil {
					arg.Scale = int(rset.Row[5].(float64))
				}
			}
		}
		args = append(args, arg)
	}
	if rset.Err != nil {
		return st, errgo.Notef(rset.Err, qry)
	}
	if len(args) == 0 {
		return st, errgo.Newf("%q has no arguments!", fun)
	}

	st.Qry = "BEGIN "
	i := 0
	if args[0].Name == "" { // function
		st.Qry += ":1 := "
		args = args[1:]
		st.Returns = true
		i++
	}
	vals := make([]string, 0, len(args))
	st.Converters = make([]ConvFunc, cap(vals))
	for j, arg := range args {
		vals = append(vals, fmt.Sprintf("%s=>:%d", arg.Name, i))
		if arg.Type == "DATE" {
			st.Converters[j] = strToDate
		}
		i++
	}
	for _, p := range fixParams {
		vals = append(vals, fmt.Sprintf("%s=>%d", p[0], i))
		st.FixParams = append(st.FixParams, p[1])
		i++
	}
	st.ParamCount = i
	st.Qry += fun + "(" + strings.Join(vals, ", ") + "); END;"
	st.Stmt, err = ses.Prep(qry)
	return st, err
}

func strToDate(s string) (interface{}, error) {
	if s == "" {
		return nil, nil
	}
	if 8 <= len(s) && len(s) <= 10 {
		return time.Parse("20060102", justNums(s, 8))
	}
	return time.Parse("20060102150405", justNums(s, 14))
}
func justNums(s string, maxLen int) string {
	var i int
	return strings.Map(
		func(r rune) rune {
			if maxLen >= 0 {
				if i > maxLen {
					return -1
				}
				i++
			}
			if '0' <= r && r <= '9' {
				return r
			}
			return -1
		},
		s)
}
