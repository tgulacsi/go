// Copyright 2023 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package fsfuse_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tgulacsi/go/fsfuse"
)

func TestDirFS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fusefs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempDir)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := fsfuse.Mount(ctx, fsfuse.NewServer(os.DirFS("..")), tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		m.Unmount(ctx)
	}()

	{
		want, err := listDir(ctx, "..")
		if err != nil {
			t.Fatal(err)
		}
		got, err := listDir(ctx, tempDir)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(want, got); d != "" {
			t.Fatal(d)
		}
	}

	{
		want, err := shaDir(ctx, "..")
		if err != nil {
			t.Fatal(err)
		}
		got, err := shaDir(ctx, tempDir)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got); d != "" {
			t.Fatal(d)
		}
	}
}
func shaDir(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "find", ".", "-exec", "sha512sum", "{}", ";")
	cmd.Dir = dir
	var buf, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &buf, &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %w\n%s", cmd.Args, err, errBuf.String())
	}
	vv := strings.Split(buf.String(), "\n")
	sort.Strings(vv)
	return vv, nil
}
func listDir(ctx context.Context, dir string) ([]string, error) {
	const printf = "%h/%f\t%U\t%G\t%M\t%n\t%s\t%l\n"
	cmd := exec.CommandContext(ctx, "find", ".", "-printf", printf)
	cmd.Dir = dir
	var buf, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &buf, &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %+v: %s", cmd.Args, err, errBuf.String())
	}
	vv := strings.Split(buf.String(), "\n")
	sort.Strings(vv)
	return vv, nil
}
