// Copyright 2019, 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package iohlp_test

import (
	"strings"
	"testing"

	"github.com/tgulacsi/go/iohlp"
)

func TestFindReader(t *testing.T) {
	const haystack = `abraka dabra`
	const needle = `da`
	i, err := iohlp.FindReader(strings.NewReader(haystack), []byte(needle))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Index(haystack, needle); i != want {
		t.Errorf("got %d, wanted %d", i, want)
	}
}
