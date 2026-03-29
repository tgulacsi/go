// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpcb

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"time"
)

// Copied from https://github.com/go-json-experiment/json/blob/d219187c3433/arshal_time.go

// daysPerYear is the exact average number of days in a year according to
// the Gregorian calendar, which has an extra day each year that is
// a multiple of 4, unless it is evenly divisible by 100 but not by 400.
// This does not take into account leap seconds, which are not deterministic.
const daysPerYear = 365.2425

var errInaccurateDateUnits = errors.New("inaccurate year, month, week, or day units")

// ParseDurationISO8601 parses a duration according to ISO 8601-1:2019,
// section 5.5.2.2 and 5.5.2.3 with the following restrictions or extensions:
//
//   - A leading minus sign is permitted for negative duration according
//     to ISO 8601-2:2019, section 4.4.1.9. We do not permit negative values
//     for each "time scale component", which is permitted by section 4.4.1.1,
//     but rarely supported by parsers.
//
//   - A leading plus sign is permitted (and ignored).
//     This is not required by ISO 8601, but not forbidden either.
//     There is some precedent for this as it is supported by the principle of
//     duration arithmetic as specified in ISO 8601-2-2019, section 14.1.
//     Of note, the JavaScript grammar for ISO 8601 permits a leading plus sign.
//
//   - A fractional value is only permitted for accurate units
//     (i.e., hour, minute, and seconds) in the last time component,
//     which is permissible by ISO 8601-1:2019, section 5.5.2.3.
//
//   - Both periods ('.') and commas (',') are supported as the separator
//     between the integer part and fraction part of a number,
//     as specified in ISO 8601-1:2019, section 3.2.6.
//     While ISO 8601 recommends comma as the default separator,
//     most formatters uses a period.
//
//   - Leading zeros are ignored. This is not required by ISO 8601,
//     but also not forbidden by the standard. Many parsers support this.
//
//   - Lowercase designators are supported. This is not required by ISO 8601,
//     but also not forbidden by the standard. Many parsers support this.
//
// If the nominal units of year, month, week, or day are present,
// this produces a best-effort value and also reports [errInaccurateDateUnits].
//
// The accepted grammar is identical to JavaScript's Duration:
//
//	https://tc39.es/proposal-temporal/#prod-Duration
//
// We follow JavaScript's grammar as JSON itself is derived from JavaScript.
// The Temporal.Duration.toJSON method is guaranteed to produce an output
// that can be parsed by this function so long as arithmetic in JavaScript
// do not use a largestUnit value higher than "hours" (which is the default).
// Even if it does, this will do a best-effort parsing with inaccurate units,
// but report [errInaccurateDateUnits].
func ParseDurationISO8601(b []byte) (time.Duration, error) {
	var invalid, overflow, inaccurate, sawFrac bool
	var sumNanos, n, co uint64

	// cutBytes is like [bytes.Cut], but uses either c0 or c1 as the separator.
	cutBytes := func(b []byte, c0, c1 byte) (prefix, suffix []byte, ok bool) {
		for i, c := range b {
			if c == c0 || c == c1 {
				return b[:i], b[i+1:], true
			}
		}
		return b, nil, false
	}

	// mayParseUnit attempts to parse another date or time number
	// identified by the desHi and desLo unit characters.
	// If the part is absent for current unit, it returns b as is.
	mayParseUnit := func(b []byte, desHi, desLo byte, unit time.Duration) []byte {
		number, suffix, ok := cutBytes(b, desHi, desLo)
		if !ok || sawFrac {
			return b // designator is not present or already saw fraction, which can only be in the last component
		}

		// Parse the number.
		// A fraction allowed for the accurate units in the last part.
		whole, frac, ok := cutBytes(number, '.', ',')
		if ok {
			sawFrac = true
			invalid = invalid || len(frac) == len("") || unit > time.Hour
			if unit == time.Second {
				n, ok = parsePaddedBase10(frac, uint64(time.Second))
				invalid = invalid || !ok
			} else {
				f, err := strconv.ParseFloat("0."+string(frac), 64)
				invalid = invalid || err != nil || len(bytes.Trim(frac[len("."):], "0123456789")) > 0
				n = uint64(math.Round(f * float64(unit))) // never overflows since f is within [0..1]
			}
			sumNanos, co = bits.Add64(sumNanos, n, 0) // overflow if co > 0
			overflow = overflow || co > 0
		}
		for len(whole) > 1 && whole[0] == '0' {
			whole = whole[len("0"):] // trim leading zeros
		}
		n, err := strconv.ParseUint(string(whole), 10, 64) // overflow if !ok && MaxUint64
		ok = err == nil
		hi, lo := bits.Mul64(n, uint64(unit))      // overflow if hi > 0
		sumNanos, co = bits.Add64(sumNanos, lo, 0) // overflow if co > 0
		invalid = invalid || (!ok && n != math.MaxUint64)
		overflow = overflow || (!ok && n == math.MaxUint64) || hi > 0 || co > 0
		inaccurate = inaccurate || unit > time.Hour
		return suffix
	}

	suffix, neg := consumeSign(b, true)
	prefix, suffix, okP := cutBytes(suffix, 'P', 'p')
	durDate, durTime, okT := cutBytes(suffix, 'T', 't')
	invalid = invalid || len(prefix) > 0 || !okP || (okT && len(durTime) == 0) || len(durDate)+len(durTime) == 0
	if len(durDate) > 0 { // nominal portion of the duration
		durDate = mayParseUnit(durDate, 'Y', 'y', time.Duration(daysPerYear*24*60*60*1e9))
		durDate = mayParseUnit(durDate, 'M', 'm', time.Duration(daysPerYear/12*24*60*60*1e9))
		durDate = mayParseUnit(durDate, 'W', 'w', time.Duration(7*24*60*60*1e9))
		durDate = mayParseUnit(durDate, 'D', 'd', time.Duration(24*60*60*1e9))
		invalid = invalid || len(durDate) > 0 // unknown elements
	}
	if len(durTime) > 0 { // accurate portion of the duration
		durTime = mayParseUnit(durTime, 'H', 'h', time.Duration(60*60*1e9))
		durTime = mayParseUnit(durTime, 'M', 'm', time.Duration(60*1e9))
		durTime = mayParseUnit(durTime, 'S', 's', time.Duration(1e9))
		invalid = invalid || len(durTime) > 0 // unknown elements
	}
	d := mayApplyDurationSign(sumNanos, neg)
	overflow = overflow || (neg != (d < 0) && d != 0) // overflows signed duration

	switch {
	case invalid:
		return 0, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, strconv.ErrSyntax)
	case overflow:
		return 0, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, strconv.ErrRange)
	case inaccurate:
		return d, fmt.Errorf("invalid ISO 8601 duration %q: %w", b, errInaccurateDateUnits)
	default:
		return d, nil
	}
}

// mayApplyDurationSign inverts n if neg is specified.
func mayApplyDurationSign(n uint64, neg bool) time.Duration {
	if neg {
		return -1 * time.Duration(n)
	} else {
		return +1 * time.Duration(n)
	}
}

// parsePaddedBase10 parses b as the zero-padded encoding of n,
// where max10 is a power-of-10 that is larger than n.
// Truncated suffix is treated as implicit zeros.
// Extended suffix is ignored, but verified to contain only digits.
func parsePaddedBase10(b []byte, max10 uint64) (n uint64, ok bool) {
	pow10 := uint64(1)
	for pow10 < max10 {
		n *= 10
		if len(b) > 0 {
			if b[0] < '0' || '9' < b[0] {
				return n, false
			}
			n += uint64(b[0] - '0')
			b = b[1:]
		}
		pow10 *= 10
	}
	if len(b) > 0 && len(bytes.TrimRight(b, "0123456789")) > 0 {
		return n, false // trailing characters are not digits
	}
	return n, true
}

// consumeSign consumes an optional leading negative or positive sign.
func consumeSign(b []byte, allowPlus bool) ([]byte, bool) {
	if len(b) > 0 {
		if b[0] == '-' {
			return b[len("-"):], true
		} else if b[0] == '+' && allowPlus {
			return b[len("+"):], false
		}
	}
	return b, false
}

// bytesCutByte is similar to bytes.Cut(b, []byte{c}),
// except c may optionally be included as part of the suffix.
func bytesCutByte(b []byte, c byte, include bool) ([]byte, []byte) {
	if i := bytes.IndexByte(b, c); i >= 0 {
		if include {
			return b[:i], b[i:]
		}
		return b[:i], b[i+1:]
	}
	return b, nil
}

// parseDec2 parses b as an unsigned, base-10, 2-digit number.
// The result is undefined if digits are not base-10.
func parseDec2(b []byte) byte {
	if len(b) < 2 {
		return 0
	}
	return 10*(b[0]-'0') + (b[1] - '0')
}
