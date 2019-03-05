/*
Copyright 2019 Tamás Gulácsi

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

package iohlp

import (
	"io"
	"runtime"
	"strings"
	"testing"
)

func TestReadAll(t *testing.T) {
	const s = "abraca dabra"
	b, closer, err := ReadAll(strings.NewReader(s), 3)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	if string(b) != s {
		t.Errorf("got %q, wanted %q", string(b), s)
	}
}

func TestReadALot(t *testing.T) {
	const N = 128

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	{
		b, err := ReadAll(&dummyReader{N: N << 20}, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Read %d bytes", len(b))
		runtime.ReadMemStats(&m2)
		t.Logf("One big read consumed\t%d bytes", m2.Sys-m1.Sys)
	}
	runtime.GC()
	runtime.ReadMemStats(&m2)
	t.Logf("One big read after GC:\t%d bytes", m2.Sys-m1.Sys)
}

type dummyReader struct {
	N       int64
	i       uint8
	scratch []byte
}

func (r *dummyReader) Read(p []byte) (int, error) {
	if r.N <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.N {
		n = int(r.N)
	}
	if cap(r.scratch) < n {
		r.scratch = make([]byte, n)
		for i := 0; i < n; i++ {
			r.scratch[i] = r.i
			r.i++
		}
	}
	r.N -= int64(n)
	copy(p[:n], r.scratch)
	if r.N <= 0 {
		return n, io.EOF
	}
	return n, nil
}
