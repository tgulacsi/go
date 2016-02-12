// Copyright 2015 Tamás Gulácsi. All rights reserved.

package dber

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/errgo.v1"

	"github.com/kylelemons/godebug/diff"
)

var (
	Debug = func(string, ...interface{}) {}

	ErrQueryMismatch = errgo.Newf("query mismatch")
	ErrArgsMismatch  = errgo.Newf("args mismatch")
)

type Mocker interface {
	expectQuery(qry string) Mock
}

type Mock interface {
	WithArgs(...interface{}) Mock
	WillReturnRows(...[]interface{}) Mock
	WithResult(ID, Affected int64) Mock
	WillSetArgs(map[int]interface{}) Mock
}

var _ = Txer((*Tx)(nil))

type Tx struct {
	Expects []*expectQuery
	pos     int
}

// ExpectQuery adds the query to the list of expected queries.
// Iff the query starts and ends with "/", it is treated as a regexp,
// otherwise as plain text.
func (p *Tx) ExpectQuery(qry string) Mock {
	if strings.HasPrefix(qry, "/") && strings.HasSuffix(qry, "/") {
		qry = qry[1 : len(qry)-1]
	} else {
		qry = "\\Q" + stripSpace(qry) + "\\E"
	}
	exp := &expectQuery{Qry: regexp.MustCompile(qry)}
	p.Expects = append(p.Expects, exp)
	return exp
}
func stripSpace(qry string) string {
	var i int
	return strings.Replace(
		strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				i++
				if i == 1 {
					return ' '
				}
				return -1
			}
			i = 0
			return r
		}, qry),
		" ", `\E\s+\Q`, -1)
}

func (cx Tx) Commit() error   { return nil }
func (cx Tx) Rollback() error { return nil }

var _ = Mock(&expectQuery{})

type expectQuery struct {
	Qry     *regexp.Regexp
	Args    []interface{}
	SetArgs map[int]interface{}
	Rows    [][]interface{}
	Result  ResultMock
}

func (exp *expectQuery) WithArgs(args ...interface{}) Mock {
	exp.Args = args
	return exp
}
func (exp *expectQuery) WillReturnRows(rows ...[]interface{}) Mock {
	exp.Rows = rows
	return exp
}
func (exp *expectQuery) WithResult(id, affected int64) Mock {
	exp.Result.ID, exp.Result.Affected = id, affected
	return exp
}
func (exp *expectQuery) WillSetArgs(args map[int]interface{}) Mock {
	exp.SetArgs = args
	return exp
}

// Execute checks whether the given query matches with the next expected.
func (tx *Tx) Exec(qry string, params ...interface{}) (sql.Result, error) {
	exp, err := tx.check(qry, params...)
	if err != nil {
		return nil, err
	}
	for i, v := range exp.SetArgs {
		setPtr(params[i], v)
	}
	return exp.Result, nil
}

func (tx *Tx) Query(qry string, params ...interface{}) (Rowser, error) {
	exp, err := tx.check(qry, params...)
	if err != nil {
		return nil, err
	}
	return &rowsMock{Rows: exp.Rows}, nil
}

func (tx *Tx) QueryRow(qry string, params ...interface{}) Scanner {
	exp, err := tx.check(qry, params...)
	if err != nil {
		return scannerMock{Err: err}
	}
	if len(exp.Rows) == 0 {
		return scannerMock{Err: io.EOF}
	}
	return scannerMock{Row: exp.Rows[0]}
}

const ExpectAny = "{{ExpectAny}}"

func (cu *Tx) check(qry string, args ...interface{}) (*expectQuery, error) {
	exp := cu.Expects[0]
	cu.Expects = cu.Expects[1:]
	cu.pos++
	Debug("pop expect qry=%q, remains %d.", exp.Qry, len(cu.Expects))
	if !exp.Qry.MatchString(qry) {
		return exp, errgo.WithCausef(nil, ErrQueryMismatch, "%d. awaited %q, \ngot\n%q", cu.pos, exp.Qry, qry)
	}
	if len(args) != len(exp.Args) {
		df := diff.Diff(verboseString(exp.Args), verboseString(args))
		return exp, errgo.WithCausef(nil, ErrArgsMismatch, "%d. got %d, want %d:\n%s", cu.pos, len(args), len(exp.Args), df)
	}
	// filter ExpectAny
	expArgsF := make([]interface{}, 0, len(exp.Args))
	argsF := make([]interface{}, 0, len(args))
	for i, v := range exp.Args {
		if v == ExpectAny {
			continue
		}
		expArgsF = append(expArgsF, v)
		argsF = append(argsF, args[i])
	}
	if !reflect.DeepEqual(argsF, expArgsF) {
		df := diff.Diff(verboseString(expArgsF), verboseString(argsF))
		if df != "" {
			return exp, errgo.WithCausef(nil, ErrArgsMismatch, "%d. %s", cu.pos, df)
		}
	}

	return exp, nil
}

var _ = Rowser((*rowsMock)(nil))

type rowsMock struct {
	Rows [][]interface{}
}

func (rm rowsMock) Close() error { return nil }
func (rm rowsMock) Err() error   { return nil }
func (rm *rowsMock) Next() bool {
	if len(rm.Rows) == 0 {
		return false
	}
	rm.Rows = rm.Rows[1:]
	return true
}
func (rm rowsMock) Scan(dest ...interface{}) error {
	return scannerMock{Row: rm.Rows[0]}.Scan(dest...)
}

var _ = Scanner(scannerMock{})

type scannerMock struct {
	Err error
	Row []interface{}
}

func (sm scannerMock) Scan(dest ...interface{}) error {
	for i, d := range dest {
		setPtr(d, sm.Row[i])
	}
	return nil
}

var _ = sql.Result(ResultMock{})

type ResultMock struct {
	ID, Affected int64
}

func (res ResultMock) LastInsertId() (int64, error) { return res.ID, nil }
func (res ResultMock) RowsAffected() (int64, error) { return res.Affected, nil }

func setPtr(d, s interface{}) {
	dst := reflect.ValueOf(d)
	src := reflect.ValueOf(s)
	if !src.IsValid() {
		dst.Elem().Set(reflect.Zero(dst.Type()))
	} else {
		switch src.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.Elem().SetInt(src.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			dst.Elem().SetUint(src.Uint())
		default:
			dst.Elem().Set(src)
		}
	}
}

func verboseString(a interface{}) string {
	m, ok := a.(map[string]interface{})
	if !ok {
		return strings.Replace(fmt.Sprintf("%#v", a), `, "`, ",\n\t\"", -1)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	io.WriteString(&buf, "{\n")
	for _, k := range keys {
		fmt.Fprintf(&buf, "%q: %#v,\n", k, m[k])
	}
	io.WriteString(&buf, "}")
	return buf.String()
}
