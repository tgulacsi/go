//
// 2018/06/24 14:30:00 messages: 49000 / 10.14862809s: 4900.000 1/s
// 2018/06/24 14:30:00 bytes: 196000000 / 10.14862809s: 19600000.000 1/s
//
package main

import (
	"strings"
	"context"
	"database/sql"
	"flag"
	"log"
	"time"

	"github.com/pkg/errors"

	_ "gopkg.in/goracle.v2"
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
	v_msg VARCHAR2(32767) := :1;
BEGIN
	DBMS_PIPE.PACK_MESSAGE(v_msg);
	:2 := DBMS_PIPE.SEND_MESSAGE('` + pipeName + `');
END;`
	sendStmt, err := db.PrepareContext(ctx, sendQry)
	if err != nil {
		log.Fatal(errors.Wrap(err, sendQry))
	}
	defer sendStmt.Close()

	go func() {
		var n,length int64
		start := time.Now()
		deadline := start.Add(10 * time.Second)
		msg := strings.Repeat("message ", 4096/8)[:4000]
		for deadline.After(time.Now()) {
			for i := 0; i < 1000; i++ {
				var rc int32
				if _, err = sendStmt.ExecContext(ctx, msg, sql.Out{Dest: &rc}); err != nil {
					log.Fatalf("send: %v", err)
				}
				if rc != 0 {
					log.Fatalf("send: %d", rc)
				}
				n++
				length += int64(len(msg))
			}
		}
		dur := time.Since(start)
		log.Printf("messages: %d / %s: %.3f 1/s", n, dur, float64(n) / float64(dur/time.Second))
		log.Printf("bytes: %d / %s: %.3f 1/s", length, dur, float64(length) / float64(dur/time.Second))
		var rc int32
		_, _ = sendStmt.ExecContext(ctx, "QUIT", sql.Out{Dest: &rc})
	}()

	const recvQry = `DECLARE
  v_rc PLS_INTEGER;
  v_msg VARCHAR2(32767);
BEGIN
  v_rc := DBMS_PIPE.RECEIVE_MESSAGE('` + pipeName + `', 5);
  IF v_rc = 0 THEN
    DBMS_PIPE.UNPACK_MESSAGE(v_msg);
  END IF;
  :1 := v_rc;
  :2 := v_msg;
END;`
	recvStmt, err := db.PrepareContext(ctx, recvQry)
	if err != nil {
		log.Fatal(errors.Wrap(err, recvQry))
	}
	defer recvStmt.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var rc int32
		var msg string
		if _, err := recvStmt.ExecContext(ctx, sql.Out{Dest: &rc}, sql.Out{Dest: &msg}); err != nil {
			log.Fatal(err)
		}
		if msg == "QUIT" {
			break
		}
	}

}
