package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

func main() {
	flagFrom := flag.String("f", "ISO8859-2", "charset from")
	flagTo := flag.String("t", "UTF8", "charset to")
	flag.Parse()

	f, err := htmlindex.Get(*flagFrom)
	if err != nil {
		log.Fatal(errors.Wrap(err, *flagFrom))
	}
	t, err := htmlindex.Get(*flagTo)
	if err != nil {
		log.Fatal(errors.Wrap(err, *flagFrom))
	}

	_, err = io.Copy(
		encoding.ReplaceUnsupported(t.NewEncoder()).Writer(os.Stdout),
		f.NewDecoder().Reader(os.Stdin),
	)
	if err != nil {
		log.Fatal(err)
	}
}
