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

	_ "gopkg.in/goracle.v2"

	"github.com/pkg/errors"
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
	db, err := sql.Open("goracle", *flagConnect)
	if err != nil {
		return errors.Wrap(err, *flagConnect)
	}
	defer db.Close()
	if Log != nil {
		Log("env_encoding", envEnc.Name)
	}
	qry := getQuery(flag.Arg(0), where, columns, envEnc)

	fh := os.Stdout
	if !(*flagOut == "" || *flagOut == "-") {
		os.MkdirAll(filepath.Dir(*flagOut), 0775)
		if fh, err = os.Create(*flagOut); err != nil {
			return errors.Wrap(err, *flagOut)
		}
	}
	defer fh.Close()

	if Log != nil {
		Log("msg", "writing", "file", fh.Name(), "encoding", enc)
	}
	w := io.Writer(encoding.ReplaceUnsupported(enc.NewEncoder()).Writer(fh))
	err = dump(w, db, qry, *flagHeader, *flagSep, Log)
	_ = db.Close()
	if err != nil {
		return errors.Wrap(err, "dump")
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
		return "SELECT " + cols + " FROM " + table
	}
	return "SELECT " + cols + " FROM " + table + " WHERE " + where
}

type queryer interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

func dump(w io.Writer, db queryer, qry string, header bool, sep string, Log func(...interface{}) error) error {
	rows, err := db.Query(qry)
	if err != nil {
		return errors.Wrapf(err, "executing %q", qry)
	}
	defer rows.Close()
	//log.Printf("%q: columns: %#v", qry, columns)

	columns, err := getColumns(rows)

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
			return errors.Wrapf(err, "scan into %#v", dest)
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
		return errors.Wrapf(err, "fetching rows")
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
}

type ValString struct {
	Sep   string
	Value sql.NullString
}

func (v ValString) String() string        { return csvQuoteString(v.Sep, v.Value.String) }
func (v *ValString) Pointer() interface{} { return &v.Value }

type ValInt struct {
	Value sql.NullInt64
}

func (v ValInt) String() string {
	if v.Value.Valid {
		return strconv.FormatInt(v.Value.Int64, 10)
	}
	return ""
}
func (v *ValInt) Pointer() interface{} { return &v.Value }

type ValFloat struct {
	Value sql.NullFloat64
}

func (v ValFloat) String() string {
	if v.Value.Valid {
		return strconv.FormatFloat(v.Value.Float64, 'f', -1, 64)
	}
	return ""
}
func (v *ValFloat) Pointer() interface{} { return &v.Value }

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
	hasSep := strings.Contains(s, sep)
	hasQ := strings.Contains(s, `"`)
	var n int
	if !(hasSep || hasQ) {
		return io.WriteString(w, s)
	}
	var err error
	if n, err = w.Write([]byte{'"'}); err != nil {
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
		return namedEncoding{Encoding: encoding.Nop, Name: e}, errors.Wrap(errors.New("unknown encoding"), e)
	}
}

func getColumns(rows *sql.Rows) ([]Column, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	cols := make([]Column, len(types))
	for i, t := range types {
		cols[i] = Column{Name: t.Name(), Type: t.ScanType()}
	}
	return cols, nil
}

// vim: se noet fileencoding=utf-8:
