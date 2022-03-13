// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package zipfs implements an io/fs.FS serving a read-only zip file.
package zipfs

import (
	"archive/zip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"
	"time"
)

var _ embed.FS

type ZipFS struct {
	z *zip.Reader
	m map[string]int
}

var _ = fs.GlobFS(ZipFS{})
var _ = fs.StatFS(ZipFS{})
var _ = fs.ReadDirFS(ZipFS{})

type SectionReader interface {
	io.ReaderAt
	Size() int64
}

func NewZipFS(sr SectionReader) (ZipFS, error) {
	z, err := zip.NewReader(sr, sr.Size())
	if err != nil {
		return ZipFS{}, err
	}
	m := make(map[string]int, len(z.File))
	for i, F := range z.File {
		log.Println(F.Name)
		m[F.Name] = i
	}
	return ZipFS{z: z, m: m}, nil
}

func (zf ZipFS) Open(name string) (fs.File, error) {
	name = path.Clean(name)
	if i, ok := zf.m[name]; ok {
		f := zf.z.File[i]
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		return zipFile{ReadCloser: rc, fh: f.FileInfo()}, nil
	}
	// Maybe it's a directory
	if i, ok := zf.m[name+"/"]; ok {
		dn := name + "/"
		files := make([]*zip.File, 0, len(zf.z.File))
		for _, f := range zf.z.File {
			if strings.HasPrefix(f.Name, dn) {
				files = append(files, f)
			}
		}
		return &zipDir{files: files, fi: zf.z.File[i].FileInfo()}, nil
	}
	// Maybe it's the root
	if name == "" || name == "." || name == "/" {
		return &zipDir{files: zf.z.File, fi: nil}, nil
	}
	return nil, fs.ErrNotExist
}

func (zf ZipFS) Glob(pattern string) ([]string, error) {
	des := make([]string, 0, len(zf.z.File))
	for _, f := range zf.z.File {
		if ok, err := path.Match(pattern, f.Name); ok {
			des = append(des, f.Name)
		} else if err != nil {
			return des, err
		}
	}
	return des, nil
}

func (zf ZipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	des := make([]fs.DirEntry, 0, len(zf.z.File))
	name = path.Clean(name) + "/"
	for _, f := range zf.z.File {
		if name == "" || name == "/" || name == "." || strings.HasPrefix(f.Name, name) {
			des = append(des, dirEntry{File: f})
		}
	}
	return des, nil
}

func (zf ZipFS) Stat(name string) (fs.FileInfo, error) {
	name = path.Clean(name)
	if i, ok := zf.m[name]; ok {
		return zf.z.File[i].FileInfo(), nil
	}
	// Maybe it's a directory
	if i, ok := zf.m[name+"/"]; ok {
		return zf.z.File[i].FileInfo(), nil
	}
	if name == "" || name == "/" || name == "." {
		return dummyFileInfo{}, nil
	}
	return nil, fmt.Errorf("zip root dir: %w", fs.ErrNotExist)
}

type zipFile struct {
	io.ReadCloser
	fh fs.FileInfo
}

var _ = fs.File(zipFile{})

func (zf zipFile) Stat() (fs.FileInfo, error) { return zf.fh, nil }

type zipDir struct {
	fi    fs.FileInfo
	files []*zip.File
}

var _ = fs.ReadDirFile((*zipDir)(nil))

func (zd *zipDir) Read(_ []byte) (int, error) { return 0, fs.ErrInvalid }
func (zd *zipDir) Close() error               { zd.files = nil; return nil }
func (zd *zipDir) Stat() (fs.FileInfo, error) { return zd.fi, nil }
func (zd *zipDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if len(zd.files) == 0 {
		return nil, io.EOF
	}

	var des []fs.DirEntry
	if n <= 0 || len(zd.files) <= n {
		des = make([]fs.DirEntry, len(zd.files))
	} else {
		des = make([]fs.DirEntry, n)
	}
	for i, f := range zd.files {
		des[i] = dirEntry{File: f}
	}
	zd.files = zd.files[len(des):]
	log.Println(len(zd.files), len(des))
	if len(zd.files) == 0 {
		return des, io.EOF
	}
	return des, nil
}

type dirEntry struct{ *zip.File }

func (de dirEntry) IsDir() bool {
	return len(de.File.Name) > 0 && de.File.Name[len(de.File.Name)-1] == '/'
}
func (de dirEntry) Info() (fs.FileInfo, error) { return de.File.FileInfo(), nil }
func (de dirEntry) Name() string               { return strings.TrimSuffix(de.File.Name, "/") }
func (de dirEntry) Type() fs.FileMode          { return de.File.Mode() }

type dummyFileInfo struct{}

var _ = fs.FileInfo(dummyFileInfo{})

func (dummyFileInfo) Name() string       { return "/" }
func (dummyFileInfo) Size() int64        { return 0 }
func (dummyFileInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (dummyFileInfo) ModTime() time.Time { return time.Time{} }
func (dummyFileInfo) IsDir() bool        { return true }
func (dummyFileInfo) Sys() interface{}   { return nil }
