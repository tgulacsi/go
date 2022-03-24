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
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/hack-pad/hackpadfs"
)

func TestZipFS(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Skip(err)
	}
	zf := newZipFromFS(t, os.DirFS(filepath.Join(filepath.Dir(wd), "coord")))
	if err := fs.WalkDir(zf, "", func(path string, de fs.DirEntry, err error) error {
		t.Log("path:", path, "dirEntry:", de, "error:", err)
		if err != nil {
			t.Errorf("%q: %+v", path, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	t.Log("zf:", zf)
	if err := fstest.TestFS(zf, "assets/gmaps.html"); err != nil {
		t.Fatal(err)
	}
}

func newZipFromFS(tb testing.TB, src fs.FS) ZipFS {
	sr, err := buildZipFromFS(tb, src)
	if err != nil {
		tb.Fatal(err)
	}

	zr, err := zip.NewReader(sr, sr.Size())
	if err != nil {
		tb.Fatal(err)
	}
	for _, f := range zr.File {
		tb.Log("zip:", f.Name)
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

	err := hackpadfs.WalkDir(src, ".", copyZipWalk(tb, src, archive))
	if err != nil {
		err = fmt.Errorf("building zip from FS walk: %w", err)
	} else {
		if err = archive.Close(); err != nil {
			tb.Fatal(err)
		}
	}
	return io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), err
}

func copyZipWalk(tb testing.TB, src hackpadfs.FS, archive *zip.Writer) hackpadfs.WalkDirFunc {
	return func(path string, dir hackpadfs.DirEntry, err error) error {
		//tb.Logf("copy %q at %v (%+v)", path, dir, err)
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
