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
Package loffice for calling loffice, for example for converting files to PDF
*/
package loffice

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tgulacsi/go/proc"
)

// Log is discarded by default
var Log = func(...interface{}) error { return nil }

// loffice executable name
var Loffice = "loffice"

// child timeout, in seconds
var Timeout = 300

// Convert converts from srcFn to dstFn, into the given format.
// Either filenames can be empty or "-" which treated as stdin/stdout
func Convert(srcFn, dstFn, format string) error {
	tempDir, err := ioutil.TempDir("", filepath.Base(srcFn))
	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %s", err)
	}
	defer os.RemoveAll(tempDir)
	if srcFn == "-" || srcFn == "" {
		srcFn = filepath.Join(tempDir, "source")
		fh, err := os.Create(srcFn)
		if err != nil {
			return fmt.Errorf("error creating temp file %q: %s", srcFn, err)
		}
		if _, err = io.Copy(fh, os.Stdin); err != nil {
			fh.Close()
			return fmt.Errorf("error writing stdout to %q: %s", srcFn, err)
		}
		fh.Close()
	}
	c := exec.Command(Loffice, "--nolockcheck", "--norestore", "--headless",
		"--convert-to", format, "--outdir", tempDir, srcFn)
	c.Stderr = os.Stderr
	c.Stdout = c.Stderr
	Log("msg", "calling", "args", c.Args)
	if err = proc.RunWithTimeout(Timeout, c); err != nil {
		return fmt.Errorf("error running %q: %s", c.Args, err)
	}
	dh, err := os.Open(tempDir)
	if err != nil {
		return fmt.Errorf("error opening dest dir %q: %s", tempDir, err)
	}
	defer dh.Close()
	names, err := dh.Readdirnames(3)
	if err != nil {
		return fmt.Errorf("error listing %q: %s", tempDir, err)
	}
	if len(names) > 2 {
		return fmt.Errorf("too many files in %q: %q", tempDir, names)
	}
	var tfn string
	for _, fn := range names {
		if fn != "source" {
			tfn = filepath.Join(dh.Name(), fn)
			break
		}
	}
	src, err := os.Open(tfn)
	if err != nil {
		return fmt.Errorf("cannot open %q: %s", tfn, err)
	}
	defer src.Close()
	var dst = io.WriteCloser(os.Stdout)
	if !(dstFn == "-" || dstFn == "") {
		if dst, err = os.Create(dstFn); err != nil {
			return fmt.Errorf("cannot create dest file %q: %s", dstFn, err)
		}
	}
	if _, err = io.Copy(dst, src); err != nil {
		return fmt.Errorf("error copying from %v to %v: %v", src, dst, err)
	}
	return nil
}
