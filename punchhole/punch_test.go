/*
Copyright 2014 Tamás Gulácsi.

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

package punchhole

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestPunch(t *testing.T) {
	if PunchHole == nil {
		t.Logf("No punchHole implementation is available, skipping.")
		t.Skip()
	}
	file, err := ioutil.TempFile("", "punchhole-")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	buf := make([]byte, 10<<20)
	for i := range buf {
		buf[i] = byte(1 + (i+1)&0xfe)
	}
	if _, err = file.Write(buf); err != nil {
		t.Errorf("error writing to the temp file: %v", err)
		t.FailNow()
	}
	if err = file.Sync(); err != nil {
		t.Logf("error syncing %q: %v", file.Name(), err)
	}
	for i, j := range []int{1, 31, 1 << 10} {
		if err = PunchHole(file, int64(j), int64(j)); err != nil {
			t.Errorf("%d. error punching at %d, size %d: %v", i, j, j, err)
			continue
		}
		// read back, with 1-1 bytes overlaid
		n, err := file.ReadAt(buf[:j+2], int64(j-1))
		if err != nil {
			t.Errorf("%d. error reading file: %v", i, err)
			continue
		}
		buf = buf[:n]
		if buf[0] == 0 {
			t.Errorf("%d. file at %d has been overwritten with 0!", i, j-1)
		}
		if buf[n-1] == 0 {
			t.Errorf("%d. file at %d has been overwritten with 0!", i, j-1+n)
		}
		for k, v := range buf[1 : n-1] {
			if v != 0 {
				t.Errorf("%d. error reading file at %d got %d, want 0.", i, k, v)
			}
		}
	}
}
