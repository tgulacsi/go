// Copyright (c) 2014, Tamás Gulácsi
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Package main is a program which concatenates a zipfile to another file (probably exe)
// and fixes the zip header to have the end result be usable both
// as an executable and a zipfile.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/tgulacsi/go/zipA/azip"
)

func main() {
	flagOutput := flag.String("o", "-", "output (default: stdout)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: "+os.Args[0]+" <zipfile> <exefile>")
	}
	flag.Parse()

	exeFn := flag.Arg(1)
	exe, err := os.Open(exeFn)
	if err != nil {
		log.Fatalf("cannot open exe %q: %v", exeFn, err)
	}
	defer exe.Close()

	var out io.Writer
	var tempFh *os.File
	if *flagOutput == "-" || *flagOutput == "" {
		out = os.Stdout
	} else {
		tempFh, err = ioutil.TempFile("", "zipA-"+filepath.Base(exe.Name())+"-")
		if err != nil {
			log.Fatalf("cannot create tempfile: %v", err)
		}
		out = tempFh
	}

	n, err := io.Copy(out, exe)
	if err != nil {
		log.Fatalf("error copying %q to %q: %v", exe.Name(), tempFh.Name(), err)
	}

	zw := azip.NewZipWriterOffset(out, n)
	defer zw.Close()

	if err = azip.AppendZipFile(zw, flag.Arg(0)); err != nil {
		log.Fatalf("AppendZipFile: %v", err)
	}
	if err = zw.Close(); err != nil {
		log.Printf("error closing zip (first round): %v", err)
	}
	if tempFh == nil {
		return
	}
	if err = tempFh.Sync(); err != nil {
		log.Printf("error syncing tempfile %q: %v", tempFh.Name(), err)
		return
	}
	defer tempFh.Close()

	if _, err = tempFh.Seek(0, 0); err != nil {
		log.Fatalf("error seeking in %q: %v", tempFh, err)
	}
	dest, err := os.Create(*flagOutput)
	if err != nil {
		log.Fatalf("cannot create %q: %v", dest, err)
	}
	_, err1 := io.Copy(dest, tempFh)
	err = dest.Close()
	if err1 != nil {
		err = err1
	}
	if err != nil {
		log.Fatalf("error writing %q: %v", dest, err)
	}
	exeStat, err := exe.Stat()
	if err != nil {
		log.Fatal("cannot stat %q: %v", exe.Name(), err)
	}
	if err = os.Chmod(*flagOutput, exeStat.Mode()); err != nil {
		log.Printf("error with chmod %q: %v", *flagOutput, err)
	}
}
