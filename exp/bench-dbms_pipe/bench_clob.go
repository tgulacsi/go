// +build: never

//
// 2018/06/26 13:07:50 messages: 16000 / 10.318777996s: 1600.000 1/s
// 2018/06/26 13:07:50 bytes: 524288000 / 10.318777996s: 50.000 Mb/s
//
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"

	goracle "gopkg.in/goracle.v2"
)

func main() {
	flag.Parse()
	db, err := sql.Open("goracle", flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const pipeName = `test_pipe`

	const sendQry = `DECLARE
	v_clob CLOB := :1;
BEGIN
  :2 := DBMS_LOB.substr(v_clob, 10, DBMS_LOB.getlength(v_clob)-9);
END;`
	stmt, err := db.PrepareContext(ctx, sendQry)
	if err != nil {
		log.Fatal(errors.Wrap(err, sendQry))
	}
	defer stmt.Close()

	var n, length int64
	start := time.Now()
	deadline := start.Add(10 * time.Second)
	msg := strings.Repeat("message ", 32768/8)[:32768]
	await := msg[len(msg)-10:]
	for deadline.After(time.Now()) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		for i := 0; i < 1000; i++ {
			var part string
			lob := goracle.Lob{Reader: strings.NewReader(msg),IsClob:true}
			if _, err = stmt.ExecContext(ctx, lob, sql.Out{Dest: &part}); err != nil {
				log.Fatalf("send: %v", err)
			}
			if part != await {
				log.Fatalf("got %q, awaited %q.", part, await)
			}
			n++
			length += int64(len(msg))
		}
	}
	dur := time.Since(start)
	log.Printf("messages: %d / %s: %.3f 1/s", n, dur, float64(n)/float64(dur/time.Second))
	units := []string{"b", "kb", "Mb", "Gb"}
	rate := float64(length)/float64(dur/time.Second)
	for  len(units) > 1 && rate > 1024 {
		rate /= 1024
		units = units[1:]
	}
	log.Printf("bytes: %d / %s: %.3f %s/s", length, dur, rate, units[0])
}
