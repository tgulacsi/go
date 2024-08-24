// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package mailaddr

import (
	"fmt"
	"io"
	"log/slog"
	"net/mail"
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
				aa, err := mail.ParseAddressList(f)
				if err != nil {
					slog.Warn("parse", "k", k, "v", f, "error", err)
					continue
				}
				for _, a := range aa {
					addrs = append(addrs, *a)
				}
			}
		}
	}
	return addrs, nil
}
