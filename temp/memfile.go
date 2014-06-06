/*
Copyright 2013 the Camlistore authors.

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

package temp

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"

	"camlistore.org/pkg/types"
)

const maxInMemorySlurp = 4 << 20 // 4MB.  *shrug*

// MakeReadSeekCloser makes an io.ReadSeeker + io.Closer by reading the whole reader
// If the given Reader is a Closer, too, than that Close will be called
func MakeReadSeekCloser(blobRef string, r io.Reader) (types.ReadSeekCloser, error) {
	if rc, ok := r.(io.Closer); ok {
		defer rc.Close()
	}
	ms := NewMemorySlurper(blobRef)
	_, err := io.Copy(ms, r)
	if err != nil {
		return nil, err
	}
	return ms, nil
}

// memorySlurper slurps up a blob to memory (or spilling to disk if
// over maxInMemorySlurp) and deletes the file on Close
type memorySlurper struct {
	blobRef string // only used for tempfile's prefix
	buf     *bytes.Buffer
	mem     *bytes.Reader
	file    *os.File // nil until allocated
	reading bool     // transitions at most once from false -> true
}

func NewMemorySlurper(blobRef string) *memorySlurper {
	return &memorySlurper{
		blobRef: blobRef,
		buf:     new(bytes.Buffer),
	}
}

func (ms *memorySlurper) Read(p []byte) (n int, err error) {
	if !ms.reading {
		ms.reading = true
		if ms.file != nil {
			ms.file.Seek(0, 0)
		} else {
			ms.mem = bytes.NewReader(ms.buf.Bytes())
			ms.buf = nil
		}
	}
	if ms.file != nil {
		return ms.file.Read(p)
	}
	return ms.mem.Read(p)
}

func (ms *memorySlurper) Seek(offset int64, whence int) (int64, error) {
	if !ms.reading {
		ms.reading = true
		if ms.file == nil {
			ms.mem = bytes.NewReader(ms.buf.Bytes())
			ms.buf = nil
		}
	}
	if ms.file != nil {
		return ms.file.Seek(offset, whence)
	}
	return ms.mem.Seek(offset, whence)
}

func (ms *memorySlurper) Write(p []byte) (n int, err error) {
	if ms.reading {
		panic("write after read")
	}
	if ms.file != nil {
		n, err = ms.file.Write(p)
		return
	}

	if ms.buf.Len()+len(p) > maxInMemorySlurp {
		ms.file, err = ioutil.TempFile("", ms.blobRef)
		if err != nil {
			return
		}
		_, err = io.Copy(ms.file, ms.buf)
		if err != nil {
			return
		}
		ms.buf = nil
		n, err = ms.file.Write(p)
		return
	}

	return ms.buf.Write(p)
}

func (ms *memorySlurper) Cleanup() error {
	if ms.file != nil {
		return os.Remove(ms.file.Name())
	}
	return nil
}

func (ms *memorySlurper) Close() error {
	return ms.Cleanup()
}
