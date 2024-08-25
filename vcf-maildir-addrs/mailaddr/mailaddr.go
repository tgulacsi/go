// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mailaddr

import (
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
	aa, err := mail.ParseAddressList(text)
	if err != nil {
		aa, err = mail.ParseAddressList(cleanAddress(text))
	}
	addrs := make([]mail.Address, len(aa))
	for i, a := range aa {
		addrs[i] = *a
	}
	return addrs, err
}

var rEmailAddr = regexp.MustCompile("<?[^@ <]+@[^@ >]+>?")

func cleanAddress(text string) string {
	text = strings.TrimSpace(text)
	return rEmailAddr.ReplaceAllStringFunc(text, func(s string) string {
		if s == text {
			return s
		}
		// slog.Info("replace", "s", s)
		if s[0] == '<' && s[len(s)-1] == '>' {
			return s
		}
		return strings.ReplaceAll(s, "@", "_at_")
	})
}
