// Copyright 2011, 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package i18nmail implements parsing of mail messages.

For the most part, this package follows the syntax as specified by RFC 5322.
Notable divergences:
    * Obsolete address formats are not parsed, including addresses with
      embedded route information.
    * Group addresses are not parsed.
    * The full range of spacing (the CFWS syntax element) is not supported,
      such as breaking addresses across lines.
*/
package i18nmail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/tgulacsi/go/text"

	//"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// Layouts suitable for passing to time.Parse.
// These are tried in order.
var dateLayouts = []string{
	time.RFC3339, time.RFC1123Z, time.RFC1123,
	time.RFC822Z, time.RFC822, "Monday, January 2, 2006 3:04 PM",
	time.UnixDate, time.RubyDate, time.ANSIC, time.Stamp,
}

func init() {
	// Generate layouts based on RFC 5322, section 3.3.

	dows := [...]string{"", "Mon, ", "Monday, "} // day-of-week
	days := [...]string{"2", "02"}               // day = 1*2DIGIT
	years := [...]string{"2006", "06"}           // year = 4*DIGIT / 2*DIGIT
	// "-0700 (MST)" is not in RFC 5322, but is common.
	zones := [...]string{"-0700", "MST", "-0700 (MST)", ""} // zone = (("+" / "-") 4DIGIT) / "GMT" / ...
	months := [...]string{"Jan", "January"}
	hours := [...]string{"15:04", "3:04PM", "3:04 PM"}
	seconds := [...]string{":05", ""} // second

	t := time.Now()
	for _, dow := range dows {
		for _, day := range days {
			for _, year := range years {
				for _, month := range months {
					for _, hour := range hours {
						for _, second := range seconds {
							for _, zone := range zones {
								s := dow + day + " " + month + " " + year + " " + hour + second + " " + zone
								if _, err := time.Parse(s, t.Format(s)); err == nil {
									dateLayouts = append(dateLayouts, s)
								}
							}
						}
					}
				}
			}
		}
	}
}

func parseDate(date string) (time.Time, error) {
	date = strings.TrimSpace(date)
	for _, layout := range dateLayouts {
		t, err := time.ParseInLocation(layout, date, time.Local)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q: %w", date, errors.New("mail: header could not be parsed"))
}

// A Header represents the key-value pairs in a mail message header.
type Header map[string][]string

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns "".
func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

var ErrHeaderNotPresent = errors.New("mail: header not in message")

// Date parses the Date header field.
func (h Header) Date() (time.Time, error) {
	hdr := h.Get("Date")
	if hdr == "" {
		return time.Time{}, ErrHeaderNotPresent
	}
	return parseDate(hdr)
}

// AddressList parses the named header field as a list of addresses.
func (h Header) AddressList(key string) ([]*Address, error) {
	hdr := h.Get(key)
	if hdr == "" {
		return nil, ErrHeaderNotPresent
	}
	return ParseAddressList(hdr)
}

// Decode returns the named header field as an utf-8 string.
func (h Header) Decode(key string) string {
	hdr := h.Get(key)
	if hdr == "" {
		return ""
	}
	return HeadDecode(hdr)
}

// Address represents a single mail address.
// An address such as "Barry Gibbs <bg@example.com>" is represented
// as Address{Name: "Barry Gibbs", Address: "bg@example.com"}.
type Address struct {
	Name    string // Proper name; may be empty.
	Address string // user@domain
}

var wsRepl = strings.NewReplacer("\t", " ", "  ", " ")

// ParseAddress parses a single RFC 5322 address, e.g. "Barry Gibbs <bg@example.com>"
func ParseAddress(address string) (*Address, error) {
	//return newAddrParser(address).parseAddress()
	address = wsRepl.Replace(address)
	maddr, err := AddressParser.Parse(address)
	if err != nil {
		uErr := errors.Unwrap(err)
		if uErr == nil {
			uErr = err
		}
		if strings.HasSuffix(uErr.Error(), "no angle-addr") &&
			strings.Contains(address, "<") && strings.Contains(address, ">") {
			maddr, err = AddressParser.Parse(address[strings.LastIndex(address, "<"):])
		} else {
			err = fmt.Errorf("%s: %w", address, err)
		}
	}
	if maddr == nil {
		return nil, err
	}
	return &Address{Name: maddr.Name, Address: maddr.Address}, err
}

// ParseAddressList parses the given string as a list of addresses.
func ParseAddressList(list string) ([]*Address, error) {
	//return newAddrParser(list).parseAddressList()
	al, err := AddressParser.ParseList(wsRepl.Replace(list))
	if al == nil {
		return nil, err
	}
	aL := make([]*Address, len(al))
	for i, a := range al {
		aL[i] = &Address{Name: a.Name, Address: a.Address}
	}
	return aL, err
}

// String formats the address as a valid RFC 5322 address.
// If the address's name contains non-ASCII characters
// the name will be rendered according to RFC 2047.
func (a *Address) String() string {
	s := "<" + a.Address + ">"
	if a.Name == "" {
		return s
	}
	// If every character is printable ASCII, quoting is simple.
	allPrintable := true
	for i := 0; i < len(a.Name); i++ {
		if !isVchar(a.Name[i]) {
			allPrintable = false
			break
		}
	}
	if allPrintable {
		b := bytes.NewBufferString(`"`)
		for i := 0; i < len(a.Name); i++ {
			if !isQtext(a.Name[i]) {
				b.WriteByte('\\')
			}
			b.WriteByte(a.Name[i])
		}
		b.WriteString(`" `)
		b.WriteString(s)
		return b.String()
	}

	// UTF-8 "Q" encoding
	b := bytes.NewBufferString("=?utf-8?q?")
	for i := 0; i < len(a.Name); i++ {
		switch c := a.Name[i]; {
		case c == ' ':
			b.WriteByte('_')
		case isVchar(c) && c != '=' && c != '?' && c != '_':
			b.WriteByte(c)
		default:
			fmt.Fprintf(b, "=%02X", c)
		}
	}
	b.WriteString("?= ")
	b.WriteString(s)
	return b.String()
}

// WordDecoder decodes mime rords.
var WordDecoder = &mime.WordDecoder{
	CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		//enc, err := ianaindex.MIME.Get(charset)
		enc, err := htmlindex.Get(charset)
		if err != nil {
			return input, err
		}
		return transform.NewReader(input, enc.NewDecoder()), nil
	},
}

// AddressParser is a mail address parser.
var AddressParser = &mail.AddressParser{WordDecoder: WordDecoder}

// HeadDecode decodes mail header encoding (quopri or base64) such as
// =?iso-8859-2?Q?MEN-261_K=D6BE_k=E1r.pdf?=
func HeadDecode(head string) string {
	if head == "" {
		return ""
	}
	res, err := WordDecoder.DecodeHeader(head)
	if err != nil {
		logger.Error("HeadDecode", "head", head, "error", err)
		return head
	}
	if strings.Contains(res, "=?") && !strings.HasSuffix(res, "=?u...") {
		logger.Debug("WordDecoder", "head", head, "res", res)
	}
	return res
}

// DecodeRFC2047Word decodes the string as RFC2407.
func DecodeRFC2047Word(s string) (string, error) {
	fields := strings.Split(s, "?")
	if len(fields) != 5 || fields[0] != "=" || fields[4] != "=" {
		return "", errors.New("mail: address not RFC 2047 encoded")
	}
	charset, encMark := strings.ToLower(fields[1]), strings.ToLower(fields[2])
	enc := text.GetEncoding(charset)
	if enc == nil {
		return "", fmt.Errorf("mail: charset not supported: %q", charset)
	}

	in := bytes.NewBufferString(fields[3])
	var r io.Reader
	switch encMark {
	case "b":
		r = base64.NewDecoder(base64.StdEncoding, in)
	case "q":
		r = qDecoder{r: in}
	default:
		return "", fmt.Errorf("mail: RFC 2047 encoding not supported: %q", encMark)
	}

	dec, err := io.ReadAll(text.NewReader(r, enc))
	if err != nil {
		return "", err
	}

	return string(dec), err
}

type qDecoder struct {
	r       io.Reader
	scratch [2]byte
}

func (qd qDecoder) Read(p []byte) (n int, err error) {
	// This method writes at most one byte into p.
	if len(p) == 0 {
		return 0, nil
	}
	if n, err := qd.r.Read(qd.scratch[:1]); n == 0 {
		return 0, err
	}
	switch c := qd.scratch[0]; {
	case c == '=':
		if _, err := io.ReadFull(qd.r, qd.scratch[:2]); err != nil {
			return 0, err
		}
		x, err := strconv.ParseInt(string(qd.scratch[:2]), 16, 64)
		if err != nil {
			return 0, fmt.Errorf("mail: invalid RFC 2047 encoding: %q", qd.scratch[:2])
		}
		p[0] = byte(x)
	case c == '_':
		p[0] = ' '
	default:
		p[0] = c
	}
	return 1, err
}

// isQtext returns true if c is an RFC 5322 qtest character.
func isQtext(c byte) bool {
	// Printable US-ASCII, excluding backslash or quote.
	if c == '\\' || c == '"' {
		return false
	}
	return '!' <= c && c <= '~'
}

// isVchar returns true if c is an RFC 5322 VCHAR character.
func isVchar(c byte) bool {
	// Visible (printing) characters.
	return '!' <= c && c <= '~'
}
