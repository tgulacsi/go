// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package orahlp

import (
	"database/sql"
	"flag"
	"reflect"
	"sync"
	"testing"

	"github.com/kylelemons/godebug/diff"
	"github.com/tgulacsi/go/dber"
	"gopkg.in/rana/ora.v3"
)

var flagConnect = flag.String("connect", "", "user/passw@sid to connect to")

var registerOnce sync.Once

func init() {
	flag.Parse()
}

func TestDescribeQuery(t *testing.T) {
	dbr := getConnection(t)
	defer dbr.Close()
	cols, err := DescribeQuery(dbr, "SELECT * FROM user_objects")
	if err != nil {
		t.Skipf("DescribeQuery: %v", err)
	}
	t.Logf("cols=%v", cols)
	awaited := []Column{
		{"", "OBJECT_NAME", 1, 128, 0, 0, true, 32, 1},
		{"", "SUBOBJECT_NAME", 1, 30, 0, 0, true, 32, 1},
		{"", "OBJECT_ID", 2, 22, 0, -127, true, 0, 0},
		{"", "DATA_OBJECT_ID", 2, 22, 0, -127, true, 0, 0},
		{"", "OBJECT_TYPE", 1, 19, 0, 0, true, 32, 1},
		{"", "CREATED", 12, 7, 0, 0, true, 0, 0},
		{"", "LAST_DDL_TIME", 12, 7, 0, 0, true, 0, 0},
		{"", "TIMESTAMP", 1, 19, 0, 0, true, 32, 1},
		{"", "STATUS", 1, 7, 0, 0, true, 32, 1},
		{"", "TEMPORARY", 1, 1, 0, 0, true, 32, 1},
		{"", "GENERATED", 1, 1, 0, 0, true, 32, 1},
		{"", "SECONDARY", 1, 1, 0, 0, true, 32, 1},
	}
	ver, err := GetVersion(dbr)
	if err != nil {
		t.Errorf("get version: %v", err)
	} else if ver.Major >= 11 {
		awaited = append(awaited,
			Column{"", "NAMESPACE", 2, 22, 0, -127, true, 0, 0},
			Column{"", "EDITION_NAME", 1, 30, 0, 0, true, 32, 1},
		)
	}
	if !reflect.DeepEqual(cols, awaited) {
		t.Errorf("Mismatch: \n\tgot %#v,\n\tawaited %#v", cols, awaited)
	}
}

func TestMapToSlice(t *testing.T) {
	for i, tc := range []struct {
		in, await string
		params    []interface{}
	}{
		{
			`SELECT NVL(MAX(F_dazon), :dazon) FROM T_spl_level
			WHERE (F_spl_azon = :lev_azon OR --:lev_azon OR
			       F_ssz = 0 AND F_lev_azon = /*:lev_azon*/:lev_azon)`,
			`SELECT NVL(MAX(F_dazon), :1) FROM T_spl_level
			WHERE (F_spl_azon = :2 OR --:lev_azon OR
			       F_ssz = 0 AND F_lev_azon = /*:lev_azon*/:3)`,
			[]interface{}{"dazon", "lev_azon", "lev_azon"},
		},

		{
			`INSERT INTO PERSON(NAME) VALUES('hello') RETURNING ID INTO :ID`,
			`INSERT INTO PERSON(NAME) VALUES('hello') RETURNING ID INTO :1`,
			[]interface{}{"ID"},
		},

		{
			`DECLARE
  i1 PLS_INTEGER;
  i2 PLS_INTEGER;
  v001 BRUNO.DB_WEB_ELEKTR.KOTVENY_REC_TYP;

BEGIN
  v001.dijkod := :p002#dijkod;

  DB_web.sendpreoffer_31101(p_kotveny=>v001);

  :p002#dijkod := v001.dijkod;

END;
`,
			`DECLARE
  i1 PLS_INTEGER;
  i2 PLS_INTEGER;
  v001 BRUNO.DB_WEB_ELEKTR.KOTVENY_REC_TYP;

BEGIN
  v001.dijkod := :1;

  DB_web.sendpreoffer_31101(p_kotveny=>v001);

  :2 := v001.dijkod;

END;
`,
			[]interface{}{"p002#dijkod", "p002#dijkod"},
		},
	} {

		got, params := MapToSlice(tc.in, func(s string) interface{} { return s })
		d := diff.Diff(tc.await, got)
		if d != "" {
			t.Errorf("%d. diff:\n%s", i, d)
		}
		if !reflect.DeepEqual(params, tc.params) {
			t.Errorf("%d. params: got\n\t%#v,\nwanted\n\t%#v.", i, params, tc.params)
		}
	}
}

func TestGetCompileErrors(t *testing.T) {
	dbr := getConnection(t)
	defer dbr.Close()
	errs, err := GetCompileErrors(dbr, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, ce := range errs {
		t.Logf("%s", ce.Error())
	}
}

func getConnection(t *testing.T) dber.DBer {
	registerOnce.Do(func() { ora.Register(nil) })
	db, err := sql.Open("ora", *flagConnect)
	if err != nil {
		t.Fatalf("cannot connect to %q: %v", *flagConnect, err)
	}
	return dber.SqlDBer{db}
}
