// Copyright 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

// Package biflag provides bi-state (set/not-set) standard flag library compatible flags.
package biflag

import (
	"flag"
	"fmt"
	"strconv"
)

// Interface is flag.Getter + IsSet method.
// All flags in this package implements this interface.
type Interface interface {
	flag.Getter
	IsSet() bool
}

type biBool struct {
	value *bool
	isSet bool
}

var _ Interface = ((*biBool)(nil))

func NewBool(def bool) *biBool             { return &biBool{value: &def} }
func NewBoolVar(p *bool, def bool) *biBool { *p = def; return &biBool{value: p} }
func (b *biBool) Bool() bool {
	if b == nil {
		return false
	}
	return *b.value
}
func (b *biBool) String() string {
	s := "false"
	if b != nil && b.value != nil && *b.value {
		s = "true"
	}
	if b == nil || !b.isSet {
		return "[" + s + "]"
	}
	return s
}
func (b *biBool) Get() interface{} { return *b.value }
func (b *biBool) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("%q: %w", s, err)
	}
	b.isSet = true
	*b.value = v
	return nil
}
func (b *biBool) IsSet() bool { return b != nil && b.isSet }
