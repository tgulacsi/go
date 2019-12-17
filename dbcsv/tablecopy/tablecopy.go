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
	"database/sql/driver"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	godror "github.com/godror/godror"

	"golang.org/x/sync/errgroup"
	errors "golang.org/x/xerrors"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func Main() error {
	flagSource := flag.String("src", os.Getenv("BRUNO_ID"), "user/passw@sid to read from")
	flagSourcePrep := flag.String("src-prep", "", "prepare source connection (run statements separated by ;\\n)")
	flagDest := flag.String("dst", os.Getenv("BRUNO_ID"), "user/passw@sid to write to")
	flagDestPrep := flag.String("dst-prep", "", "prepare destination connection (run statements separated by ;\\n)")
	flagReplace := flag.String("replace", "", "replace FIELD_NAME=WITH_VALUE,OTHER=NEXT")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagTimeout := flag.Duration("timeout", 1*time.Minute, "timeout")
	flagTableTimeout := flag.Duration("table-timeout", 10*time.Second, "per-table-timeout")
	flagConc := flag.Int("concurrency", 8, "concurrency")
	flagTruncate := flag.Bool("truncate", false, "truncate dest tables (must have different name)")

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
	if *flagTimeout == 0 {
		*flagTimeout = time.Hour
	}
	if *flagTableTimeout > *flagTimeout {
		*flagTableTimeout = *flagTimeout
	}

	var Log func(...interface{}) error
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
	var replace map[string]string
	if *flagReplace != "" {
		fields := strings.Split(*flagReplace, ",")
		replace = make(map[string]string, len(fields))
		for _, f := range fields {
			if i := strings.IndexByte(f, '='); i < 0 {
				continue
			} else {
				replace[strings.ToUpper(f[:i])] = f[i+1:]
			}
		}
	}

	tables := make([]copyTask, 0, 4)
	if flag.NArg() == 0 || flag.NArg() == 1 && flag.Arg(0) == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			parts := bytes.SplitN(scanner.Bytes(), []byte(" "), 2)
			tbl := copyTask{Replace: replace, Truncate: *flagTruncate}
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
		tbl := copyTask{Src: flag.Arg(0), Replace: replace, Truncate: *flagTruncate}
		if flag.NArg() > 1 {
			tbl.Where = flag.Arg(1)
			if flag.NArg() > 2 {
				tbl.Dst = flag.Args()[2]
			}
		}
		tables = append(tables, tbl)
	}

	mkInit := func(queries string) func(driver.Conn) error {
		if queries == "" {
			return func(driver.Conn) error { return nil }
		}
		qs := strings.Split(queries, ";\n") 
		return func(conn driver.Conn) error {
			for _, qry := range qs {
			stmt, err := conn.Prepare(qry)
			if err != nil {
				return errors.Errorf("%s: %w", qry, err)
			}
			_, err = stmt.Exec(nil)
			stmt.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
}

	srcConnector, err := godror.NewConnector(*flagSource, mkInit(*flagSourcePrep))
	if err != nil {
		return err
	}
	srcDB := sql.OpenDB(srcConnector)
	defer srcDB.Close()

	dstConnector, err := godror.NewConnector(*flagDest, mkInit(*flagDestPrep))
	if err != nil {
		return err
	}
	dstDB := sql.OpenDB(dstConnector)
	defer dstDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *flagTimeout)
	defer cancel()

	grp, subCtx := errgroup.WithContext(ctx)
	concLimit := make(chan struct{}, *flagConc)
	srcTx, err := srcDB.BeginTx(subCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		log.Printf("[WARN] Read-Only transaction: %v", err)
		if srcTx, err = srcDB.BeginTx(subCtx, nil); err != nil {
			return errors.Errorf("%s: %w", "beginTx", err)
		}
	}
	defer srcTx.Rollback()

	dstTx, err := dstDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer dstTx.Rollback()

	for _, task := range tables {
		if task.Src == "" {
			continue
		}
		if task.Dst == "" {
			task.Dst = task.Src
		} else if !strings.EqualFold(task.Dst, task.Src) {
			dstDB.ExecContext(subCtx, fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s WHERE 1=0", task.Dst, task.Src))
			if task.Truncate {
				dstDB.ExecContext(subCtx, "TRUNCATE TABLE "+task.Dst)
			}
		}
	}
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
			start := time.Now()
			oneCtx, oneCancel := context.WithTimeout(subCtx, *flagTableTimeout)
			n, err := One(oneCtx, dstTx, srcTx, task, Log)
			oneCancel()
			dur := time.Since(start)
			log.Println(task.Src, n, dur)
			return err
		})
	}
	if err := grp.Wait(); err != nil {
		return err
	}
	return dstTx.Commit()
}

type copyTask struct {
	Src, Dst, Where string
	Replace         map[string]string
	Truncate        bool
}

func One(ctx context.Context, dstTx, srcTx *sql.Tx, task copyTask, Log func(...interface{}) error) (int64, error) {
	if task.Dst == "" {
		task.Dst = task.Src
	}
	var n int64
	srcCols, err := getColumns(ctx, srcTx, task.Src)
	if err != nil {
		return n, err
	}

	dstCols, err := getColumns(ctx, dstTx, task.Dst)
	if err != nil {
		return n, err
	}
	m := make(map[string]struct{}, len(dstCols))
	for _, c := range dstCols {
		m[c] = struct{}{}
	}

	var srcQry, dstQry, ph strings.Builder
	srcQry.WriteString("SELECT ")
	fmt.Fprintf(&dstQry, "INSERT INTO %s (", task.Dst)
	var i int
	tbr := make([]string, 0, len(task.Replace))
	for _, k := range srcCols {
		if _, ok := m[k]; !ok {
			continue
		}
		if _, ok := task.Replace[k]; ok {
			tbr = append(tbr, k)
			continue
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
	for _, k := range tbr {
		dstQry.WriteByte(',')
		dstQry.WriteString(k)
		ph.WriteString(",'")
		ph.WriteString(strings.ReplaceAll(task.Replace[k], "'", "''"))
		ph.WriteByte('\'')
	}
	fmt.Fprintf(&srcQry, " FROM %s", task.Src)
	if task.Where != "" {
		fmt.Fprintf(&srcQry, " WHERE %s", task.Where)
	}
	fmt.Fprintf(&dstQry, ") VALUES (%s)", ph.String())

	stmt, err := dstTx.PrepareContext(ctx, dstQry.String())
	if err != nil {
		return n, errors.Errorf("%s: %w", dstQry.String(), err)
	}
	defer stmt.Close()
	if Log != nil {
		Log("src", dstQry.String())
		Log("dst", srcQry.String())
	}

	rows, err := srcTx.QueryContext(ctx, srcQry.String())
	if err != nil {
		return n, errors.Errorf("%s: %w", srcQry.String(), err)
	}
	defer rows.Close()

	values := make([]interface{}, i)
	for i := range values {
		var x interface{}
		values[i] = &x
	}
	for rows.Next() {
		if err = rows.Scan(values...); err != nil {
			return n, err
		}
		if _, err = stmt.ExecContext(ctx, values...); err != nil {
			return n, errors.Errorf("%s %v: %w", dstQry.String(), values, err)
		}
		n++
	}
	return n, nil
}

func getColumns(ctx context.Context, tx *sql.Tx, tbl string) ([]string, error) {
	qry := "SELECT * FROM " + tbl + " WHERE 1=0"
	rows, err := tx.QueryContext(ctx, qry)
	if err != nil {
		return nil, errors.Errorf("%s: %w", qry, err)
	}
	cols, err := rows.Columns()
	rows.Close()
	return cols, err
}

// vim: se noet fileencoding=utf-8:
