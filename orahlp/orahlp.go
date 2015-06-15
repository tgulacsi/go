// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// orahlp package contains Oracle DB helper functions
package orahlp

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/tgulacsi/go/dber"
	"gopkg.in/errgo.v1"
)

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
  EXCEPTION WHEN OTHERS THEN NULL;
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
	log.Printf("res=%q", res)
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
			return cols, errgo.Notef(err, "parsing %q (parsed: %d)", line, n)
		}
		col.Nullable = nullable != 0
		cols = append(cols, col)
	}
	return cols, nil
}

type Version struct {
	// major.maintenance.application-server.component-specific.platform-specific
	Major, Maintenance, AppServer, Component, Platform int8
}

func GetVersion(db dber.Queryer) (Version, error) {
	var s sql.NullString
	if err := db.QueryRow("SELECT MIN(VERSION) FROM product_component_version " +
		" WHERE product LIKE 'Oracle Database%'").Scan(&s); err != nil {
		return Version{Major: -1}, err
	}
	var v Version
	if _, err := fmt.Sscanf(s.String, "%d.%d.%d.%d.%d",
		&v.Major, &v.Maintenance, &v.AppServer, &v.Component, &v.Platform); err != nil {
		return v, errgo.Notef(err, "scan version number %q", s.String)
	}
	return v, nil
}
