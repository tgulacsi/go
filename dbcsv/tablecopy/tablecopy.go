/*
   Copyright 2019 Tamás Gulácsi

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package main in tablecopy is a table copier between databases.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "gopkg.in/goracle.v2"

	"github.com/pkg/errors"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func Main() error {
	flagSource := flag.String("source", os.Getenv("BRUNO_ID"), "user/passw@sid to read from")
	flagDest := flag.String("dest", os.Getenv("BRUNO_ID"), "user/passw@sid to write to")
	flagVerbose := flag.Bool("v", false, "verbose logging")

	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), strings.Replace(`Usage of {{.prog}}:
	{{.prog}} [options] 'T_able'

will execute a "SELECT * FROM T_able@source_db" and an "INSERT INTO T_able@dest_db"

	{{.prog}} [options] 'Source_table' 'F_ield=1'

will execute a "SELECT * FROM Source_table@source_db WHERE F_ield=1" and an "INSERT INTO Source_table@dest_db"

	{{.prog}} 'Source_table' '1=1' 'Dest_table'
will execute a "SELECT * FROM Source_table@source_db WHERE F_ield=1" and an "INSERT INTO Dest_table@dest_db", matching the fields.

`, "{{.prog}}", os.Args[0], -1))
		flag.PrintDefaults()
	}
	flag.Parse()

	var Log func(...interface{}) error
	_ = Log
	if *flagVerbose {
		Log = func(keyvals ...interface{}) error {
			if len(keyvals)%2 != 0 {
				keyvals = append(keyvals, "")
			}
			vv := make([]interface{}, len(keyvals)/2)
			for i := range vv {
				v := fmt.Sprintf("%+v", keyvals[(i<<1)+1])
				if strings.Contains(v, " ") {
					v = `"` + v + `"`
				}
				vv[i] = fmt.Sprintf("%s=%s", keyvals[(i<<1)], v)
			}
			log.Println(vv...)
			return nil
		}
	}

	srcTbl := flag.Arg(0)
	dstTbl := srcTbl
	var where string
	if flag.NArg() > 1 {
		where = flag.Arg(1)
		if flag.NArg() > 2 {
			dstTbl = flag.Args()[2]
		}
	}
	if where == "" {
		where = "1=1"
	}
	srcDB, err := sql.Open("goracle", *flagSource)
	if err != nil {
		return errors.Wrap(err, *flagDest)
	}
	defer srcDB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srcTx, err := srcDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		log.Printf("[WARN] Read-Only transaction: %v", err)
		if srcTx, err = srcDB.BeginTx(ctx, nil); err != nil {
			return errors.Wrap(err, "beginTx")
		}
	}
	defer srcTx.Rollback()
	srcCols, err := getColumns(ctx, srcTx, srcTbl)
	if err != nil {
		return err
	}

	dstDB, err := sql.Open("goracle", *flagDest)
	if err != nil {
		return errors.Wrap(err, *flagDest)
	}
	defer dstDB.Close()
	dstTx, err := dstDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dstTx.Rollback()

	dstCols, err := getColumns(ctx, dstTx, dstTbl)
	if err != nil {
		return err
	}

	var srcQry, dstQry, ph strings.Builder
	srcQry.WriteString("SELECT ")
	fmt.Fprintf(&dstQry, "INSERT INTO %s (", dstTbl)
	var i int
	for k := range srcCols {
		if _, ok := dstCols[k]; !ok {
			delete(srcCols, k)
		}
		if i == 0 {
			srcQry.WriteByte(',')
			dstQry.WriteByte(',')
			ph.WriteByte(',')
		}
		i++
		srcQry.WriteString(k)
		dstQry.WriteString(k)
		fmt.Fprintf(&ph, ":%d", i)
	}
	fmt.Fprintf(&srcQry, " FROM %s WHERE %s", srcTbl, where)
	fmt.Fprintf(&dstQry, ") VALUES (%s)", ph.String())

	log.Println(srcQry)
	log.Println(dstQry)

	return dstTx.Commit()
}

func getColumns(ctx context.Context, tx *sql.Tx, tbl string) (map[string]struct{}, error) {
	qry := "SELECT * FROM " + tbl + " WHERE 1=0"
	rows, err := tx.QueryContext(ctx, qry)
	if err != nil {
		return nil, errors.Wrap(err, qry)
	}
	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct{}, len(colNames))
	for _, c := range colNames {
		m[c] = struct{}{}
	}
	return m, err
}

// vim: se noet fileencoding=utf-8:
