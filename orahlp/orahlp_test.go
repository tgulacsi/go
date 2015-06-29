// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package orahlp

import (
	"database/sql"
	"flag"
	"reflect"
	"testing"

	"github.com/rana/ora"
	"github.com/tgulacsi/go/dber"
)

var flagConnect = flag.String("connect", "", "user/passw@sid to connect to")

func init() {
	flag.Parse()
}

func TestDescribeQuery(t *testing.T) {
	_ = ora.GetDrv()
	db, err := sql.Open("ora", *flagConnect)
	if err != nil {
		t.Fatalf("cannot connect to %q: %v", *flagConnect, err)
	}
	defer db.Close()
	dbr := dber.SqlDBer{db}
	cols, err := DescribeQuery(dbr, "SELECT * FROM user_objects")
	if err != nil {
		t.Errorf("DescribeQuery: %v", err)
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
