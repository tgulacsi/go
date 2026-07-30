// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

// Package safesql implements an SQL type that can be used to
// mark SQL strings.
package safesql

import (
	"strings"
)

// SQL holds an SQL string
type (
	SQL struct{ str string }

	SQLer interface{ SQL() string }

	// stringConstant is an unexported string type.
	//
	// Users of this package cannot create values of this type
	// except by passing an untyped string constant to functions
	// which expect a stringConstant.
	//
	// This type should only be used in function and method parameters.
	stringConstant string
)

func (s SQL) String() string { return s.str }

// FromConstant creates an SQL value from an untyped string constant.
func FromConstant(s stringConstant) SQL { return SQL{str: string(s)} }

// Concat SQL parts together.
func Concat(ss ...SQL) SQL {
	var buf strings.Builder
	for _, s := range ss {
		buf.WriteString(s.String())
	}
	return SQL{buf.String()}
}
