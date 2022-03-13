// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package zipfs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/hack-pad/hackpadfs"
)

func TestZipFS(t *testing.T) {
	zf := newZipFromFS(t, os.DirFS(".."))
	if err := fs.WalkDir(zf, "", func(path string, de fs.DirEntry, err error) error {
		t.Log(path, de, err)
		if err != nil {
			t.Errorf("%q: %+v", path, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func newZipFromFS(tb testing.TB, src fs.FS) ZipFS {
	sr, err := buildZipFromFS(tb, src)
	if err != nil {
		tb.Fatal(err)
	}

	zf, err := NewZipFS(sr)
	if err != nil {
		tb.Fatal(err)
	}
	return zf
}

func buildZipFromFS(tb testing.TB, src fs.FS) (SectionReader, error) {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	defer archive.Close()

	err := hackpadfs.WalkDir(src, ".", copyZipWalk(src, archive))
	if err != nil {
		err = fmt.Errorf("building zip from FS walk: %w", err)
	} else {
		if err = archive.Close(); err != nil {
			tb.Fatal(err)
		}
	}
	return io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), err
}

func copyZipWalk(src hackpadfs.FS, archive *zip.Writer) hackpadfs.WalkDirFunc {
	return func(path string, dir hackpadfs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := dir.Info()
		if err != nil || info.IsDir() {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = path
		w, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		fileBytes, err := hackpadfs.ReadFile(src, path)
		if err != nil {
			return err
		}
		_, err = w.Write(fileBytes)
		return err
	}
}
