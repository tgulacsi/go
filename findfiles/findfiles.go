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
	"archive/tar"
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
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

	var foundCh chan fileInfo
	var wg sync.WaitGroup
	if *flagTar != "" {
		var tfh *os.File
		fn := *flagTar
		if fn == "-" {
			tfh = os.Stdout
		} else {
			if tfh, err = os.Create(*flagTar); err != nil {
				log.Fatalf("cannot open tar output %q: %s", *flagTar, err)
			}
		}
		defer tfh.Close()
		tw := tar.NewWriter(tfh)
		defer tw.Close()

		foundCh = make(chan fileInfo, 16)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range foundCh {
				if err = addFile(tw, info); err != nil {
					log.Fatalf("%s", err)
				}
			}
		}()
	}

	w := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Mode()&os.ModeType > 0 {
			return nil
		}
		bn := filepath.Base(path)
		if old, ok := needles[bn]; ok {
			if old != "" {
				log.Printf("found AGAIN %q at %q", bn, path)
				return nil
			}
			log.Printf("found %q at %q", bn, path)
			needles[bn] = path
			if foundCh != nil {
				foundCh <- fileInfo{FileInfo: info, Path: path}
			}
		}
		return nil
	}
	for _, root := range flag.Args()[1:] {
		if err = filepath.Walk(root, w); err != nil {
			log.Fatalf("error walking %q: %s", root, err)
		}
	}

	if foundCh != nil {
		close(foundCh)
		log.Printf("waiting tar to finish...")
		wg.Wait()
		log.Printf("tar done")
	}
}

type fileInfo struct {
	os.FileInfo
	Path string
}

func addFile(tw *tar.Writer, info fileInfo) error {
	fh, err := os.Open(info.Path)
	if err != nil {
		return fmt.Errorf("error opening %q: %s", info.Path, err)
	}
	defer fh.Close()
	hdr, err := tar.FileInfoHeader(info.FileInfo, "")
	if err != nil {
		return fmt.Errorf("error creating tar header: %s", err)
	}
	if err = tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("error writing tar header: %s", err)
	}
	if _, err = io.Copy(tw, fh); err != nil {
		return fmt.Errorf("error writing file %q: %s", info.Path, err)
	}
	return nil
}
