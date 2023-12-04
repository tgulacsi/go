// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lvdump is for dumping the records from a
// LevelDB database, format is http://cr.yp.to/cdb/cdbmake.html
package lvdump

import (
	"bufio"
	"fmt"
	"io"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// Dump records.
//
// A record is encoded as +klen,dlen:key->data followed by a newline.
// Here klen is the number of bytes in key and dlen is the number of bytes in data.
// The end of data is indicated by an extra newline.
func Dump(w io.Writer, src string) error {
	//slog.Debug("open src", "file", src)
	db, err := leveldb.OpenFile(src, &opt.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer db.Close()

	it := db.NewIterator(nil, nil)
	defer it.Release()
	out := bufio.NewWriter(w)
	defer out.Flush()
	for it.Next() {
		fmt.Fprintf(out, "+%d,%d:%s->", len(it.Key()), len(it.Value()), it.Key())
		if _, err := out.Write(it.Value()); err != nil {
			return err
		}
		out.WriteByte('\n')
	}
	if err := out.WriteByte('\n'); err != nil {
		return err
	}
	return out.Flush()
}
