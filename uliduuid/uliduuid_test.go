// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: GPL-3.0

package uliduuid_test

import (
	"testing"

	"github.com/tgulacsi/go/uliduuid"
)

func TestUUIDInc(t *testing.T) {
	for counterBits := 8; counterBits > 0; counterBits-- {
		u := uliduuid.NewUUID(counterBits)
		for i := 0; i < (1<<counterBits)+1; i++ {
			o := u
			t.Log(u)
			u.Inc()
			if o == u {
				t.Error("UUID remaind the same after Inc:", u)
			}
		}
	}
}

func TestUnmarshal(t *testing.T) {
	var hdr struct {
		ParentTransactionID string
		TransactionID       uliduuid.ID
	}
	hdr.ParentTransactionID = "X"
	u := uliduuid.NewUUID(2)
	if err := hdr.TransactionID.UnmarshalText([]byte(u.String())); err != nil {
		t.Fatal(err)
	}
	t.Logf("TransactionID=%q", hdr.TransactionID)
	if !hdr.TransactionID.IsUUID() || hdr.TransactionID.UUID() != u.UUID {
		t.Errorf("got %q wanted %q", hdr.TransactionID, u)
	}
}
