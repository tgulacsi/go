// Copyright 2020 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bddump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package bddump

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/dgraph-io/badger/v2"
)

// Log is used for logging.
var Log = func(...interface{}) error { return nil }

// Dump records.
//
// A record is encoded as +klen,dlen:key->data followed by a newline.
// Here klen is the number of bytes in key and dlen is the number of bytes in data.
// The end of data is indicated by an extra newline.
func Dump(w io.Writer, src string) error {
	defer os.Stdout.Close()
	//Log("msg","open src", "file", src)
	db, err := badger.Open(badger.DefaultOptions(src).WithReadOnly(true))
	if err != nil {
		return err
	}
	defer db.Close()

	out := bufio.NewWriter(w)
	defer out.Flush()

	opt := badger.DefaultIteratorOptions
	opt.PrefetchSize = 16
	if err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()
		n := 0
		var value []byte
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			if value, err = item.ValueCopy(value[:0]); err != nil {
				return err
			}
			fmt.Fprintf(out, "+%d,%d:%s->", len(key), len(value), key)
			if _, err := out.Write(value); err != nil {
				return err
			}
			if err := out.WriteByte('\n'); err != nil {
				return err
			}
			n++
		}
		Log("msg", "Finished.", "rows", n)
		return out.WriteByte('\n')
	}); err != nil {
		return err
	}
	return out.Flush()
}
