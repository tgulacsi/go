// Copyright 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package pdfreport

import (
	"io/fs"
	"testing"
)

func TestEmbed(t *testing.T) {
	var seen []string
	fs.WalkDir(_statikFS, "assets", func(path string, d fs.DirEntry, err error) error {
		t.Logf("%q: %v %+v", path, d, err)
		seen = append(seen, path)
		return err
	})
	t.Log(seen)
	if len(seen) == 0 {
		t.Errorf("seen: %v", seen)
	}

	if len(fonts) == 0 {
		t.Error("no fonts found")
	}
}
