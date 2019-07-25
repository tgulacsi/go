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
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "gopkg.in/goracle.v2"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func Main() error {
	flagSource := flag.String("src", os.Getenv("BRUNO_ID"), "user/passw@sid to read from")
	flagDest := flag.String("dst", os.Getenv("BRUNO_ID"), "user/passw@sid to write to")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagConc := flag.Int("concurrency", 8, "concurrency")

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

	tables := make([]copyTask, 0, 4)
	if flag.NArg() == 0 || flag.NArg() == 1 && flag.Arg(0) == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			parts := bytes.SplitN(scanner.Bytes(), []byte(" "), 2)
			var tbl copyTask
			if i := bytes.IndexByte(parts[0], '='); i >= 0 {
				tbl.Src, tbl.Dst = string(parts[0][:i]), string(parts[0][i+1:])
			} else {
				tbl.Src = string(parts[0])
			}
			if len(parts) > 1 {
				tbl.Where = string(parts[1])
			}
			tables = append(tables, tbl)
		}
	} else {
		tbl := copyTask{Src: flag.Arg(0)}
		tbl.Dst = tbl.Src
		if flag.NArg() > 1 {
			tbl.Where = flag.Arg(1)
			if flag.NArg() > 2 {
				tbl.Dst = flag.Args()[2]
			}
		}
		tables = append(tables, tbl)
	}

	srcDB, err := sql.Open("goracle", *flagSource)
	if err != nil {
		return errors.Wrap(err, *flagDest)
	}
	defer srcDB.Close()
	dstDB, err := sql.Open("goracle", *flagDest)
	if err != nil {
		return errors.Wrap(err, *flagDest)
	}
	defer dstDB.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grp, subCtx := errgroup.WithContext(ctx)
	concLimit := make(chan struct{}, *flagConc)
	for _, task := range tables {
		if task.Src == "" {
			continue
		}
		task := task
		grp.Go(func() error {
			select {
			case concLimit <- struct{}{}:
				defer func() { <-concLimit }()
			case <-subCtx.Done():
				return subCtx.Err()
			}
			return One(subCtx, dstDB, srcDB, task)
		})
	}
	return grp.Wait()
}

type copyTask struct {
	Src, Dst, Where string
}

func One(ctx context.Context, dstDB, srcDB *sql.DB, task copyTask) error {
	if task.Dst == "" {
		task.Dst = task.Src
	}
	srcTx, err := srcDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		log.Printf("[WARN] Read-Only transaction: %v", err)
		if srcTx, err = srcDB.BeginTx(ctx, nil); err != nil {
			return errors.Wrap(err, "beginTx")
		}
	}
	defer srcTx.Rollback()
	srcCols, err := getColumns(ctx, srcTx, task.Src)
	if err != nil {
		return err
	}

	dstTx, err := dstDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dstTx.Rollback()

	dstCols, err := getColumns(ctx, dstTx, task.Dst)
	if err != nil {
		return err
	}

	var srcQry, dstQry, ph strings.Builder
	srcQry.WriteString("SELECT ")
	fmt.Fprintf(&dstQry, "INSERT INTO %s (", task.Dst)
	var i int
	for k := range srcCols {
		if _, ok := dstCols[k]; !ok {
			delete(srcCols, k)
		}
		if i != 0 {
			srcQry.WriteByte(',')
			dstQry.WriteByte(',')
			ph.WriteByte(',')
		}
		i++
		srcQry.WriteString(k)
		dstQry.WriteString(k)
		fmt.Fprintf(&ph, ":%d", i)
	}
	fmt.Fprintf(&srcQry, " FROM %s", task.Src)
	if task.Where != "" {
		fmt.Fprintf(&srcQry, " WHERE %s", task.Where)
	}
	fmt.Fprintf(&dstQry, ") VALUES (%s)", ph.String())

	stmt, err := dstTx.PrepareContext(ctx, dstQry.String())
	if err != nil {
		return errors.Wrap(err, dstQry.String())
	}
	defer stmt.Close()
	log.Println(dstQry.String())

	log.Println(srcQry.String())
	rows, err := srcTx.QueryContext(ctx, srcQry.String())
	if err != nil {
		return errors.Wrap(err, srcQry.String())
	}
	defer rows.Close()

	values := make([]interface{}, i)
	for i := range values {
		var x interface{}
		values[i] = &x
	}
	for rows.Next() {
		if err = rows.Scan(values...); err != nil {
			return err
		}
		if _, err = stmt.ExecContext(ctx, values...); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
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
