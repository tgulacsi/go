// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package vcf

import (
	"errors"
	"io"
	"net/mail"
	"strings"

	"github.com/emersion/go-vcard"
)

// ScanForAddrs scans the vCard for addresses.
func ScanForAddrs(r io.Reader) ([]mail.Address, error) {
	var addrs []mail.Address
	vr := vcard.NewDecoder(r)
	for {
		card, err := vr.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return addrs, err
		}
		if email := card.Value(vcard.FieldEmail); email != "" {
			var nm string
			if name := card.Name(); name != nil {
				if name.FamilyName != "" {
					nm = strings.TrimRight(name.FamilyName+", "+name.GivenName, ", ")
				} else {
					nm = name.GivenName
				}
				if name.AdditionalName != "" {
					nm += " (" + name.AdditionalName + ")"
				}
			}
			addrs = append(addrs, mail.Address{Name: nm, Address: email})
		}
	}
	return addrs, nil
}
