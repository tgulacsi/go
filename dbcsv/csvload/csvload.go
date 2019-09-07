// Copyright 2019 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/tgulacsi/go/dbcsv"
	"golang.org/x/sync/errgroup"
	errors "golang.org/x/xerrors"

	_ "gopkg.in/goracle.v2"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

var dateFormat = "2006-01-02 15:04:05"

var ForceString bool

const chunkSize = 1024

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
	flagMemProf := flag.String("memprofile", "", "file to output memory profile to")
	flagCPUProf := flag.String("cpuprofile", "", "file to output CPU profile to")
	flagJustPrint := flag.Bool("just-print", false, "just print the INSERTs")
	flag.Parse()

	if flag.NArg() != 2 {
		log.Fatal("Need two args: the table and the source.")
	}
	if *flagCPUProf != "" {
		f, err := os.Create(*flagCPUProf)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	writeHeapProf := func() {}
	if *flagMemProf != "" {
		f, err := os.Create(*flagMemProf)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		writeHeapProf = func() {
			log.Println("writeHeapProf")
			f.Seek(0, 0)
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}
	}

	if strings.HasPrefix(*flagDB, "$") {
		*flagDB = os.ExpandEnv(*flagDB)
	}
	db, err := sql.Open("goracle", *flagDB)
	if err != nil {
		return errors.Errorf("%s: %w", *flagDB, err)
	}
	defer db.Close()

	db.SetMaxIdleConns(*flagConcurrency)

	tbl := strings.ToUpper(flag.Arg(0))
	src := flag.Arg(1)

	if ForceString {
		err = cfg.OpenVolatile(flag.Arg(1))
	} else {
		err = cfg.Open(flag.Arg(1))
	}
	if err != nil {
		return err
	}
	defer cfg.Close()

	rows := make(chan dbcsv.Row)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(rows)
		cfg.ReadRows(ctx,
			func(_ string, row dbcsv.Row) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case rows <- row:
				}
				return nil
			},
		)
	}()

	if *flagJustPrint {
		cols, err := getColumns(ctx, db, tbl)
		if err != nil {
			return err
		}
		var buf strings.Builder
		for i, col := range cols {
			if i != 0 {
				buf.Write([]byte{',', ' '})
			}
			buf.WriteString(col.Name)
		}
		fmt.Println("INSERT ALL")
		prefix := "  INTO " + tbl + " (" + buf.String() + ")"
		colMap := make(map[string]Column, len(cols))
		for _, col := range cols {
			colMap[col.Name] = col
		}
		cols = cols[:0]
		for _, nm := range (<-rows).Values {
			cols = append(cols, colMap[strings.ToUpper(nm)])
		}
		dRepl := strings.NewReplacer(".", "", "-", "")
		for row := range rows {
			buf.Reset()
			for j, s := range row.Values {
				if j != 0 {
					buf.Write([]byte{',', ' '})
				}
				col := cols[j]
				if col.Type != Date {
					if err = quote(&buf, s); err != nil {
						return err
					}
				} else {
					buf.WriteString("TO_DATE('")
					d := dRepl.Replace(s)
					if len(d) == 6 {
						d = "20" + d
					}
					buf.WriteString(d)
					buf.WriteString("','YYYYMMDD')")
				}
			}
			fmt.Printf("%s VALUES (%s)\n", prefix, buf.String())
		}
		fmt.Println("SELECT 1 FROM DUAL;")
		return nil
	}

	columns, err := CreateTable(ctx, db, tbl, rows, *flagTruncate, *flagTablespace)
	cancel()
	if err != nil {
		return err
	}
	var buf strings.Builder
	fmt.Fprintf(&buf, `INSERT INTO "%s" (`, tbl)
	for i, c := range columns {
		if i != 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(c.Name)
	}
	buf.WriteString(") VALUES (")
	for i := range columns {
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
	grp, ctx := errgroup.WithContext(ctx)

	type rowsType struct {
		Rows  [][]string
		Start int64
	}
	rowsCh := make(chan rowsType, *flagConcurrency)
	chunkPool := sync.Pool{New: func() interface{} { z := make([][]string, 0, chunkSize); return &z }}

	var inserted int64
	for i := 0; i < *flagConcurrency; i++ {
		grp.Go(func() error {
			tx, txErr := db.BeginTx(ctx, nil)
			if txErr != nil {
				return txErr
			}
			defer tx.Rollback()
			stmt, prepErr := tx.Prepare(qry)
			if prepErr != nil {
				return errors.Errorf("%s: %w", qry, prepErr)
			}
			nCols := len(columns)
			cols := make([][]string, nCols)
			rowsI := make([]interface{}, nCols)

			for rs := range rowsCh {
				chunk := rs.Rows
				if err = ctx.Err(); err != nil {
					return err
				}
				if len(chunk) == 0 {
					continue
				}
				nRows := len(chunk)
				for j := range cols {
					if cap(cols[j]) < nRows {
						cols[j] = make([]string, nRows)
					} else {
						cols[j] = cols[j][:nRows]
						for i := range cols[j] {
							cols[j][i] = ""
						}
					}
				}
				for k, row := range chunk {
					if len(row) > len(cols) {
						log.Printf("%d. more elements in the row (%d) then columns (%d)!", rs.Start+int64(k), len(row), len(cols))
						row = row[:len(cols)]
					}
					for j, v := range row {
						cols[j][k] = v
					}
				}

				for i, col := range cols {
					if rowsI[i], err = columns[i].FromString(col); err != nil {
						log.Printf("%d. col: %+v", i, err)
						for k, row := range chunk {
							if _, err = columns[i].FromString(col[k : k+1]); err != nil {
								log.Printf("%d.%q %q: %q", rs.Start+int64(k), columns[i].Name, col[k:k+1], row)
								break
							}
						}

						if err != nil {
							return errors.Errorf("%s: %w", columns[i].Name, err)
						}
						return nil
					}
				}

				_, err = stmt.Exec(rowsI...)
				{
					z := chunk[:0]
					chunkPool.Put(&z)
				}
				if err == nil {
					atomic.AddInt64(&inserted, int64(len(chunk)))
					continue
				}
				err = errors.Errorf("%s: %w", qry, err)
				log.Println(err)

				rowsR := make([]reflect.Value, len(rowsI))
				rowsI2 := make([]interface{}, len(rowsI))
				for j, I := range rowsI {
					rowsR[j] = reflect.ValueOf(I)
					rowsI2[j] = ""
				}
				R2 := reflect.ValueOf(rowsI2)
				for j := range cols[0] { // rows
					for i, r := range rowsR { // cols
						if r.Len() <= j {
							log.Printf("%d[%q]=%d", j, columns[i].Name, r.Len())
							rowsI2[i] = ""
							continue
						}
						R2.Index(i).Set(r.Index(j))
					}
					if _, err = stmt.Exec(rowsI2...); err != nil {
						err = errors.Errorf("%s, %q: %w", qry, rowsI2, err)
						log.Println(err)
						return err
					}
				}

				return err
			}
			return tx.Commit()
		})
	}

	var n int64
	var headerSeen bool
	chunk := (*(chunkPool.Get().(*[][]string)))[:0]
	if err = cfg.ReadRows(ctx,
		func(_ string, row dbcsv.Row) error {
			if err = ctx.Err(); err != nil {
				chunk = chunk[:0]
				return err
			}

			if !headerSeen {
				headerSeen = true
				return nil
			} else if n%10000 == 0 {
				writeHeapProf()
			}
			for i, s := range row.Values {
				row.Values[i] = strings.TrimSpace(s)
			}
			chunk = append(chunk, row.Values)
			if len(chunk) < chunkSize {
				return nil
			}

			select {
			case rowsCh <- rowsType{Rows: chunk, Start: n}:
				n += int64(len(chunk))
			case <-ctx.Done():
				return ctx.Err()
			}

			chunk = (*chunkPool.Get().(*[][]string))[:0]
			return nil
		},
	); err != nil {
		return err
	}

	if len(chunk) != 0 {
		rowsCh <- rowsType{Rows: chunk, Start: n}
		n += int64(len(chunk))
	}
	close(rowsCh)

	err = grp.Wait()
	dur := time.Since(start)
	log.Printf("Read %d, inserted %d rows from %q to %q in %s.", n, inserted, src, tbl, dur)
	return err
}

func typeOf(s string) Type {
	if ForceString {
		return String
	}

	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return Unknown
	}
	var hasNonDigit bool
	var dotCount int
	var length int
	_ = strings.Map(func(r rune) rune {
		length++
		if r == '.' {
			dotCount++
		} else if !hasNonDigit {
			hasNonDigit = !('0' <= r && r <= '9')
		}
		return -1
	},
		s)

	if !hasNonDigit && s[0] != '0' {
		if dotCount == 1 {
			return Float
		}
		if dotCount == 0 {
			return Int
		}
	}
	if 10 <= len(s) && len(s) <= len(dateFormat) {
		if _, err := time.Parse(dateFormat[:len(s)], s); err == nil {
			return Date
		}
	}
	return String
}

func CreateTable(ctx context.Context, db *sql.DB, tbl string, rows <-chan dbcsv.Row, truncate bool, tablespace string) ([]Column, error) {
	tbl = strings.ToUpper(tbl)
	qry := "SELECT COUNT(0) FROM user_tables WHERE UPPER(table_name) = :1"
	var n int64
	var cols []Column
	if err := db.QueryRowContext(ctx, qry, tbl).Scan(&n); err != nil {
		return cols, errors.Errorf("%s: %w", qry, err)
	}
	if n > 0 && truncate {
		qry = `TRUNCATE TABLE ` + tbl
		if _, err := db.ExecContext(ctx, qry); err != nil {
			return cols, errors.Errorf("%s: %w", qry, err)
		}
	}

	if n == 0 {
		row := <-rows
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
		if ForceString {
			for i := range cols {
				cols[i].Type = String
			}
		}
		for row := range rows {
			for i, v := range row.Values {
				if len(v) > cols[i].Length {
					cols[i].Length = len(v)
				}
				if cols[i].Type == String {
					continue
				}
				typ := typeOf(v)
				if cols[i].Type == Unknown {
					cols[i].Type = typ
				} else if typ != cols[i].Type {
					cols[i].Type = String
				}
			}
		}
		var buf bytes.Buffer
		buf.WriteString(`CREATE TABLE "` + tbl + `" (`)
		for i, c := range cols {
			if i != 0 {
				buf.WriteString(",\n")
			}
			if c.Type == Date {
				fmt.Fprintf(&buf, "  %s DATE", c.Name)
				continue
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
		log.Println(qry)
		if _, err := db.Exec(qry); err != nil {
			return cols, errors.Errorf("%s: %w", qry, err)
		}
		cols = cols[:0]
	}

	qry = `SELECT column_name, data_type, NVL(data_length, 0), NVL(data_precision, 0), NVL(data_scale, 0), nullable
  FROM user_tab_cols WHERE table_name = :1
  ORDER BY column_id`
	tRows, err := db.QueryContext(ctx, qry, tbl)
	if err != nil {
		return cols, errors.Errorf("%s: %w", qry, err)
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
	Date    = Type(4)
)

func (t Type) String() string {
	switch t {
	case Int, Float:
		return "NUMBER"
	case Date:
		return "DATE"
	default:
		return "VARCHAR2"
	}
}

func (c Column) FromString(ss []string) (interface{}, error) {
	if c.DataType == "DATE" || c.Type == Date {
		res := make([]time.Time, len(ss))
		for i, s := range ss {
			if s == "" {
				continue
			}
			var err error
			if res[i], err = time.Parse(dateFormat[:len(s)], s); err != nil {
				return res, errors.Errorf("%d. %q: %w", i, s, err)
			}
		}
		return res, nil
	}

	if strings.HasPrefix(c.DataType, "VARCHAR2") {
		for i, s := range ss {
			if len(s) > c.Length {
				ss[i] = s[:c.Length]
				return ss, errors.Errorf("%d. %q is longer (%d) then allowed (%d) for column %v", i, s, len(s), c.Length, c)
			}
		}
		return ss, nil
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
				ss[i] = ""
				return ss, errors.Errorf("%d. %q is not integer (%q)", i, s, e)
			}
		}
		return ss, nil
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
				ss[i] = ""
				return ss, errors.Errorf("%d. %q is not float (%q)", i, s, e)
			}
		}
		return ss, nil
	}
	return ss, nil
}

func getColumns(ctx context.Context, db *sql.DB, tbl string) ([]Column, error) {
	// TODO(tgulacsi): this is Oracle-specific!
	const qry = "SELECT column_name, data_type, data_length, data_precision, data_scale, nullable FROM user_tab_cols WHERE table_name = UPPER(:1) ORDER BY column_id"
	rows, err := db.QueryContext(ctx, qry, tbl)
	if err != nil {
		return nil, errors.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		var c Column
		var prec, scale sql.NullInt64
		var nullable string
		if err = rows.Scan(&c.Name, &c.DataType, &c.Length, &prec, &scale, &nullable); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "Y"
		switch c.DataType {
		case "DATE":
			c.Type = Date
			c.Length = 8
		case "NUMBER":
			c.Precision, c.Scale = int(prec.Int64), int(scale.Int64)
			if c.Scale > 0 {
				c.Type = Float
				c.Length = c.Precision + 1
			} else {
				c.Type = Int
				c.Length = c.Precision
			}
		default:
			c.Type = String
		}
		cols = append(cols, c)
	}
	return cols, rows.Close()
}

var qRepl = strings.NewReplacer(
	"'", "''",
	"&", "'||CHR(38)||'",
)

func quote(w io.Writer, s string) error {
	if _, err := w.Write([]byte{'\''}); err != nil {
		return err
	}
	if _, err := io.WriteString(w, qRepl.Replace(s)); err != nil {
		return err
	}
	_, err := w.Write([]byte{'\''})
	return err
}

// vim: set fileencoding=utf-8 noet:
