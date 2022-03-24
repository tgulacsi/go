// Copyright 2022 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package zipfs implements an io/fs.FS serving a read-only zip file.
package zipfs

import (
	"archive/zip"
	"bytes"
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

// ZipFS wraps a zip.Reader as an fs.FS.
type ZipFS struct {
	z *zip.Reader
	m map[string]int
}

var _ = fs.GlobFS(ZipFS{})
var _ = fs.StatFS(ZipFS{})
var _ = fs.ReadDirFS(ZipFS{})

// SectionReader is the interface returned by io.NewSectionReader.
type SectionReader interface {
	io.ReaderAt
	Size() int64
}

// BytesSectionReader wraps the []byte slice as a SectionReader.
func BytesSectionReader(p []byte) *io.SectionReader {
	return io.NewSectionReader(bytes.NewReader(p), 0, int64(len(p)))
}

// MustNewZipFS calls ZipFS and panics on error.
//
// Example usage:
// //go:generate zip -9r assets.zip assets/
// //go:embed assets.zip
// var assetsZIP []byte
// var assetsFS = zipfs.MustNewZipFS(BytesSectionReader(assetsZIP))
func MustNewZipFS(sr SectionReader) ZipFS {
	zf, err := NewZipFS(sr)
	if err != nil {
		panic(err)
	}
	return zf
}

// NewZipFS provides the given zip file as an fs.FS.
func NewZipFS(sr SectionReader) (ZipFS, error) {
	z, err := zip.NewReader(sr, sr.Size())
	if err != nil {
		return ZipFS{}, err
	}
	m := make(map[string]int, len(z.File))
	for i, F := range z.File {
		m[F.Name] = i
	}
	return ZipFS{z: z, m: m}, nil
}

// Open the named file.
func (zf ZipFS) Open(name string) (fs.File, error) {
	if i, ok := zf.m[name]; ok {
		f := zf.z.File[i]
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		log.Println(name, f.FileInfo().Mode())
		return zipFile{ReadCloser: rc, fi: f.FileInfo()}, nil
	}
	// Maybe it's a directory
	var dn string
	var ok bool
	var rootFi fs.FileInfo
	if ok = name == "." || name == "./"; !ok {
		var i int
		if i, ok = zf.m[name+"/"]; ok {
			dn = name + "/"
		}
		rootFi = zf.z.File[i].FileInfo()
	}
	if !ok {
		return nil, fs.ErrNotExist
	}
	files := make([]dirEntry, 0, len(zf.z.File))
	seen := make(map[string]struct{})
	for _, f := range zf.z.File {
		if dn != "" && !strings.HasPrefix(f.Name, dn) {
			continue
		}
		de := dirEntry{File: f, name: strings.TrimPrefix(f.Name, dn)}
		if i := strings.IndexByte(de.name, '/'); i >= 0 {
			dir := de.name[:i]
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				files = append(files, dirEntry{File: f, name: dir})
			}
		}
		files = append(files, de)
	}
	return &zipDir{files: files, fi: rootFi}, nil
}

// Glob returns all the files matching the pattern.
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

// ReadDir reads the named directory.
func (zf ZipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	des := make([]fs.DirEntry, 0, len(zf.z.File))
	name += "/"
	for _, f := range zf.z.File {
		if name == "" || name == "/" || name == "." || name == "./" || strings.HasPrefix(f.Name, name) {
			des = append(des, dirEntry{File: f, name: strings.TrimPrefix(f.Name, name)})
		}
	}
	log.Printf("ReadDir(%q): %+v", name, des)
	return des, nil
}

// Stat the named file.
func (zf ZipFS) Stat(name string) (fs.FileInfo, error) {
	if i, ok := zf.m[name]; ok {
		log.Printf("zf.Stat(%q): %v", name, zf.z.File[i].FileInfo().Mode())
		return zf.z.File[i].FileInfo(), nil
	}
	// Maybe it's a directory
	if i, ok := zf.m[name+"/"]; ok {
		log.Printf("zf.Stat(%q): %v", name, zf.z.File[i].FileInfo().Mode())
		return zf.z.File[i].FileInfo(), nil
	}
	if name == "" || name == "." {
		log.Printf("zf.Stat(%q): %v", name, dummyFileInfo{})
		return dummyFileInfo{}, nil
	}
	return nil, fmt.Errorf("zip root dir: %w", fs.ErrNotExist)
}

type zipFile struct {
	io.ReadCloser
	fi fs.FileInfo
}

var _ = fs.File(zipFile{})

func (zf zipFile) Name() string { log.Println("zf.Name:", zf); return zf.fi.Name() }
func (zf zipFile) Stat() (fs.FileInfo, error) {
	log.Printf("zf.Stat(%q): %v", zf.Name(), zf.fi.Mode())
	return zf.fi, nil
}

type zipDir struct {
	fi    fs.FileInfo
	files []dirEntry
}

var _ = fs.ReadDirFile((*zipDir)(nil))

func (zd *zipDir) Name() string {
	log.Println("zd.Name:", zd)
	if zd.fi == nil {
		return "."
	}
	return zd.fi.Name()
}
func (zd *zipDir) Read(_ []byte) (int, error) { return 0, fs.ErrInvalid }
func (zd *zipDir) Close() error               { zd.files = nil; return nil }
func (zd *zipDir) Stat() (fs.FileInfo, error) {
	log.Printf("zd.Stat(%q): %v", zd.Name(), zd.fi.Mode())
	return zd.fi, nil
}
func (zd *zipDir) ReadDir(n int) (des []fs.DirEntry, err error) {
	defer func(n int) { log.Printf("ReadDir[%q](%d): %v, %+v", zd.Name(), n, des, err) }(n)
	if len(zd.files) == 0 {
		return nil, nil
	}

	if n <= 0 || len(zd.files) <= n {
		n = len(zd.files)
	}
	var advance int
	for _, f := range zd.files {
		advance++
		if strings.IndexByte(f.name, '/') >= 0 {
			continue
		}
		if des == nil {
			des = make([]fs.DirEntry, 0, n)
		}
		des = append(des, f)
	}
	zd.files = zd.files[advance:]
	return des, nil
}

type dirEntry struct {
	name string
	*zip.File
}

func (de dirEntry) IsDir() bool {
	return len(de.File.Name) > 0 && de.File.Name[len(de.File.Name)-1] == '/'
}
func (de dirEntry) Info() (fs.FileInfo, error) {
	log.Printf("de.Info(%q): %v", de.name, de.File.FileInfo().Mode())
	return de.File.FileInfo(), nil
}
func (de dirEntry) Name() string {
	return strings.TrimSuffix(de.name, "/")
}
func (de dirEntry) Type() fs.FileMode { return de.File.Mode() }

type dummyFileInfo struct{}

var _ = fs.FileInfo(dummyFileInfo{})

func (dummyFileInfo) Name() string       { return "." }
func (dummyFileInfo) Size() int64        { return 0 }
func (dummyFileInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (dummyFileInfo) ModTime() time.Time { return time.Time{} }
func (dummyFileInfo) IsDir() bool        { return true }
func (dummyFileInfo) Sys() interface{}   { return nil }
