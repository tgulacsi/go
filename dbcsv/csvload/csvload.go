// Copyright 2017 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	_ "gopkg.in/rana/ora.v4"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	encName := os.Getenv("LANG")
	if i := strings.IndexByte(encName, '.'); i >= 0 {
		encName = encName[i+1:]
	} else if encName == "" {
		encName = "UTF-8"
	}

	flagDB := flag.String("dsn", "$BRUNO_ID", "database to connect to")
	flagEnc := flag.String("encoding", encName, "input encoding")
	flagTruncate := flag.Bool("truncate", false, "truncate table")
	flagTablespace := flag.String("tablespace", "DATA", "tablespace to create table in")
	flagSep := flag.String("sep", ";", "CSV separator")
	flagConcurrency := flag.Int("concurrency", 8, "concurrency")
	flag.Parse()

	enc, err := htmlindex.Get(*flagEnc)
	if err != nil {
		return errors.Wrap(err, *flagEnc)
	}
	if strings.HasPrefix(*flagDB, "$") {
		*flagDB = os.ExpandEnv(*flagDB)
	}
	db, err := sql.Open("ora", *flagDB)
	if err != nil {
		return errors.Wrap(err, *flagDB)
	}
	defer db.Close()

	tbl := strings.ToUpper(flag.Arg(0))
	src := flag.Arg(1)
	if src == "" {
		src = "-"
	}
	in := os.Stdin
	if src != "-" {
		if in, err = os.Open(src); err != nil {
			return errors.Wrap(err, src)
		}
	}
	defer in.Close()

	rows, _ := getReader(in, *flagSep, enc)
	cols, err := CreateTable(db, tbl, rows, *flagTruncate, *flagTablespace)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `INSERT INTO "%s" (`, tbl)
	for i, c := range cols {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(c.Name)
	}
	buf.WriteString(") VALUES (")
	for i, _ := range cols {
		if i != 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, ":%d", i+1)
	}
	buf.WriteString(")")
	qry := buf.String()
	log.Println(qry)

	start := time.Now()

	rdr, err := getReader(in, *flagSep, enc)
	if err != nil {
		return errors.Wrap(err, src)
	}

	rowsCh := make(chan []string, *flagConcurrency)

	var grp errgroup.Group
	for i := 0; i < *flagConcurrency; i++ {
		grp.Go(func() error {
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			stmt, err := tx.Prepare(qry)
			if err != nil {
				return errors.Wrap(err, qry)
			}
			var rowI []interface{}

			for row := range rowsCh {
				rowI = rowI[:0]
				for _, v := range row {
					rowI = append(rowI, v)
				}
				if _, err := stmt.Exec(rowI...); err != nil {
					return errors.Wrapf(err, "%s, %q", qry, row)
				}
			}
			return tx.Commit()
		})
	}

	var n int64
	for {
		row, err := rdr.Read()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		n++

		if n == 1 {
			continue
		}
		rowsCh <- row
	}
	close(rowsCh)

	err = grp.Wait()
	dur := time.Since(start)
	log.Printf("Imported %d rows from %q to %q in %s.", n, src, tbl, dur)
	return err
}

func getReader(f io.ReadSeeker, sep string, enc encoding.Encoding) (*csv.Reader, error) {
	p, err := f.Seek(0, io.SeekCurrent)
	if err == nil && p != 0 {
		_, err = f.Seek(0, io.SeekStart)
	}
	r := io.Reader(f)
	if enc != nil {
		r = enc.NewDecoder().Reader(f)
	}
	rdr := csv.NewReader(r)
	rdr.LazyQuotes = true
	if sep != "" {
		rdr.Comma = ([]rune(sep))[0]
	}
	return rdr, err
}

func typeOf(s string) Type {
	if len(s) == 0 {
		return Unknown
	}
	var hasNonDigit bool
	var dotCount int
	strings.Map(func(r rune) rune {
		if r == '.' {
			dotCount++
		} else {
			hasNonDigit = hasNonDigit || !('0' <= r && r <= '9')
		}
		return -1
	},
		strings.TrimSpace(s))

	if !hasNonDigit {
		if dotCount == 1 {
			return Float
		}
		if dotCount == 0 {
			return Int
		}
	}
	return String
}

func CreateTable(db *sql.DB, tbl string, rows *csv.Reader, truncate bool, tablespace string) ([]Column, error) {
	tbl = strings.ToUpper(tbl)
	qry := "SELECT COUNT(0) FROM cat WHERE UPPER(table_name) = :1"
	var n int64
	var cols []Column
	if err := db.QueryRow(qry, tbl).Scan(&n); err != nil {
		return cols, errors.Wrap(err, qry)
	}
	if n > 0 && truncate {
		qry = `TRUNCATE TABLE "` + tbl + `"`
		if _, err := db.Exec(qry); err != nil {
			return cols, errors.Wrap(err, qry)
		}
	}

	if n == 0 {
		for {
			row, err := rows.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				return cols, err
			}

			if cols == nil {
				cols = make([]Column, len(row))
				for i, v := range row {
					if len(v) > 30 {
						v = fmt.Sprintf("%s_%02d", v[:27], i)
					}
					cols[i].Name = v
				}
				continue
			}
			for i, v := range row {
				if len(v) > cols[i].Length {
					cols[i].Length = len(v)
				}
				if cols[i].Type == Unknown {
					cols[i].Type = typeOf(v)
				}
			}
		}
		var buf bytes.Buffer
		buf.WriteString(`CREATE TABLE "` + tbl + `" (`)
		for i, c := range cols {
			if i != 0 {
				buf.WriteString(",\n")
			}
			fmt.Fprintf(&buf, "  %s %s(%s)", c.Name, c.Type.String(), c.Length)
		}
		buf.WriteString("\n)")
		if tablespace != "" {
			buf.WriteString(" TABLESPACE ")
			buf.WriteString(tablespace)
		}
		qry = buf.String()
		if _, err := db.Exec(qry); err != nil {
			return cols, errors.Wrap(err, qry)
		}
		cols = cols[:0]
	}

	qry = `SELECT column_name, data_type, NVL(data_length, 0), NVL(data_precision, 0), NVL(data_scale, 0), nullable
  FROM user_tab_cols WHERE table_name = :1
  ORDER BY column_id`
	tRows, err := db.Query(qry, tbl)
	if err != nil {
		return cols, errors.Wrap(err, qry)
	}
	defer tRows.Close()
	for tRows.Next() {
		var c Column
		var nullable string
		if err = tRows.Scan(&c.Name, &c.DataType, &c.Length, &c.Precision, &c.Scale, &nullable); err != nil {
			return cols, err
		}
		c.Nullable = nullable != "N"
		cols = append(cols, c)
	}
	return cols, nil
}

type Column struct {
	Length           int
	Name             string
	Type             Type
	DataType         string
	Precision, Scale int
	Nullable         bool
}
type Type uint8

const (
	Unknown = Type(0)
	String  = Type(1)
	Int     = Type(2)
	Float   = Type(3)
)

func (t Type) String() string {
	switch t {
	case Int, Float:
		return "NUMBER"
	default:
		return "VARCHAR2"
	}
}
