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
	"io/ioutil"
	"os"
	"os/exec"

	//"github.com/golang/glog"

	"github.com/tgulacsi/go/proc"
)

// loffice executable name
var Loffice = "loffice"

// child timeout, in seconds
var Timeout = 300

// Convert converts from srcFn to dstFn, into the given format.
// Eithe filenames can be empty or "-" which treated as stdin/stdout
func Convert(srcFn, dstFn, format string) error {
	tempDir, err := ioutil.TempDir("", srcFn)
	if err != nil {
		return err
	}
	c := exec.Command(Loffice, "--nolockcheck", "--norestore", "--headless",
		"--convert-to", format, "--outdir", tempDir)
	c.Stderr = os.Stderr
	if dstFn == "" || dstFn == "-" {
		c.Stdout = os.Stdout
	} else {
		var err error
		if c.Stdout, err = os.Create(dstFn); err != nil {
			return err
		}
	}
	return proc.RunWithTimeout(Timeout, c)
}
