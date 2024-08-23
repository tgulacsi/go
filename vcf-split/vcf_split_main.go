// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flagDir := flag.String("o", "", "output dir")
	flag.Parse()
	fh, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer fh.Close()
	scanner := bufio.NewScanner(fh)

	var buf bytes.Buffer
	var inCard bool
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		if inCard {
			buf.Write(b)
			buf.WriteByte('\n')
			if bytes.Equal(b, []byte("END:VCARD")) {
				if err := writeCard(*flagDir, buf.Bytes()); err != nil {
					return err
				}
				inCard = false
			}
		} else if bytes.Equal(b, []byte("BEGIN:VCARD")) {
			inCard = true
			buf.Reset()
			buf.Write(b)
			buf.WriteByte('\n')
		} else {
			log.Printf("SKIP %q", b)
		}
	}
	return writeCard(*flagDir, buf.Bytes())
}

func writeCard(dir string, data []byte) error {
	var uid string
	if i := bytes.Index(data, []byte("\nUID:")); i >= 0 {
		b := data[i+5:]
		if j := bytes.IndexByte(b, '\n'); j > 0 {
			uid = string(b[:j])
		}
	}
	if uid == "" {
		a := sha256.Sum224(data)
		uid = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(a[:])
		if i := bytes.Index(data, []byte("\nEND:VCARD")); i >= 0 {
			data = append(
				data[:i],
				("\nUID:" + uid + "\nEND:VCARD")...)
		}
	}
	name := uid
	if i := bytes.Index(data, []byte("\nN:")); i >= 0 {
		b := data[i+3:]
		if i := bytes.IndexByte(b, '\n'); i > 0 {
			lastIsU := true
			name = strings.Trim(strings.Map(func(r rune) rune {
				switch r {
				case ';', ':', '/', '\\', ' ', '\t', '\n', '\v', '\r':
					if lastIsU {
						return -1
					}
					lastIsU = true
					return '_'
				}
				lastIsU = false
				return r
			},
				string(bytes.TrimRight(b[:i], ";"))),
				"_")
		}
	}
	fn := filepath.Join(dir, name+".vcf")
	return renameio.WriteFile(fn, append(data, '\n'), 0644)
}
