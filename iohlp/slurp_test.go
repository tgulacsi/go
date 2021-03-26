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
	"time"
)

func TestMakeSectionReader(t *testing.T) {
	const want = "abraca dabra"
	rat, err := MakeSectionReader(strings.NewReader(want), 3)
	if err != nil {
		t.Fatal(err)
	}
	gotB := make([]byte, len(want))
	if n, err := rat.ReadAt(gotB, 0); err != nil {
		t.Fatal(err)
	} else if n != len(want) {
		t.Errorf("got %d, wanted %d", n, len(want))
	}
	got := string(gotB)
	if got != want {
		t.Errorf("got %q, wanted %q", got, want)
	}
	runtime.GC()
	g := got[1:2]
	got, gotB = "", nil
	t.Log("GC didn't panic", g)
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
}

func TestReadALot(t *testing.T) {
	const N = 128 << 20

	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	{
		rat, err := MakeSectionReader(&dummyReader{N: N}, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		b := make([]byte, N)
		if n, err := rat.ReadAt(b, 0); err != nil {
			t.Fatal(err)
		} else if n != cap(b) {
			t.Errorf("wanted %d, read %d", cap(b), n)
		}
		t.Logf("Read %d bytes", len(b))
		runtime.ReadMemStats(&m2)
		t.Logf("One big read consumed\t%d bytes", m2.Sys-m1.Sys)
		sb := b[1:2]
		b = nil
		t.Logf("sb: %q", sb)
	}
	runtime.GC()
	runtime.ReadMemStats(&m2)
	t.Logf("One big read after GC:\t%d bytes", m2.Sys-m1.Sys)
}

type dummyReader struct {
	scratch []byte
	N       int64
	i       uint8
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
