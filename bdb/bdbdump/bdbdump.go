package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tgulacsi/go/globalctx"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/pb"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}
func Main() error {
	flag.Parse()

	db, err := badger.Open(badger.DefaultOptions(flag.Arg(0)).WithReadOnly(true).WithBypassLockGuard(true))
	if err != nil {
		return err
	}
	defer db.Close()

	defer os.Stdout.Close()
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	stream := db.NewStream()
	stream.Send = func(kvlist *pb.KVList) (err error) {
		for _, kv := range kvlist.Kv {
			k, v := kv.GetKey(), kv.GetValue()
			if _, err = fmt.Fprintf(out, "+%d,%d:%s->", len(k), len(v), k); err != nil {
				return err
			}
			if _, err := out.Write(v); err != nil {
				return err
			}
			if err = out.WriteByte('\n'); err != nil {
				return err
			}
		}
		return nil
	}
	ctx, cancel := globalctx.Wrap(context.Background())
	err = stream.Orchestrate(ctx)
	cancel()
	if err != nil {
		return err
	}
	if err = out.WriteByte('\n'); err != nil {
		return err
	}
	return out.Flush()
}
