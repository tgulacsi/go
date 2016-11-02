/*
Copyright 2013 Tamás Gulácsi

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

package text

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// GetEncoding returns the encoding.Encoding for the text name of the encoding
// Returns nil if the encoding is not found.
//
// Knows the ISO8859 family, KOI8, Windows and Mac codepages,
// but misses other Asian codepages
func GetEncoding(name string) encoding.Encoding {
	name = strings.ToLower(strings.TrimSpace(name))
	var err error
	if strings.HasPrefix(name, "iso") {
		i := strings.LastIndex(name, "-")
		if i < 0 {
			if len(name) != len("iso88591") {
				return nil
			}
			i = len(name) - 2
		}
		if i, err = strconv.Atoi(name[i+1:]); err != nil {
			return nil
		}
		switch i {
		case 1:
			return ISO8859_1
		case 2:
			return charmap.ISO8859_2
		case 3:
			return charmap.ISO8859_3
		case 4:
			return charmap.ISO8859_4
		case 5:
			return charmap.ISO8859_5
		case 6:
			return charmap.ISO8859_6
		case 7:
			return charmap.ISO8859_7
		case 8:
			return charmap.ISO8859_8
		case 10:
			return charmap.ISO8859_10
		case 13:
			return charmap.ISO8859_13
		case 14:
			return charmap.ISO8859_14
		case 15:
			return charmap.ISO8859_15
		case 16:
			return charmap.ISO8859_16
		}
		return nil
	}
	switch strings.Replace(name, "-", "", -1) {
	case "utf8":
		return encoding.Replacement
	case "cp437":
		return charmap.CodePage437
	case "cp850":
		return charmap.CodePage850
	case "cp852":
		return charmap.CodePage852
	case "cp855":
		return charmap.CodePage855
	case "cp858":
		return charmap.CodePage858
	case "cp862":
		return charmap.CodePage862
	case "cp866":
		return charmap.CodePage866
	case "koi8r":
		return charmap.KOI8R
	case "koi8u":
		return charmap.KOI8U
	case "mac", "macintosh":
		return charmap.Macintosh
	case "macCyrillic", "macintoshCyrillic":
		return charmap.MacintoshCyrillic
	case "windows1250", "win1250":
		return charmap.Windows1250
	case "windows1251", "win1251":
		return charmap.Windows1251
	case "windows1252", "win1252":
		return charmap.Windows1252
	case "windows1253", "win1253":
		return charmap.Windows1253
	case "windows1254", "win1254":
		return charmap.Windows1254
	case "windows1255", "win1255":
		return charmap.Windows1255
	case "windows1256", "win1256":
		return charmap.Windows1256
	case "windows1257", "win1257":
		return charmap.Windows1257
	case "windows1258", "win1258":
		return charmap.Windows1258
	case "windows874", "win874":
		return charmap.Windows874
	}
	return nil
}

// ISO8859_1 is the Encoding for ISO-8859-1
var ISO8859_1 encoding.Encoding = cmap("ISO8859-1")

type cmap string

func (m cmap) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: fromISO8859_1{}}
}

func (m cmap) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: toISO8859_1{}}
}

type fromISO8859_1 struct {
	transform.NopResetter
}
type toISO8859_1 struct {
	transform.NopResetter
}

func (m fromISO8859_1) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	var b [4]byte
	for i, c := range src {
		n := utf8.EncodeRune(b[:], rune(c))
		if nDst+n > len(dst) {
			err = transform.ErrShortDst
			break
		}
		nSrc = i + 1
		copy(dst[nDst:], b[:n])
		nDst += n
	}
	return nDst, nSrc, err
}

func (m toISO8859_1) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		r, n := utf8.DecodeRune(src[nSrc:])
		if n == 0 { //EOF
			break
		}
		if r == utf8.RuneError {
			err = encoding.ErrInvalidUTF8
			break
		}
		if r > 0xff {
			r = '?'
		}
		if nDst+1 > len(dst) {
			err = transform.ErrShortDst
			break
		}
		nSrc += n
		dst[nDst] = byte(r)
		nDst++
	}

	return nDst, nSrc, err
}
