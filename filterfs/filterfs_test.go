// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package filterfs

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestOneFileFS(t *testing.T) {
	const existingFile = "existing.file"
	FS := oneFileFS{Name: existingFile, FS: fstest.MapFS{
		existingFile: &fstest.MapFile{
			Data: []byte("árvíztűrő tükörfúrógép"),
			Mode: 0644,
		},
		"not-existing.file": &fstest.MapFile{
			Data: []byte("not exist"),
			Mode: 0644,
		},
		"not-exist.link": &fstest.MapFile{
			Data: []byte(existingFile),
			Mode: 0644 | fs.ModeSymlink,
		},
	}}
	if err := fstest.TestFS(FS, existingFile); err != nil {
		t.Fatal(err)
	}
}
