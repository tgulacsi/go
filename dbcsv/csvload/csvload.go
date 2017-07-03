// Copyright 2017 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pkg/errors"
	"github.com/tgulacsi/go/dbcsv"
	"golang.org/x/sync/errgroup"
	ora "gopkg.in/rana/ora.v4"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

var dateFormat = "2006-01-02 15:04:05"

var ForceString bool

func Main() error {
	encName := os.Getenv("LANG")
	if i := strings.IndexByte(encName, '.'); i >= 0 {
		encName = encName[i+1:]
	} else if encName == "" {
		encName = "UTF-8"
	}

	cfg := &dbcsv.Config{}
	flagDB := flag.String("dsn", "$BRUNO_ID", "database to connect to")
	flag.StringVar(&cfg.Charset, "charset", encName, "input charset")
	flagTruncate := flag.Bool("truncate", false, "truncate table")
	flagTablespace := flag.String("tablespace", "DATA", "tablespace to create table in")
	flag.StringVar(&cfg.Delim, "delim", ";", "CSV separator")
	flagConcurrency := flag.Int("concurrency", 8, "concurrency")
	flag.StringVar(&dateFormat, "date", dateFormat, "date format, in Go notation")
	flag.IntVar(&cfg.Skip, "skip", 0, "skip rows")
	flag.IntVar(&cfg.Sheet, "sheet", 0, "sheet of spreadsheet")
	flag.StringVar(&cfg.ColumnsString, "columns", "", "columns, comma separated indexes")
	flag.BoolVar(&ForceString, "force-string", false, "force all columns to be VARCHAR2")
	flag.Parse()

	if flag.NArg() != 2 {
		log.Fatal("Need two args: the table and the source.")
	}

	if strings.HasPrefix(*flagDB, "$") {
		*flagDB = os.ExpandEnv(*flagDB)
	}
	db, err := sql.Open("ora", *flagDB)
	if err != nil {
		return errors.Wrap(err, *flagDB)
	}
	defer db.Close()

	db.SetMaxIdleConns(*flagConcurrency)
	db.SetMaxOpenConns(*flagConcurrency + 2)

	tbl := strings.ToUpper(flag.Arg(0))
	src := flag.Arg(1)

	fileName := flag.Arg(1)

	rows := make(chan dbcsv.Row)

	ctx, cancel := context.WithCancel(context.Background())
	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		return cfg.ReadRows(ctx, rows, fileName)
	})

	columns, err := CreateTable(ctx, db, tbl, rows, *flagTruncate, *flagTablespace)
	cancel()
	grp.Wait()
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `INSERT INTO "%s" (`, tbl)
	for i, c := range columns {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(c.Name)
	}
	buf.WriteString(") VALUES (")
	for i, _ := range columns {
		if i != 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, ":%d", i+1)
	}
	buf.WriteString(")")
	qry := buf.String()
	log.Println(qry)

	start := time.Now()

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	grp, ctx = errgroup.WithContext(ctx)

	const chunkSize = 128
	rowsCh := make(chan [][]string, *flagConcurrency)
	chunkPool := sync.Pool{New: func() interface{} { return make([][]string, 0, chunkSize) }}

	pool, err := ora.NewPool(*flagDB, *flagConcurrency)
	if err != nil {
		return errors.Wrap(err, *flagDB)
	}
	defer pool.Close()

	for i := 0; i < *flagConcurrency; i++ {
		grp.Go(func() error {
			ses, err := pool.Get()
			if err != nil {
				return err
			}
			defer ses.Close()
			tx, err := ses.StartTx()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			stmt, err := ses.Prep(qry)
			if err != nil {
				return errors.Wrap(err, qry)
			}
			var cols [][]string
			var rowsI []interface{}

			for chunk := range rowsCh {
				if err := ctx.Err(); err != nil {
					return err
				}
				if len(chunk) == 0 {
					continue
				}
				n := len(chunk[0])
				if cap(cols) < n {
					cols = make([][]string, n)
				} else {
					cols = cols[:n]
					for j := range cols {
						cols[j] = cols[j][:0]
					}
				}
				for _, row := range chunk {
					for j, v := range row {
						cols[j] = append(cols[j], v)
					}
				}
				rowsI = rowsI[:0]
				for i, col := range cols {
					rowsI = append(rowsI, columns[i].FromString(col))
				}
				for i := len(cols); i < len(columns); i++ {
					rowsI = append(rowsI, make([]string, len(cols[0])))
				}

				_, err := stmt.Exe(rowsI...)
				chunkPool.Put(chunk)
				if err != nil {
					err = errors.Wrapf(err, "%s, %q", qry, rowsI)
					log.Println(err)

					rowsI2 := make([]interface{}, len(rowsI))
					for j := range cols[0] {
						rowsI2 = rowsI2[:0]
						for i, col := range cols {
							if len(col) <= j {
								break
							}
							rowsI2 = append(rowsI2, columns[i].FromString(col[j:j+1]))
						}
						for i := len(cols); i < len(columns); i++ {
							rowsI2 = append(rowsI2, []string{""})
						}
						if _, err := stmt.Exe(rowsI2...); err != nil {
							err = errors.Wrapf(err, "%s, %q", qry, rowsI2)
							log.Println(err)
							return err
							break
						}
					}

					return err
				}
			}
			return tx.Commit()
		})
	}

	rows = make(chan dbcsv.Row)
	grp.Go(func() error {
		return cfg.ReadRows(ctx, rows, fileName)
	})
	var n int64
	chunk := chunkPool.Get().([][]string)[:0]
	for row := range rows {
		if err := ctx.Err(); err != nil {
			chunk = chunk[:0]
			break
		}
		n++

		if n == 1 {
			continue
		}
		for i, s := range row.Values {
			row.Values[i] = strings.TrimSpace(s)
		}
		chunk = append(chunk, row.Values)
		if len(chunk) < chunkSize {
			continue
		}

		select {
		case rowsCh <- chunk:
		case <-ctx.Done():
			return ctx.Err()
		}

		chunk = chunkPool.Get().([][]string)[:0]
	}
	if len(chunk) != 0 {
		rowsCh <- chunk
	}
	close(rowsCh)

	err = grp.Wait()
	dur := time.Since(start)
	log.Printf("Imported %d rows from %q to %q in %s.", n, src, tbl, dur)
	return err
}

func typeOf(s string) Type {
	if ForceString {
		return String
	}

	if len(s) == 0 {
		return Unknown
	}
	var hasNonDigit bool
	var dotCount int
	var length int
	strings.Map(func(r rune) rune {
		length++
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

func CreateTable(ctx context.Context, db *sql.DB, tbl string, rows <-chan dbcsv.Row, truncate bool, tablespace string) ([]Column, error) {
	tbl = strings.ToUpper(tbl)
	qry := "SELECT COUNT(0) FROM cat WHERE UPPER(table_name) = :1"
	var n int64
	var cols []Column
	if err := db.QueryRowContext(ctx, qry, tbl).Scan(&n); err != nil {
		return cols, errors.Wrap(err, qry)
	}
	if n > 0 && truncate {
		qry = `TRUNCATE TABLE "` + tbl + `"`
		if _, err := db.ExecContext(ctx, qry); err != nil {
			return cols, errors.Wrap(err, qry)
		}
	}

	if n == 0 {
		for row := range rows {
			if cols == nil {
				log.Printf("row: %q", row.Values)
				cols = make([]Column, len(row.Values))
				for i, v := range row.Values {
					v = strings.Map(func(r rune) rune {
						r = unicode.ToLower(r)
						switch r {
						case 'á':
							return 'a'
						case 'é':
							return 'e'
						case 'í':
							return 'i'
						case 'ö', 'ő', 'ó':
							return 'o'
						case 'ü', 'ű', 'ú':
							return 'u'
						case '_':
							return '_'
						default:
							if 'a' <= r && r <= 'z' || '0' <= r && r <= '9' {
								return r
							}
							return '_'
						}
					},
						v)
					if len(v) > 30 {
						v = fmt.Sprintf("%s_%02d", v[:27], i)
					}
					cols[i].Name = v
				}
				continue
			}
			for i, v := range row.Values {
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
			length := c.Length
			if length == 0 {
				length = 1
			}
			fmt.Fprintf(&buf, "  %s %s(%d)", c.Name, c.Type.String(), length)
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
	tRows, err := db.QueryContext(ctx, qry, tbl)
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

func (c Column) FromString(ss []string) interface{} {
	if c.DataType == "DATE" {
		res := make([]time.Time, len(ss))
		for i, s := range ss {
			if s == "" {
				continue
			}
			res[i], _ = time.Parse(dateFormat, s)
		}
		return res
	}

	if strings.HasPrefix(c.DataType, "VARCHAR2") {
		for i, s := range ss {
			if len(s) > c.Length {
				fmt.Fprintf(os.Stderr, "%q is longer (%d) then allowed (%d) for column %v", s, len(s), c.Length, c)
				ss[i] = s[:c.Length]
			}
			return ss
		}
	}
	if c.Type == Int {
		for i, s := range ss {
			e := strings.Map(func(r rune) rune {
				if !('0' <= r && r <= '9' || r == '-') {
					return r
				}
				return -1
			}, s)
			if e != "" {
				fmt.Fprintf(os.Stderr, "%q is not integer (%q)", s, e)
				ss[i] = ""
			}
		}
		return ss
	}
	if c.Type == Float {
		for i, s := range ss {
			e := strings.Map(func(r rune) rune {
				if !('0' <= r && r <= '9' || r == '-' || r == '.') {
					return r
				}
				return -1
			}, s)
			if e != "" {
				fmt.Fprintf(os.Stderr, "%q is not float (%q)", s, e)
				ss[i] = ""
			}
		}
		return ss
	}
	return ss
}
