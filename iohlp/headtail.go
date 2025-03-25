// Copyright 2024 Tamas Gulacsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp

import (
	"fmt"
	"hash"
	"hash/fnv"
	"strings"
)

// HeadTailKeeper is an io.Writer which keeps only Limit bytes from the start (head),
// and Limit bytes from the end (tail).
type HeadTailKeeper struct {
	Limit      int
	head, tail []byte
	hsh        hash.Hash64
}

func (htw *HeadTailKeeper) Write(p []byte) (int, error) {
	if htw.hsh == nil {
		htw.hsh = fnv.New64()
	}
	htw.hsh.Write(p)
	length := len(p)
	if rem := htw.Limit - len(htw.head); rem > 0 {
		m := len(p)
		if m > rem {
			m = rem
		}
		htw.head = append(htw.head, p[:m]...)
		p = p[m:]
	}
	if len(p) == 0 {
		return length, nil
	}
	if rem := len(p) - htw.Limit; rem >= 0 {
		htw.tail = append(htw.tail[:0], p[rem:]...)
	} else {
		htw.tail = append(htw.tail, p...)
		if len(htw.tail) > htw.Limit {
			htw.tail = htw.tail[len(htw.tail)-htw.Limit:]
		}
	}
	return length, nil
}
func (htw *HeadTailKeeper) Reset() {
	htw.head = htw.head[:0]
	htw.tail = htw.tail[:0]
	if htw.hsh != nil {
		htw.hsh.Reset()
	}
}
func (htw *HeadTailKeeper) Head() []byte  { return htw.head }
func (htw *HeadTailKeeper) Tail() []byte  { return htw.tail }
func (htw *HeadTailKeeper) Sum64() uint64 { return htw.hsh.Sum64() }
func (htw *HeadTailKeeper) String() string {
	if len(htw.tail) == 0 {
		return string(htw.head)
	}
	var buf strings.Builder
	buf.Grow(len(htw.head) + 3 + 19 + 3 + len(htw.tail))
	buf.Write(htw.head)
	buf.WriteString("...")
	if htw.hsh != nil {
		fmt.Fprintf(&buf, "%x", htw.hsh.Sum64())
		buf.WriteString("...")
	}
	buf.Write(htw.tail)
	return buf.String()
}
