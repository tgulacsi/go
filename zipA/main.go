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
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func main() {
	flagOutput := flag.String("o", "-", "output (default: stdout)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: "+os.Args[0]+" <zipfile> <exefile>")
	}
	flag.Parse()

	zr, err := zip.OpenReader(flag.Arg(0))
	if err != nil {
		log.Fatalf("cannot open source zip %q: %v", flag.Arg(0), err)
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		return
	}

	// The first element in the zip starts at "beginning".
	// We have to create a file where the first exeStat.Size() bytes
	// are the exe, and then comes the zip - but that zip must contain files
	// with offsets shifted with

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

	zw := zip.NewWriter(out)
	defer zw.Close()
	setCountOfWriter(zw, n)

	// copy the zip's contents
	for _, f := range zr.File {
		w, err := zw.CreateHeader(&f.FileHeader)
		if err != nil {
			log.Printf("error creating header for %q: %v", f.FileHeader.Name, err)
			continue
		}
		src, err := f.Open()
		if err != nil {
			log.Printf("error opening %s: %v", f.Name)
			continue
		}
		// We always get "zip: checksum error", but the resulting file is OK.
		if _, err = io.Copy(w, src); err != nil && !strings.HasSuffix(err.Error(), "checksum error") {
			src.Close()
			log.Printf("error copying from %s: %v", f.Name, err)
		}
		src.Close()
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

// These are dirty hacks - to work it assumes that the archive/zip 's
// structures looks like these.

// localFile is a copy of archive/zip 's File.
type localFile struct {
	zip.FileHeader
	zipr         io.ReaderAt
	zipsize      int64
	headerOffset int64
}

// getHeaderOffset returns the header offset of the file (only data offset is published).
func getHeaderOffset(f *zip.File) int64 {
	return (*localFile)(unsafe.Pointer(f)).headerOffset
}

// localWriter is an even dirtier hack: it assmes that the archive/zip 's
// Writer struct's first element is an *zip.countWriter.
type localWriter struct {
	cw *localCountWriter
}

// localCountWriter is the copy of the archive/zip 's countWriter
type localCountWriter struct {
	w     io.Writer
	count int64
}

// setCountOfWriter sets the zip.Writer's underlying countWriter's count.
// Returns the previously set count.
func setCountOfWriter(w *zip.Writer, count int64) int64 {
	cw := ((*localWriter)(unsafe.Pointer(w))).cw
	oldCount := cw.count
	cw.count = count
	return oldCount
}
