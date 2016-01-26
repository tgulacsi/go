// Copyright 2011-2015, Tamás Gulácsi.
// All rights reserved.
// For details, see the LICENSE file.

package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/errgo.v1"
	"gopkg.in/rana/ora.v3"
)

func dbExec(ses *ora.Ses, fun string, fixParams [][2]string, retOk int64, rows <-chan Row, oneTx bool) (int, error) {
	st, err := getQuery(ses, fun, fixParams)
	if err != nil {
		return 0, err
	}
	var (
		stmt     *ora.Stmt
		tx       *ora.Tx
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
			if tx, err = ses.StartTx(); err != nil {
				return n, err
			}
			if stmt != nil {
				stmt.Close()
			}
			if stmt, err = ses.Prep(st.Qry); err != nil {
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
			v, err := conv(s)
			if err != nil {
				log.Printf("row=%#v error=%v", row, err)
				return n, errgo.Notef(err, "convert %q (row %d, col %d)", s, row.Line, i+1)
			}
			values = append(values, v)
		}
		for i := len(values) + 1; i < st.ParamCount-len(st.FixParams); i++ {
			values = append(values, "")
		}
		for _, s := range st.FixParams {
			values = append(values, s)
		}
		if _, err = stmt.Exe(values...); err != nil {
			log.Printf("values=%d ParamCount=%d", len(values), st.ParamCount)
			log.Printf("execute %q with row %d (%#v): %v", st.Qry, row.Line, values, err)
			return n, errgo.Notef(err, "qry=%q params=%#v", st.Qry, values)
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
					return n, errgo.Newf("returned %v (%s) for line %d (%q).",
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
				st.FixParams = append(st.FixParams, &t)
			case "NUMBER":
				var f float64
				st.FixParams = append(st.FixParams, &f)
			default:
				var s string
				st.FixParams = append(st.FixParams, &s)
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

func strToDate(s string) (interface{}, error) {
	if s == "" {
		return nil, nil
	}
	if 8 <= len(s) && len(s) <= 10 {
		return time.Parse(dateFormat, justNums(s, 8))
	}
	return time.Parse(dateTimeFormat, justNums(s, 14))
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
