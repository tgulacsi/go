package utf7

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/transform"
)

type encoder struct{ flavor }

func (encoder) Reset() {}

// Encode encodes a string with modified UTF-7.
func (e encoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for i := 0; i < len(src); {
		ch := src[i]

		var b []byte
		if min <= ch && ch <= max {
			b = []byte{ch}
			if ch == e.shiftChar {
				b = append(b, '-')
			}

			i++
		} else {
			start := i

			// Find the next printable ASCII code point
			i++
			for i < len(src) && (src[i] < min || src[i] > max) {
				i++
			}

			if !atEOF && i == len(src) {
				err = transform.ErrShortSrc
				return
			}

			b = e.encode(src[start:i])
		}

		if nDst+len(b) > len(dst) {
			err = transform.ErrShortDst
			return
		}

		nSrc = i

		for _, ch := range b {
			dst[nDst] = ch
			nDst++
		}
	}

	return
}

// Converts string s from UTF-8 to UTF-16-BE, encodes the result as base64,
// removes the padding, and adds UTF-7 shifts.
func (F flavor) encode(s []byte) []byte {
	// len(s) is sufficient for UTF-8 to UTF-16 conversion if there are no
	// control code points (see table below).
	b := make([]byte, 0, len(s)+4)
	for len(s) > 0 {
		r, size := utf8.DecodeRune(s)
		if r > utf8.MaxRune {
			r, size = utf8.RuneError, 1 // Bug fix (issue 3785)
		}
		s = s[size:]
		if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
			b = append(b, byte(r1>>8), byte(r1))
			r = r2
		}
		b = append(b, byte(r>>8), byte(r))
	}

	// Encode as base64
	n := F.b64Enc.EncodedLen(len(b)) + 2
	b64 := make([]byte, n)
	F.b64Enc.Encode(b64[1:], b)

	// Strip padding
	n -= 2 - (len(b)+2)%3
	b64 = b64[:n]

	// Add UTF-7 shifts
	b64[0] = F.shiftChar
	b64[n-1] = '-'
	return b64
}

// Escape passes through raw UTF-8 as-is and escapes the special UTF-7 marker
// (the ampersand character).
func (F flavor) Escape(src string) string {
	var sb strings.Builder
	sb.Grow(len(src))

	shiftChar := rune(F.shiftChar)
	for _, ch := range src {
		sb.WriteRune(ch)
		if ch == shiftChar {
			sb.WriteByte('-')
		}
	}

	return sb.String()
}
