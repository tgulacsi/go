// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package bash

import (
	"bufio"
	// "fmt"
	"io"
	"strings"
)

func Unquote(s string) string {
	// fmt.Printf("Unqote(%q)\n", s)
	if len(s) < 2 {
		return s
	} else if s[0] == '\'' && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	} else if !(s[0] == '"' && s[len(s)-1] == s[0]) {
		// fmt.Printf("SKIP[%c%c] %q\n", s[0], s[len(s)-1], s)
		return s
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	b.Grow(len(s))

	// fmt.Println("Unqote", s)
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\', '"', '$', '`':
				i++
				b.WriteByte(s[i])
				continue
			case '\n':
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}

	return b.String()
}

func Quote(s string) string {
	if !strings.ContainsAny(s, "\\ \"$`") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	_ = QuoteWrite(&b, s)
	return b.String()
}

func QuoteWrite(w io.Writer, ss ...string) error {
	var n int
	for _, s := range ss {
		n += 1 + len(s) + 2
	}
	bw := bufio.NewWriterSize(w, n)
	for i, s := range ss {
		if i != 0 {
			if err := bw.WriteByte(' '); err != nil {
				return err
			}
		}
		if err := quoteWriteOne(bw, s); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func quoteWriteOne(bw *bufio.Writer, s string) error {
	if !strings.ContainsAny(s, "\\ \"$`") {
		_, err := bw.WriteString(s)
		return err
	}
	if err := bw.WriteByte('"'); err != nil {
		return err
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', '$', '`':
			if err := bw.WriteByte('\\'); err != nil {
				return err
			}
		}
		if err := bw.WriteByte(s[i]); err != nil {
			return err
		}
	}
	return bw.WriteByte('"')
}

func QuotePrint(ss []string) string {
	var b strings.Builder
	_ = QuoteWrite(&b, ss...)
	return b.String()
}
