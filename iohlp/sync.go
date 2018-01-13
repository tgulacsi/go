/*
Copyright 2016 Tamás Gulácsi

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
	"sync"
)

// ConcurrentReader wraps the given io.Reader such that it can be called concurrently.
func ConcurrentReader(r io.Reader) io.Reader {
	return &concurrentReader{r: r}
}

type concurrentReader struct {
	mu sync.Mutex
	r  io.Reader
}

func (cr *concurrentReader) Read(p []byte) (int, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.r.Read(p)
}
