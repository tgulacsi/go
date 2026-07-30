// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package safesql_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCollectSQL(t *testing.T) {
	b, err := exec.CommandContext(t.Context(),
		"go", "run", "./cmd/safesql", "--",
		"collect", filepath.Join(".", "testdata", "go"),
	).CombinedOutput()
	t.Log(string(b))
	if err != nil {
		t.Fatal(err)
	}
}
