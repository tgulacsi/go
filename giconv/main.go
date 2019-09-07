package main

import (
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	errors "golang.org/x/xerrors"
)

func main() {
	flagFrom := flag.String("f", "ISO8859-2", "charset from")
	flagTo := flag.String("t", "UTF8", "charset to")
	flag.Parse()

	f, err := get(*flagFrom)
	if err != nil {
		log.Fatal(err)
	}
	t, err := get(*flagTo)
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(
		encoding.ReplaceUnsupported(t.NewEncoder()).Writer(os.Stdout),
		f.NewDecoder().Reader(os.Stdin),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func get(name string) (encoding.Encoding, error) {
	e, err := htmlindex.Get(name)
	if err == nil {
		return e, nil
	}
	switch strings.ToUpper(name) {
	case "UTF8", "UTF-8":
		return encoding.Nop, nil
	default:
		return encoding.Nop, errors.Errorf("%s: %w", name, err)
	}
}
