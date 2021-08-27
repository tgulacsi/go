// Copyright 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package biflag_test

import (
	"flag"
	"testing"

	"github.com/tgulacsi/go/biflag"
)

func TestNotSet(t *testing.T) {
	fs := flag.NewFlagSet("biflag-unset", flag.PanicOnError)
	b := biflag.NewBool(true)
	fs.Var(b, "bool", "bool")
	if err := fs.Parse([]string{""}); err != nil {
		t.Fatal(err)
	}
	t.Logf("b=%v=%v isSet=%t", b, b.Bool(), b.IsSet())
}
func TestSet(t *testing.T) {
	fs := flag.NewFlagSet("biflag-set", flag.PanicOnError)
	b := biflag.NewBool(true)
	fs.Var(b, "bool-false", "bool")
	if err := fs.Parse([]string{"-bool-false=false"}); err != nil {
		t.Fatal(err)
	}
	t.Logf("b=%v=%v isSet=%t", b, b.Bool(), b.IsSet())
}
