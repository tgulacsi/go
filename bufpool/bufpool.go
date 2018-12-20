/*
  Copyright 2018 Tamás Gulácsi

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

package bufpool

import (
	"bytes"
	"strings"
	"sync"
)

var Default = New(DefaultSize)

const DefaultSize = 8192

func New(size int) *bufferPool {
	if size == 0 {
		size = DefaultSize
	}
	return &bufferPool{Pool: &sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, size)) }}}
}

func Get() *bytes.Buffer    { return Default.Get() }
func Put(buf *bytes.Buffer) { Default.Put(buf) }

type Pool interface {
	Get() *bytes.Buffer
	Put(*bytes.Buffer)
}

type bufferPool struct {
	Pool *sync.Pool
}

func (p *bufferPool) Get() *bytes.Buffer {
	buf := p.Pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}
func (p *bufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	p.Pool.Put(buf)
}

func NewStrings() *stringsPool {
	return &stringsPool{Pool: &sync.Pool{New: func() interface{} { return &strings.Builder{} }}}
}

var DefaultStrings = NewStrings()

type BuilderPool interface {
	Get() *strings.Builder
	Put(*strings.Builder)
}

func GetBuilder() *strings.Builder        { return DefaultStrings.Get() }
func PutBuilder(builder *strings.Builder) { DefaultStrings.Put(builder) }

type stringsPool struct {
	Pool *sync.Pool
}

func (p *stringsPool) Get() *strings.Builder {
	buf := p.Pool.Get().(*strings.Builder)
	buf.Reset()
	return buf
}
func (p *stringsPool) Put(buf *strings.Builder) {
	if buf == nil {
		return
	}
	buf.Reset()
	p.Pool.Put(buf)
}
