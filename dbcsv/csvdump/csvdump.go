/*
   Copyright 2017 Tamás Gulácsi

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package main in csvdump represents a cursor->csv dumper
package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"

	"gopkg.in/goracle.v2"

	errors "golang.org/x/xerrors"
)

var envEnc = namedEncoding{Encoding: encoding.Nop, Name: "utf-8"}

type namedEncoding struct {
	encoding.Encoding
	Name string
}

func main() {
	if e := os.Getenv("LANG"); e != "" {
		if i := strings.LastIndexByte(e, '.'); i >= 0 {
			e = e[i+1:]
		}
		if enc, err := encFromName(e); err != nil {
			log.Println(err)
		} else {
			envEnc = enc
		}
	}
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func Main() error {
	flagConnect := flag.String("connect", os.Getenv("BRUNO_ID"), "user/passw@sid to connect to")
	flagDateFormat := flag.String("date", dateFormat, "date format, in Go notation")
	flagSep := flag.String("sep", ";", "separator")
	flagHeader := flag.Bool("header", true, "print header")
	flagEnc := flag.String("encoding", envEnc.Name, "encoding to use for output")
	flagOut := flag.String("o", "-", "output (defaults to stdout)")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagCall := flag.Bool("call", false, "the first argument is not the WHERE, but the PL/SQL block to be called, the followings are not the columns but the arguments")

	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), strings.Replace(`Usage of {{.prog}}:
	{{.prog}} [options] 'T_able' 'F_ield=1'

will execute a "SELECT * FROM T_able WHERE F_ield=1" and dump all the columns;

	{{.prog}} -call [options] 'DB_lista.csv' 'p_a=1' 'p_b=c'

will execute "BEGIN :1 := DB_lista.csv(p_a=>:2, p_b=>3); END" with p_a=1, p_b=c
and dump all the columns of the cursor returned by the function.

`, "{{.prog}}", os.Args[0], -1))
		flag.PrintDefaults()
	}
	flag.Parse()

	var Log func(...interface{}) error
	if *flagVerbose {
		Log = func(keyvals ...interface{}) error {
			if len(keyvals)%2 != 0 {
				keyvals = append(keyvals, "")
			}
			vv := make([]interface{}, len(keyvals)/2)
			for i := range vv {
				v := fmt.Sprintf("%+v", keyvals[(i<<1)+1])
				if strings.Contains(v, " ") {
					v = `"` + v + `"`
				}
				vv[i] = fmt.Sprintf("%s=%s", keyvals[(i<<1)], v)
			}
			log.Println(vv...)
			return nil
		}
	}

	enc, err := encFromName(*flagEnc)
	if err != nil {
		return err
	}
	dateFormat = *flagDateFormat
	dEnd = `"` + strings.NewReplacer(
		"2006", "9999",
		"01", "12",
		"02", "31",
		"15", "23",
		"04", "59",
		"05", "59",
	).Replace(dateFormat) + `"`

	var qry string
	var params []interface{}
	if *flagCall {
		var buf strings.Builder
		fmt.Fprintf(&buf, `BEGIN :1 := %s(`, flag.Arg(0))
		params = make([]interface{}, flag.NArg()-1)
		for i, x := range flag.Args()[1:] {
			arg := strings.SplitN(x, "=", 2)
			params[i] = ""
			if len(arg) > 1 {
				params[i] = arg[1]
			}
			if i != 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%s=>:%d", arg[0], i+2)
		}
		buf.WriteString("); END;")
		qry = buf.String()
		if Log != nil {
			Log("call", qry, "params", params)
		}
	} else {
		var (
			where   string
			columns []string
		)
		if flag.NArg() > 1 {
			where = flag.Arg(1)
			if flag.NArg() > 2 {
				columns = flag.Args()[2:]
			}
		}
		qry = getQuery(flag.Arg(0), where, columns, envEnc)
	}
	db, err := sql.Open("goracle", *flagConnect)
	if err != nil {
		return errors.Errorf("%s: %w", *flagConnect, err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		log.Printf("[WARN] Read-Only transaction: %v", err)
		if tx, err = db.BeginTx(ctx, nil); err != nil {
			return errors.Errorf("%s: %w", "beginTx", err)
		}
	}
	defer tx.Rollback()
	if Log != nil {
		Log("env_encoding", envEnc.Name)
	}

	fh := os.Stdout
	if !(*flagOut == "" || *flagOut == "-") {
		os.MkdirAll(filepath.Dir(*flagOut), 0775)
		if fh, err = os.Create(*flagOut); err != nil {
			return errors.Errorf("%s: %w", *flagOut, err)
		}
	}
	defer fh.Close()

	if Log != nil {
		Log("msg", "writing", "file", fh.Name(), "encoding", enc)
	}
	w := io.Writer(encoding.ReplaceUnsupported(enc.NewEncoder()).Writer(fh))
	if *flagCall {
		err = dumpCall(ctx, w, tx, qry, params, *flagHeader, *flagSep, Log)
	} else {
		err = dumpQry(ctx, w, tx, qry, *flagHeader, *flagSep, Log)
	}
	cancel()
	if err != nil {
		return errors.Errorf("dump: %w", err)
	}
	return fh.Close()
}

func getQuery(table, where string, columns []string, enc encoding.Encoding) string {
	if table == "" && where == "" && len(columns) == 0 {
		if enc == nil {
			enc = encoding.Nop
		}
		b, err := ioutil.ReadAll(enc.NewDecoder().Reader(os.Stdin))
		if err != nil {
			panic(err)
		}
		return string(b)
	}
	table = strings.TrimSpace(table)
	if len(table) > 6 && strings.HasPrefix(strings.ToUpper(table), "SELECT ") {
		return table
	}
	cols := "*"
	if len(columns) > 0 {
		cols = strings.Join(columns, ", ")
	}
	if where == "" {
		return "SELECT " + cols + " FROM " + table //nolint:gas
	}
	return "SELECT " + cols + " FROM " + table + " WHERE " + where //nolint:gas
}

type queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

type execer interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

func dumpCall(ctx context.Context, w io.Writer, db execer, qry string, params []interface{}, header bool, sep string, Log func(...interface{}) error) error {
	var rows driver.Rows
	params = append(append(make([]interface{}, 0, 1+len(params)),
		sql.Out{Dest: &rows}), params...)
	if _, err := db.ExecContext(ctx, qry, params...); err != nil {
		return errors.Errorf("executing %q: %w", qry, err)
	}
	defer rows.Close()

	columns, err := getColumns(rows)
	if err != nil {
		return err
	}

	sepB := []byte(sep)
	dest := make([]driver.Value, len(columns))
	bw := bufio.NewWriterSize(w, 65536)
	defer bw.Flush()
	values := make([]stringer, len(columns))
	for i, col := range columns {
		c := col.Converter(sep)
		values[i] = c
	}
	if header {
		for i, col := range columns {
			if i > 0 {
				bw.Write(sepB)
			}
			csvQuote(bw, sep, col.Name)
		}
		bw.Write([]byte{'\n'})
	}

	start := time.Now()
	n := 0
	for {
		if nextErr := rows.Next(dest); nextErr != nil {
			if nextErr == io.EOF {
				break
			}
			return errors.Errorf("Next: %w", nextErr)
		}
		for i, data := range dest {
			if i > 0 {
				bw.Write(sepB)
			}
			if data == nil {
				continue
			}
			if err = values[i].Scan(data); err != nil {
				return errors.Errorf("scan %q into col %d: %w", data, i, err)
			}
			bw.WriteString(values[i].String())
		}
		bw.Write([]byte{'\n'})
		n++
	}
	dur := time.Since(start)
	if Log != nil {
		Log("msg", "dump finished", "rows", n, "dur", dur, "speed", float64(n)/float64(dur)*float64(time.Second))
	}
	if err != nil {
		return errors.Errorf("fetching rows: %w", err)
	}
	return nil
}

func dumpQry(ctx context.Context, w io.Writer, db queryer, qry string, header bool, sep string, Log func(...interface{}) error) error {
	rows, err := db.QueryContext(ctx, qry, goracle.FetchRowCount(1024))
	if err != nil {
		return errors.Errorf("executing %q: %w", qry, err)
	}
	defer rows.Close()
	//log.Printf("%q: columns: %#v", qry, columns)

	columns, err := getColumns(rows)
	if err != nil {
		return err
	}

	sepB := []byte(sep)
	dest := make([]interface{}, len(columns))
	bw := bufio.NewWriterSize(w, 65536)
	defer bw.Flush()
	values := make([]stringer, len(columns))
	for i, col := range columns {
		c := col.Converter(sep)
		values[i] = c
		dest[i] = c.Pointer()
	}
	if header {
		for i, col := range columns {
			if i > 0 {
				bw.Write(sepB)
			}
			csvQuote(bw, sep, col.Name)
		}
		bw.Write([]byte{'\n'})
	}

	start := time.Now()
	n := 0
	for rows.Next() {
		if err = rows.Scan(dest...); err != nil {
			return errors.Errorf("scan into %#v: %w", dest, err)
		}
		for i, data := range dest {
			if i > 0 {
				bw.Write(sepB)
			}
			if data == nil {
				continue
			}
			bw.WriteString(values[i].String())
		}
		bw.Write([]byte{'\n'})
		n++
	}
	err = rows.Err()
	dur := time.Since(start)
	if Log != nil {
		Log("msg", "dump finished", "rows", n, "dur", dur, "speed", float64(n)/float64(dur)*float64(time.Second), "error", err)
	}
	if err != nil {
		return errors.Errorf("fetching rows: %w", err)
	}
	return nil
}

type Column struct {
	Name string
	reflect.Type
}

func (col Column) Converter(sep string) stringer {
	return getColConverter(col.Type, sep)
}

type stringer interface {
	String() string
	Pointer() interface{}
	Scan(interface{}) error
}

type ValString struct {
	Sep   string
	Value sql.NullString
}

func (v ValString) String() string            { return csvQuoteString(v.Sep, v.Value.String) }
func (v *ValString) Pointer() interface{}     { return &v.Value }
func (v *ValString) Scan(x interface{}) error { return v.Value.Scan(x) }

type ValInt struct {
	Value sql.NullInt64
}

func (v ValInt) String() string {
	if v.Value.Valid {
		return strconv.FormatInt(v.Value.Int64, 10)
	}
	return ""
}
func (v *ValInt) Pointer() interface{}     { return &v.Value }
func (v *ValInt) Scan(x interface{}) error { return v.Value.Scan(x) }

type ValFloat struct {
	Value sql.NullFloat64
}

func (v ValFloat) String() string {
	if v.Value.Valid {
		return strconv.FormatFloat(v.Value.Float64, 'f', -1, 64)
	}
	return ""
}
func (v *ValFloat) Pointer() interface{}     { return &v.Value }
func (v *ValFloat) Scan(x interface{}) error { return v.Value.Scan(x) }

type ValTime struct {
	Value time.Time
	Quote bool
}

var (
	dEnd       string
	dateFormat = "2006-01-02"
)

func (v ValTime) String() string {
	if v.Value.IsZero() {
		return ""
	}
	if v.Value.Year() < 0 {
		return dEnd
	}
	if v.Quote {
		return `"` + v.Value.Format(dateFormat) + `"`
	}
	return v.Value.Format(dateFormat)
}
func (vt ValTime) ConvertValue(v interface{}) (driver.Value, error) {
	if v == nil {
		return time.Time{}, nil
	}
	t, _ := v.(time.Time)
	return t, nil
}
func (vt *ValTime) Scan(v interface{}) error {
	if v == nil {
		vt.Value = time.Time{}
		return nil
	}
	t, _ := v.(time.Time)
	vt.Value = t
	return nil
}
func (v *ValTime) Pointer() interface{} { return v }

func getColConverter(typ reflect.Type, sep string) stringer {
	switch typ.Kind() {
	case reflect.String:
		return &ValString{Sep: sep}
	case reflect.Float32, reflect.Float64:
		return &ValFloat{}
	case reflect.Int32, reflect.Int64, reflect.Int:
		return &ValInt{}
	}
	switch typ {
	case reflect.TypeOf(time.Time{}):
		return &ValTime{Quote: strings.Contains(dateFormat, sep)}
	}
	return &ValString{Sep: sep}
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 1024)) }}

func csvQuoteString(sep, s string) string {
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	csvQuote(buf, sep, s)
	return buf.String()
}

func csvQuote(w io.Writer, sep, s string) (int, error) {
	needQuote := strings.Contains(s, sep) || strings.ContainsAny(s, `"`+"\n")
	if !needQuote {
		return io.WriteString(w, s)
	}
	n, err := w.Write([]byte{'"'})
	if err != nil {
		return n, err
	}
	m, err := io.WriteString(w, strings.Replace(s, `"`, `""`, -1))
	n += m
	if err != nil {
		return n, err
	}
	m, err = w.Write([]byte{'"'})
	return n + m, err
}

func encFromName(e string) (namedEncoding, error) {
	switch strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(e)) {
	case "", "utf8":
		return namedEncoding{Encoding: encoding.Nop, Name: "utf-8"}, nil
	case "iso88591":
		return namedEncoding{Encoding: charmap.ISO8859_1, Name: "iso-8859-1"}, nil
	case "iso88592":
		return namedEncoding{Encoding: charmap.ISO8859_2, Name: "iso-8859-2"}, nil
	default:
		return namedEncoding{Encoding: encoding.Nop, Name: e}, errors.Errorf("%s: %w", e, errors.New("unknown encoding"))
	}
}

func getColumns(rows interface{}) ([]Column, error) {
	if r, ok := rows.(*sql.Rows); ok {
		types, err := r.ColumnTypes()
		if err != nil {
			return nil, err
		}
		cols := make([]Column, len(types))
		for i, t := range types {
			cols[i] = Column{Name: t.Name(), Type: t.ScanType()}
		}
		return cols, nil
	}

	colNames := rows.(driver.Rows).Columns()
	cols := make([]Column, len(colNames))
	r := rows.(driver.RowsColumnTypeScanType)
	for i, name := range colNames {
		cols[i] = Column{
			Name: name,
			Type: r.ColumnTypeScanType(i),
		}
	}
	return cols, nil
}

// vim: se noet fileencoding=utf-8:
