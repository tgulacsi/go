/*
Copyright 2013 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package main of findfiles is a program for finding files listed in a
\n-separated text file given as first argument.
Every other argument is treated as a search root.
*/
package main

import (
	"bufio"
	"flag"
	"log"
    "path/filepath"
	"os"
)

var (
	flagTar = flag.String("tar", "", "tar output")
)

func main() {
	//flag.Usage = `findfiles [options] listOfNeedles.txt searchroot [searchroot ...]`
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatalf("at least two args are needed: list of needles and search root")
	}
	fh, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("error opening list of needles %q: %s", flag.Arg(0), err)
	}
	defer fh.Close()

	var needles = make(map[string]string, 1000)
	scan := bufio.NewScanner(fh)
	scan.Split(bufio.ScanLines)
	for scan.Scan() {
		needles[string(scan.Bytes())] = ""
	}
	if err = scan.Err(); err != nil {
		log.Fatalf("error reading lines of %q: %s", fh.Name(), err)
	}

	log.Printf("start searching for %d needles", len(needles))

	w := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Mode()&os.ModeType > 0 {
			return nil
		}
		bn := filepath.Base(path)
		if _, ok := needles[bn]; ok {
			log.Printf("found %q at %q", bn, path)
			needles[bn] = path
		}
		return nil
	}
	for _, root := range flag.Args()[1:] {
		if err = filepath.Walk(root, w); err != nil {
			log.Fatalf("errof walking %q: %s", root, err)
		}
	}
}
