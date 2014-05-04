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
	"fmt"
	"io"
	"io/ioutil"
	"testing"
)

func TestZRRead(t *testing.T) {
	for i, j := range []int{1, 3, 1<<21 - 1} {
		buf := make([]byte, j+1)
		for i := range buf {
			buf[i] = byte((i + 1) & 0xff)
		}
		n, err := (&zeroReader{int64(j)}).Read(buf)
		if err != nil {
			t.Errorf("%d. %v", i, err)
		}
		if n != j {
			t.Errorf("%d. size mismatch: got %d awaited %d.", n, j)
		}
		for k, v := range buf[:n] {
			if v != 0 {
				t.Errorf("%d. not zero (%d) at %d.", i, v, k)
			}
		}
	}
}

func BenchmarkZRWriteTo(t *testing.B) {
	t.StopTimer()
	length := int64(t.N)
	zr := &zeroReader{length}
	cw := &countWriter{Writer: ioutil.Discard}
	t.StartTimer()
	n, err := io.Copy(cw, zr)
	t.StopTimer()
	t.SetBytes(n)
	if err != nil {
		t.Error(err)
	}
	if n != length {
		t.Errorf("written %d, wanted %d", n, length)
	}
}

type countWriter struct {
	io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	for i, v := range p {
		if v != 0 {
			return i, fmt.Errorf("non-zero byte (%d) at %d", v, i)
		}
	}
	n, err := cw.Writer.Write(p)
	cw.n += int64(n)
	return n, err
}
