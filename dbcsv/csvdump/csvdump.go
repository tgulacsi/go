/*
   Package main in csvdump represents a cursor->csv dumper

   Copyright 2013 Tamás Gulácsi

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
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rana/ora"
	"github.com/tgulacsi/go/dber"
	"github.com/tgulacsi/go/orahlp"

	"gopkg.in/errgo.v1"
)

func getQuery(table, where string, columns []string) string {
	if strings.HasPrefix(table, "SELECT ") {
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

func dump(w io.Writer, db dber.DBer, qry string) error {
	columns, err := GetColumns(db, qry)
	if err != nil {
		return err
	}
	rows, err := db.Query(qry)
	if err != nil {
		return errgo.Notef(err, "executing %q", qry)
	}
	defer rows.Close()
	log.Printf("columns: %#v", columns)

	dest := make([]interface{}, len(columns))
	bw := bufio.NewWriterSize(w, 65536)
	defer bw.Flush()
	for i, col := range columns {
		if i > 0 {
			bw.Write([]byte{';'})
		}
		bw.Write([]byte{'"'})
		bw.WriteString(col.Name)
		bw.Write([]byte{'"'})

		dest[i] = col.Converter()
	}
	bw.Write([]byte{'\n'})
	n := 0
	for rows.Next() {
		if err = rows.Scan(dest...); err != nil {
			return errgo.Notef(err, "scan into %#v", dest)
		}
		for i, data := range dest {
			if i > 0 {
				bw.Write([]byte{';'})
			}
			if data == nil {
				continue
			}
			bw.WriteString(data.(stringer).String())
		}
		bw.Write([]byte{'\n'})
		n++
	}
	err = rows.Err()
	log.Printf("written %d rows.", n)
	if err != nil {
		return errgo.Notef(err, "fetching rows")
	}
	return nil
}

type ColConverter func(interface{}) string

type Column struct {
	orahlp.Column
	String ColConverter
}

func (col Column) Converter() stringer {
	return getColConverter(col.Column)
}

func GetColumns(db dber.Execer, qry string) (cols []Column, err error) {
	desc, err := orahlp.DescribeQuery(db, qry)
	if err != nil {
		return nil, errgo.Notef(err, "Describe %q", qry)
	}
	cols = make([]Column, len(desc))
	for i, col := range desc {
		cols[i].Column = col
	}
	return cols, nil
}

type stringer interface {
	String() string
}

type ValString string

func (v ValString) String() string { return string(v) }

type ValInt int64

func (v ValInt) String() string { return strconv.FormatInt(int64(v), 10) }

type ValFloat float64

func (v ValFloat) String() string { return strconv.FormatFloat(float64(v), 'f', -1, 64) }

type ValTime time.Time

func (v ValTime) String() string { return `"` + time.Time(v).Format(time.RFC3339) + `"` }

func getColConverter(col orahlp.Column) stringer {
	switch col.Type {
	case 2:
		if col.Scale == 0 {
			return new(ValInt)
		}
		return new(ValFloat)
	case 12:
		return new(ValTime)
	default:
		return new(ValString)
	}
}

func main() {
	var (
		where   string
		columns []string
	)

	flagConnect := flag.String("connect", os.Getenv("BRUNO_ID"), "user/passw@sid to connect to")
	flag.Parse()
	if flag.NArg() > 1 {
		where = flag.Arg(1)
		if flag.NArg() > 2 {
			columns = flag.Args()[2:]
		}
	}
	_ = ora.GetDrv()
	db, err := sql.Open("ora", *flagConnect)
	if err != nil {
		log.Printf("error connecting to %s: %v", *flagConnect, err)
		os.Exit(2)
	}
	qry := getQuery(flag.Arg(0), where, columns)
	err = dump(os.Stdout, dber.SqlDBer{db}, qry)
	_ = db.Close()
	if err != nil {
		log.Printf("error dumping: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
