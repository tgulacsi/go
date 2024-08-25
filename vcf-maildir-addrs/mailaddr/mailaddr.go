// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mailaddr

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
)

// ScanForAddrs
func ScanForAddrs(r io.Reader) ([]mail.Address, error) {
	// READ MIME HEADERS
	msg, err := message.Read(r)
	if err != nil {
		return nil, fmt.Errorf("message.Read: %w", err)
	}
	ff := msg.Header.FieldsByKey("X-Spam-Flag")
	for ff.Next() {
		if f, _ := ff.Text(); f != "" && strings.EqualFold(f[:1], "Y") {
			slog.Debug("skip spam", "X-Spam-Flag", f)
			return nil, nil
		}
	}
	var addrs []mail.Address
	for _, k := range []string{"From", "To", "Cc", "Bcc", "Reply-To"} {
		ff := msg.Header.FieldsByKey(k)
		for ff.Next() {
			if f, _ := ff.Text(); f != "" {
				aa, err := ParseAddressList(f)
				addrs = append(addrs, aa...)
				if err != nil {
					slog.Warn("parse", "k", k, "v", f, "error", err)
				}
			}
		}
	}
	return addrs, nil
}

func ParseAddressList(text string) ([]mail.Address, error) {
	if strings.IndexByte(text, '@') < 0 {
		return nil, nil
	}
	if aa, err := mail.ParseAddressList(text); err == nil {
		addrs := make([]mail.Address, len(aa))
		for i, a := range aa {
			addrs[i] = *a
		}
		return addrs, err
	}
	// aa, err = mail.ParseAddressList(cleanAddress(text))
	var addrs []mail.Address
	var errs []error
	for _, s := range strings.SplitAfter(text, ">,") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if s[len(s)-1] == ',' {
			s = s[:len(s)-1]
		}
		s = cleanAddress(s)
		// if n := strings.Count(s, "@"); n > 1 {
		// 	s = strings.Replace(s, "@", "_at_", n-1)
		// }
		if i := strings.LastIndexByte(s, '<'); i >= 0 && s[0] != '"' {
			if prefix := strings.TrimRight(s[:i], " "); !strings.HasSuffix(prefix, `"`) {
				s = `"` + prefix + `" ` + s[i:]
			}
		}
		if a, err := mail.ParseAddress(s); err != nil {
			errs = append(errs, fmt.Errorf("parse %q: %w", s, err))
		} else {
			addrs = append(addrs, *a)
		}
	}
	return addrs, errors.Join(errs...)
}

var rEmailAddr = regexp.MustCompile("<[^@<]+@[^>@]+>|<?[^@ <]+@[^@ >]+>?")

func cleanAddress(text string) string {
	text = strings.TrimSpace(text)
	return rEmailAddr.ReplaceAllStringFunc(text, func(s string) string {
		if s == text {
			return s
		}
		s = strings.ReplaceAll(s, " ", "")
		// slog.Info("replace", "s", s)
		if s[0] == '<' && s[len(s)-1] == '>' {
			return s
		}
		return strings.ReplaceAll(s, "@", "_at_")
	})
}
