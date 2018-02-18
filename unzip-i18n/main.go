// Copyright 2016 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/tgulacsi/go/text"
	"golang.org/x/text/encoding/htmlindex"
)

func main() {

	flagList := flag.Bool("l", false, "just list the files")
	flagEnc := flag.String("encoding", "cp850", "encoding")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "\n%s [flags] to-be-extracted.zip [filename...]\n\n", os.Args[1])
		flag.PrintDefaults()
	}
	flag.Parse()

	fh, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		log.Fatal(err)
	}
	zr, err := zip.NewReader(fh, fi.Size())
	if err != nil {
		log.Fatal(err)
	}
	enc := text.GetEncoding(*flagEnc)
	if enc == nil {
		if enc, err = htmlindex.Get(*flagEnc); err != nil {
			log.Fatal(err)
		}
	}
	var wanted map[string]struct{}
	if n := flag.NArg() - 1; n > 0 {
		wanted = make(map[string]struct{}, n)
		for i := 0; i < n; i++ {
			wanted[flag.Arg(i+1)] = struct{}{}
		}
	}
	d := enc.NewDecoder()
	for _, f := range zr.File {
		name := f.Name
		if f.NonUTF8 {
			s, err := d.String(name)
			if err != nil {
				log.Printf("Decode %q: %v", f.Name, err)
			} else {
				name = s
			}
		}
		if wanted != nil {
			if _, ok := wanted[name]; !ok {
				continue
			}
		}
		fmt.Printf("%q\n", name)
		if *flagList {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		os.MkdirAll(filepath.Dir(name), 0755)
		dest, err := os.Create(name)
		if err != nil {
			rc.Close()
			log.Printf("create %q: %v", name, err)
			continue
		}
		_, err = io.Copy(dest, rc)
		rc.Close()
		if closeErr := dest.Close(); closeErr != nil && err == nil {
			log.Printf("Close %q: %v", dest.Name(), err)
		}
	}
}
