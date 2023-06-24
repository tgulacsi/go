// Copyright 2023 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package fsfuse_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/tgulacsi/go/fsfuse"
)

func TestZipFS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fusefs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempDir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := fuse.Mount(tempDir, fsfuse.NewFS(
		os.DirFS("."),
		uint32(os.Geteuid()),
		uint32(os.Getegid()),
		0,
	),
		&fuse.MountConfig{
			OpContext:   ctx,
			ReadOnly:    true,
			DebugLogger: log.Default(),
			ErrorLogger: log.Default(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Join(ctx)
	fuse.Unmount(tempDir)
}
