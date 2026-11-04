// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package bash

import (
	"bufio"
	"io"
	"strings"
)

func Unquote(s string) string {
	if len(s) < 2 {
		return s
	} else if s[0] == '\'' && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	s = s[1 : len(s)-1]
	var b strings.Builder
	b.Grow(len(s))

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

func QuoteWrite(w io.Writer, s string) error {
	if !strings.ContainsAny(s, "\\ \"$`") {
		_, err := io.WriteString(w, s)
		return err
	}
	bw := bufio.NewWriterSize(w, len(s)+2)
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
	for i, s := range ss {
		if i != 0 {
			b.WriteByte(' ')
		}
		QuoteWrite(&b, s)
	}
	return b.String()
}
