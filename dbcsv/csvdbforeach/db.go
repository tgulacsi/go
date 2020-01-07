// Copyright 2017, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tgulacsi/go/dbcsv"
	errors "golang.org/x/xerrors"
)

const (
	dateFormat     = dbcsv.DateFormat
	dateTimeFormat = dbcsv.DateTimeFormat
)

func dbExec(db *sql.DB, fun string, fixParams [][2]string, retOk int64, rows <-chan dbcsv.Row, oneTx bool) (int, error) {
	st, err := getQuery(db, fun, fixParams)
	if err != nil {
		return 0, err
	}
	log.Printf("st=%#v", st)
	var (
		stmt     *sql.Stmt
		tx       *sql.Tx
		values   []interface{}
		startIdx int
		ret      int64
		n        int
		buf      bytes.Buffer
	)
	if st.Returns {
		values = append(values, &ret)
		startIdx = 1
	}

	for row := range rows {
		if tx == nil {
			if tx, err = db.Begin(); err != nil {
				return n, err
			}
			if stmt != nil {
				stmt.Close()
			}
			if stmt, err = tx.Prepare(st.Qry); err != nil {
				tx.Rollback()
				return n, err
			}
		}

		values = values[:startIdx]
		for i, s := range row.Values {
			conv := st.Converters[i]
			if conv == nil {
				values = append(values, s)
				continue
			}
			v, convErr := conv(s)
			if convErr != nil {
				log.Printf("row=%#v error=%v", row, convErr)
				return n, errors.Errorf("convert %q (row %d, col %d): %w", s, row.Line, i+1, convErr)
			}
			values = append(values, v)
		}
		for i := len(values) + 1; i < st.ParamCount-len(st.FixParams); i++ {
			values = append(values, "")
		}
		values = append(values, st.FixParams...)
		//log.Printf("%q %#v", st.Qry, values)
		if _, err = stmt.Exec(values...); err != nil {
			log.Printf("values=%d ParamCount=%d", len(values), st.ParamCount)
			log.Printf("execute %q with row %d (%#v): %v", st.Qry, row.Line, values, err)
			return n, errors.Errorf("qry=%q params=%#v: %w", st.Qry, values, err)
		}
		n++
		if st.Returns && values[0] != nil {
			out := strings.Join(deref(st.FixParams), ", ")
			if ret == retOk {
				fmt.Fprintf(stdout, "%d: OK [%s]\t%s\n", ret, out, row.Values)
			} else {
				fmt.Fprintf(stderr, "%d: %s\t%s\n", ret, out, row.Values)
				log.Printf("ROLLBACK (ret=%v)", ret)
				tx.Rollback()
				tx = nil
				buf.Reset()
				cw := csv.NewWriter(&buf)
				cw.Write(append([]string{fmt.Sprintf("%d", ret), out}, row.Values...))
				cw.Flush()
				stdout.Write(buf.Bytes())
				if oneTx {
					return n, errors.Errorf("returned %v (%s) for line %d (%q)",
						ret, out, row.Line, row.Values)
				}
			}
		}
		if tx != nil && !oneTx {
			log.Printf("COMMIT")
			if err = tx.Commit(); err != nil {
				return n, err
			}
			tx = nil
		}
	}
	if stmt != nil {
		stmt.Close()
	}
	if tx != nil {
		return n, tx.Commit()
	}
	return n, nil
}

type ConvFunc func(string) (interface{}, error)

type Statement struct {
	Qry        string
	Returns    bool
	Converters []ConvFunc
	ParamCount int
	FixParams  []interface{}
}

type querier interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

func getQuery(db querier, fun string, fixParams [][2]string) (Statement, error) {
	var st Statement
	args := make([]Arg, 0, 32)
	fun = strings.TrimSpace(fun)

	if strings.HasPrefix(fun, "BEGIN ") && strings.HasSuffix(fun, "END;") {
		st.Qry = fun
		if i := strings.IndexByte(fun, '('); i >= 0 && strings.Contains(fun[5:i], ":=") { //function
			st.Returns = true
		}
		var nm []byte
		var state uint8
		names := make([]string, 0, strings.Count(fun, ":"))
		_ = strings.Map(func(r rune) rune {
			switch state {
			case 0:
				if r == ':' {
					state = 1
					nm = nm[:0]
				}
			case 1:
				if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' ||
					'0' <= r && r <= '9' ||
					len(nm) > 0 && r == '_' {
					nm = append(nm, byte(r))
				} else {
					names = append(names, string(nm))
					nm = nm[:0]
					state = 0
				}
			}
			return -1
		},
			fun)
		if len(nm) > 0 {
			names = append(names, string(nm))
		}
		st.ParamCount = len(names)
		st.Converters = make([]ConvFunc, len(names))
		return st, nil
	}

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
		return st, errors.Errorf("bad function name: %s", fun)
	}
	qry += " ORDER BY sequence"
	rows, err := db.Query(qry, params...)
	if err != nil {
		return st, errors.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()

	for rows.Next() {
		var arg Arg
		var length, precision, scale sql.NullInt64
		if err = rows.Scan(&arg.Name, &arg.Type, &arg.InOut, &length, &precision, &scale); err != nil {
			return st, err
		}
		if length.Valid {
			arg.Length = int(length.Int64)
			if precision.Valid {
				arg.Precision = int(precision.Int64)
				if scale.Valid {
					arg.Scale = int(scale.Int64)
				}
			}
		}
		args = append(args, arg)
	}
	if err = rows.Err(); err != nil {
		return st, errors.Errorf("%s: %w", qry, err)
	}
	if len(args) == 0 {
		return st, errors.Errorf("%s has no arguments!", fun)
	}

	st.Qry = "BEGIN "
	i := 1
	if args[0].Name == "" { // function
		st.Qry += ":x1 := "
		args = args[1:]
		st.Returns = true
		i++
	}
	fixParamNames := make([]string, len(fixParams))
	for j, x := range fixParams {
		fixParamNames[j] = strings.ToUpper(x[0])
	}
	vals := make([]string, 0, len(args))
	st.Converters = make([]ConvFunc, cap(vals))
ArgLoop:
	for j, arg := range args {
		for _, x := range fixParamNames {
			if x == arg.Name {
				continue ArgLoop
			}
		}
		vals = append(vals, fmt.Sprintf("%s=>:x%d", strings.ToLower(arg.Name), i))
		if arg.InOut == "OUT" {
			switch arg.Type {
			case "DATE":
				var t time.Time
				st.FixParams = append(st.FixParams, sql.Out{Dest:&t})
			case "NUMBER":
				var f float64
				st.FixParams = append(st.FixParams, sql.Out{Dest:&f})
			default:
				var s string
				st.FixParams = append(st.FixParams, sql.Out{Dest:&s})
			}
		} else if arg.Type == "DATE" {
			st.Converters[j] = strToDate
		}
		i++
	}
	for _, p := range fixParams {
		vals = append(vals, fmt.Sprintf("%s=>:x%d", p[0], i))
		st.FixParams = append(st.FixParams, p[1])
		i++
	}
	st.ParamCount = i
	st.Qry += fun + "(" + strings.Join(vals, ", ") + "); END;"
	return st, err
}

type Arg struct {
	Name, Type, InOut        string
	Length, Precision, Scale int
}

func strToDate(s string) (interface{}, error) {
	s = justNums(s, 14)
	if s == "" {
		return nil, nil
	}
	if len(s) < 14 {
		return time.ParseInLocation(dateFormat, s[:8], time.Local)
	}
	return time.ParseInLocation(dateTimeFormat, s, time.Local)
}
func justNums(s string, maxLen int) string {
	var i int
	return strings.Map(
		func(r rune) rune {
			if maxLen >= 0 {
				if i > maxLen {
					return -1
				}
			}
			if '0' <= r && r <= '9' {
				i++
				return r
			}
			return -1
		},
		s)
}

func deref(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == nil {
			out = append(out, "")
			continue
		}
		switch x := v.(type) {
		case *string:
			out = append(out, *x)
		case *int64:
			out = append(out, strconv.FormatInt(*x, 10))
		case *float64:
			out = append(out, fmt.Sprintf("%f", *x))
		case *time.Time:
			out = append(out, x.Format("2006-01-02"))
		default:
			rv := reflect.ValueOf(v)
			if rv.Kind() != reflect.Ptr {
				continue
			}
			out = append(out, fmt.Sprintf("%v", rv.Elem().Interface()))
		}
	}
	return out
}

// vim: set fileencoding=utf-8 noet:
