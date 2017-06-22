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
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tgulacsi/go/dber"
	"github.com/tgulacsi/go/orahlp"
	_ "gopkg.in/rana/ora.v4"

	"github.com/pkg/errors"
)

func getQuery(table, where string, columns []string) string {
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

func dump(w io.Writer, db dber.DBer, qry string, header bool, sep string) error {
	columns, err := GetColumns(db, qry)
	if err != nil {
		return err
	}
	rows, err := db.Query(qry)
	if err != nil {
		return errors.Wrapf(err, "executing %q", qry)
	}
	defer rows.Close()
	//log.Printf("columns: %#v", columns)

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
	log.Printf("written %d rows.", n)
	if err != nil {
		return errors.Wrapf(err, "fetching rows")
	}
	return nil
}

type Column struct {
	orahlp.Column
}

func (col Column) Converter(sep string) stringer {
	return getColConverter(col.Column, sep)
}

func GetColumns(db dber.Execer, qry string) (cols []Column, err error) {
	desc, err := orahlp.DescribeQuery(db, qry)
	if err != nil {
		return nil, errors.Wrapf(err, "Describe %q", qry)
	}
	cols = make([]Column, len(desc))
	for i, col := range desc {
		cols[i].Column = col
	}
	return cols, nil
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
}

var (
	dEnd       string
	dateFormat = "2006-01-02"
)

func (v ValTime) String() string {
	if v.Value.Year() < 0 {
		return dEnd
	}
	return `"` + v.Value.Format(dateFormat) + `"`
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

func getColConverter(col orahlp.Column, sep string) stringer {
	switch col.Type {
	case 2:
		if col.Scale == 0 {
			return &ValInt{}
		}
		return &ValFloat{}
	case 12:
		return &ValTime{}
	default:
		return &ValString{Sep: sep}
	}
}

func main() {
	var (
		where   string
		columns []string
	)

	flagConnect := flag.String("connect", os.Getenv("BRUNO_ID"), "user/passw@sid to connect to")
	flagDateFormat := flag.String("date", dateFormat, "date format, in Go notation")
	flagSep := flag.String("sep", ";", "separator")
	flagHeader := flag.Bool("header", true, "print header")
	flag.Parse()
	dateFormat = *flagDateFormat
	dEnd = `"` + strings.NewReplacer(
		"2006", "9999",
		"01", "12",
		"02", "31",
		"15", "23",
		"04", "59",
		"05", "59",
	).Replace(dateFormat) + `"`
	if flag.NArg() > 1 {
		where = flag.Arg(1)
		if flag.NArg() > 2 {
			columns = flag.Args()[2:]
		}
	}
	//ora.Log = lg15.Log
	//lg15.Log.SetHandler(log15.StderrHandler)
	db, err := sql.Open("ora", *flagConnect)
	if err != nil {
		log.Printf("error connecting to %s: %v", *flagConnect, err)
		os.Exit(2)
	}
	qry := getQuery(flag.Arg(0), where, columns)
	err = dump(os.Stdout, dber.SqlDBer{DB: db}, qry, *flagHeader, *flagSep)
	_ = db.Close()
	if err != nil {
		log.Printf("error dumping: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
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

// vim: se noet fileencoding=utf-8:
