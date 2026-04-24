// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: LGPL-3.0

package crypthlp_test

import (
	"os"
	"testing"

	"github.com/tgulacsi/go/crypthlp"
)

func TestParseJKS(t *testing.T) {
	b, err := os.ReadFile("testdata/pdf_signer.jks")
	if err != nil {
		t.Skip(err)
	}
	if bag, err := crypthlp.ReadJKSBytes(t.Context(), b, "19Kobe96", false); err != nil {
		t.Error(err)
	} else {
		key, cert, cas, err := bag.Parse()
		t.Logf("key: %+v cert: %+v cas: %+v", key, cert, cas)
		if err != nil {
			t.Error(err)
		}
	}
}
