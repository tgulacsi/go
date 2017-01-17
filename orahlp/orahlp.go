// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// Package orahlp contains Oracle DB helper functions
package orahlp

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/tgulacsi/go/dber"
)

// SplitDSN splits the DSN (user/passw@sid).
func SplitDSN(dsn string) (username, password, sid string) {
	if strings.HasPrefix(dsn, "/@") {
		return "", "", dsn[2:]
	}
	if i := strings.LastIndex(dsn, "@"); i >= 0 {
		sid, dsn = dsn[i+1:], dsn[:i]
	}
	if i := strings.IndexByte(dsn, '/'); i >= 0 {
		username, password = dsn[:i], dsn[i+1:]
	}
	return
}

// Column is the described column.
type Column struct {
	Schema, Name                   string
	Type, Length, Precision, Scale int
	Nullable                       bool
	CharsetID, CharsetForm         int
}

// DescribeQuery describes the columns in the qry string,
// using DBMS_SQL.PARSE + DBMS_SQL.DESCRIBE_COLUMNS2.
//
// This can help using unknown-at-compile-time, a.k.a.
// dynamic queries.
func DescribeQuery(db dber.Execer, qry string) ([]Column, error) {
	//res := strings.Repeat("\x00", 32767)
	res := make([]byte, 32767)
	if _, err := db.Exec(`DECLARE
  c INTEGER;
  col_cnt INTEGER;
  rec_tab DBMS_SQL.DESC_TAB;
  a DBMS_SQL.DESC_REC;
  v_idx PLS_INTEGER;
  res VARCHAR2(32767);
BEGIN
  c := DBMS_SQL.OPEN_CURSOR;
  BEGIN
    DBMS_SQL.PARSE(c, :1, DBMS_SQL.NATIVE);
    DBMS_SQL.DESCRIBE_COLUMNS(c, col_cnt, rec_tab);
    v_idx := rec_tab.FIRST;
    WHILE v_idx IS NOT NULL LOOP
      a := rec_tab(v_idx);
      res := res||a.col_schema_name||' '||a.col_name||' '||a.col_type||' '||
                  a.col_max_len||' '||a.col_precision||' '||a.col_scale||' '||
                  (CASE WHEN a.col_null_ok THEN 1 ELSE 0 END)||' '||
                  a.col_charsetid||' '||a.col_charsetform||
                  CHR(10);
      v_idx := rec_tab.NEXT(v_idx);
    END LOOP;
	--Loop ended, close cursor
    DBMS_SQL.CLOSE_CURSOR(c);
  EXCEPTION WHEN OTHERS THEN NULL;
    --Error happened, close cursor anyway!
    DBMS_SQL.CLOSE_CURSOR(c);
	RAISE;
  END;
  :2 := UTL_RAW.CAST_TO_RAW(res);
END;`, qry, &res,
	); err != nil {
		return nil, err
	}
	if i := bytes.IndexByte(res, 0); i >= 0 {
		res = res[:i]
	}
	lines := bytes.Split(res, []byte{'\n'})
	cols := make([]Column, 0, len(lines))
	var nullable int
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var col Column
		switch j := bytes.IndexByte(line, ' '); j {
		case -1:
			continue
		case 0:
			line = line[1:]
		default:
			col.Schema, line = string(line[:j]), line[j+1:]
		}
		if n, err := fmt.Sscanf(string(line), "%s %d %d %d %d %d %d %d",
			&col.Name, &col.Type, &col.Length, &col.Precision, &col.Scale, &nullable, &col.CharsetID, &col.CharsetForm,
		); err != nil {
			return cols, errors.Wrapf(err, "parsing %q (parsed: %d)", line, n)
		}
		col.Nullable = nullable != 0
		cols = append(cols, col)
	}
	return cols, nil
}

// Version data.
type Version struct {
	// major.maintenance.application-server.component-specific.platform-specific
	Major, Maintenance, AppServer, Component, Platform int8
}

// GetVersion returns the Oracle product version.
func GetVersion(db dber.Queryer) (Version, error) {
	var s sql.NullString
	if err := db.QueryRow("SELECT MIN(VERSION) FROM product_component_version " +
		" WHERE product LIKE 'Oracle Database%'").Scan(&s); err != nil {
		return Version{Major: -1}, err
	}
	var v Version
	if _, err := fmt.Sscanf(s.String, "%d.%d.%d.%d.%d",
		&v.Major, &v.Maintenance, &v.AppServer, &v.Component, &v.Platform); err != nil {
		return v, errors.Wrapf(err, "scan version number %q", s.String)
	}
	return v, nil
}

// MapToSlice modifies query for map (:paramname) to :%d placeholders + slice of params.
//
// Calls metParam for each parameter met, and returns the slice of their results.
func MapToSlice(qry string, metParam func(string) interface{}) (string, []interface{}) {
	if metParam == nil {
		metParam = func(string) interface{} { return nil }
	}
	arr := make([]interface{}, 0, 16)
	var buf bytes.Buffer
	state, p, last := 0, 0, 0
	var prev rune

	Add := func(i int) {
		state = 0
		if i-p <= 1 { // :=
			return
		}
		arr = append(arr, metParam(qry[p+1:i]))
		param := fmt.Sprintf(":%d", len(arr))
		buf.WriteString(qry[last:p])
		buf.WriteString(param)
		last = i
	}

	for i, r := range qry {
		switch state {
		case 2:
			if r == '\n' {
				state = 0
			}
		case 3:
			if prev == '*' && r == '/' {
				state = 0
			}
		case 0:
			switch r {
			case '-':
				if prev == '-' {
					state = 2
				}
			case '*':
				if prev == '/' {
					state = 3
				}
			case ':':
				state = 1
				p = i
				// An identifier consists of a letter optionally followed by more letters, numerals, dollar signs, underscores, and number signs.
				// http://docs.oracle.com/cd/B19306_01/appdev.102/b14261/fundamentals.htm#sthref309
			}
		case 1:
			if !('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' ||
				(i-p > 1 && ('0' <= r && r <= '9' || r == '$' || r == '_' || r == '#'))) {

				Add(i)
			}
		}
		prev = r
	}
	if state == 1 {
		Add(len(qry))
	}
	if last <= len(qry)-1 {
		buf.WriteString(qry[last:])
	}
	return buf.String(), arr
}

// CompileError represents a compile-time error as in user_errors view.
type CompileError struct {
	Owner, Name, Type    string
	Line, Position, Code int64
	Text                 string
	Warning              bool
}

func (ce CompileError) Error() string {
	prefix := "ERROR "
	if ce.Warning {
		prefix = "WARN  "
	}
	return fmt.Sprintf("%s %s.%s %s %d:%d [%d] %s",
		prefix, ce.Owner, ce.Name, ce.Type, ce.Line, ce.Position, ce.Code, ce.Text)
}

// GetCompileErrors returns the slice of the errors in user_errors.
//
// If all is false, only errors are returned; otherwise, warnings, too.
func GetCompileErrors(queryer dber.Queryer, all bool) ([]CompileError, error) {
	rows, err := queryer.Query(`
	SELECT USER owner, name, type, line, position, message_number, text, attribute
		FROM user_errors
		ORDER BY name, sequence`)
	if err != nil {
		return nil, err
	}
	var errors []CompileError
	var warn string
	for rows.Next() {
		var ce CompileError
		if err = rows.Scan(&ce.Owner, &ce.Name, &ce.Type, &ce.Line, &ce.Position, &ce.Code, &ce.Text, &warn); err != nil {
			return errors, err
		}
		ce.Warning = warn == "WARNING"
		if !ce.Warning || all {
			errors = append(errors, ce)
		}
	}
	return errors, rows.Err()
}
