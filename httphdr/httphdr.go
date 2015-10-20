/*
copyright 2015 tamás gulácsi

licensed under the apache license, version 2.0 (the "license");
you may not use this file except in compliance with the license.
you may obtain a copy of the license at

     http://www.apache.org/licenses/license-2.0

unless required by applicable law or agreed to in writing, software
distributed under the license is distributed on an "as is" basis,
without warranties or conditions of any kind, either express or implied.
see the license for the specific language governing permissions and
limitations under the license.
*/

// Package httphdr provides some support for HTTP headers.
package httphdr

import (
	"bytes"
	"fmt"
	"sync"
)

var bPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 128)) }}

// ContentDisposition returns a formatted http://tools.ietf.org/html/rfc6266 header.
func ContentDisposition(dispType string, filename string) string {
	b := bPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		bPool.Put(b)
	}()
	b.WriteString(dispType)
	b.WriteString(` filename="`)
	justLatin := true
	start := b.Len()
	for _, r := range filename {
		if r < 0x80 {
			b.WriteByte(byte(r))
		} else {
			fmt.Fprintf(b, "%%%x", r)
			justLatin = false
		}
	}
	if justLatin {
		b.WriteByte('"')
		return b.String()
	}
	encoded := b.Bytes()[start:]
	b.WriteString(`" filename*=utf-8''`)
	b.Write(encoded)
	return b.String()
}
