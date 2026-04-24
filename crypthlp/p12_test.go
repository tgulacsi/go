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
	for _, outputPEM := range []bool{false, true} {
		if _, _, _, err := crypthlp.ReadJKSBytes(t.Context(), b, "19Kobe96", outputPEM); err != nil {
			t.Errorf("outputPEM=%t: %+v", outputPEM, err)
		}
	}
}
