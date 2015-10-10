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
	"strings"
	"sync"

	"github.com/tgulacsi/go/orahlp"
	"github.com/tgulacsi/go/text"

	"github.com/tealeg/xlsx"
	"gopkg.in/errgo.v1"
	"gopkg.in/rana/ora.v3"
)

func main() {
	flagSheet := flag.Int("sheet", 0, "Index of sheet to convert, zero based")
	flagConnect := flag.String("connect", "$BRUNO_ID", "database connection string")
	flagFunc := flag.String("code", "DBMS_OUTPUT.PUT_LINE", "function name to be called with each line")
	flagDelim := flag.String("d", ";", "Delimiter to use between fields")
	flagCharset := flag.String("charset", "utf-8", "input charset")
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

	if err = dbExec(ses, *flagFunc, rows); err != nil {
		log.Fatal("exec %q: %v", *flagFunc, err)
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
	n := 0
	for _, row := range sheet.Rows {
		if row != nil {
			vals = vals[:0]
			for _, cell := range row.Cells {
				vals = append(vals, cell.String())
			}
			rows <- Row{Line: n, Values: vals}
			n++
		}
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

func dbExec(ses *ora.Ses, fun string, rows <-chan Row) error {
	qry, err := getQuery(ses, fun)
	if err != nil {
		return err
	}

	var (
		tx     *ora.Tx
		st     *ora.Stmt
		values []interface{}
	)

	defer st.Close()
	for row := range rows {
		if tx == nil {
			if tx, err = ses.StartTx(); err != nil {
				return err
			}
			if st, err = ses.Prep(qry); err != nil {
				st.Close()
				tx.Rollback()
				return err
			}
		}

		values = values[:0]
		for _, s := range row.Values {
			values = append(values, s)
		}
		if _, err = st.Exe(values...); err != nil {
			log.Printf("execute %q with row %d (%#v): %v", qry, row.Line, row.Values, err)
			return err
		}
	}
	if tx != nil {
		st.Close()
		return tx.Commit()
	}
	return nil
}

func getQuery(ses *ora.Ses, fun string) (string, error) {
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
		return "", errgo.Newf("bad function name: %q", fun)
	}
	rset, err := ses.PrepAndQry(qry, fun)
	if err != nil {
		return "", errgo.Notef(err, qry)
	}

	type Arg struct {
		Name, Type, InOut        string
		Length, Precision, Scale int
	}
	args := make([]Arg, 0, 32)
	for rset.Next() {
		args = append(args,
			Arg{Name: rset.Row[0].(string), Type: rset.Row[1].(string), InOut: rset.Row[2].(string),
				Length: int(rset.Row[3].(int64)), Precision: int(rset.Row[4].(int64)), Scale: int(rset.Row[5].(int64))})
	}
	if rset.Err != nil {
		return "", errgo.Notef(rset.Err, qry)
	}

	qry = "BEGIN "
	i := 0
	if args[0].Name == "" { // function
		qry += ":1 := "
		args = args[1:]
		i++
	}
	qry += fun + "("
	vals := make([]string, 0, len(args))
	for _, arg := range args {
		vals = append(vals, fmt.Sprintf("%s=>:%d", arg.Name, i))
		i++
	}
	qry += "); END;"
	return qry, nil
}
