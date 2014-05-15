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

	"code.google.com/p/go.text/encoding"
	"code.google.com/p/go.text/encoding/charmap"
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
			return nil
		}
		if i, err = strconv.Atoi(name[i+1:]); err != nil {
			return nil
		}
		switch i {
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
	case "cp437":
		return charmap.CodePage437
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
