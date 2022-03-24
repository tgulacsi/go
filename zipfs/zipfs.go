// Copyright 2022 Tamás Gulácsi. All rights reserved.
// Copyright 2020 The Go Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package zipfs implements an io/fs.FS serving a read-only zip file.
package zipfs

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

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
	fsys := make(ZipFS, len(z.File))
	for _, F := range z.File {
		fi := F.FileInfo()
		fsys[F.Name] = &zipFile{File: F, Mode: fi.Mode(), ModTime: F.Modified}
	}
	return fsys, nil
}

// The following code is copied from the Go source tree testing/fstest/mapfs.go
// It's been modified just a little bit to be used as a zipfs.
//
// Copyright 2020 The Go Authors. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A ZipFS is a simple in-memory file system for use in tests,
// represented as a map from path names (arguments to Open)
// to information about the files or directories they represent.
//
// The map need not include parent directories for files contained
// in the map; those will be synthesized if needed.
// But a directory can still be included by setting the zipFile.Mode's ModeDir bit;
// this may be necessary for detailed control over the directory's FileInfo
// or to create an empty directory.
//
// File system operations read directly from the map,
// so that the file system can be changed by editing the map as needed.
// An implication is that file system operations must not run concurrently
// with changes to the map, which would be a race.
// Another implication is that opening or reading a directory requires
// iterating over the entire map, so a ZipFS should typically be used with not more
// than a few hundred entries or directory reads.
type ZipFS map[string]*zipFile

// A zipFile describes a single file in a ZapFS.
type zipFile struct {
	Mode    fs.FileMode // FileInfo.Mode
	ModTime time.Time   // FileInfo.ModTime
	Sys     interface{} // FileInfo.Sys
	*zip.File
}

var _ fs.FS = ZipFS(nil)
var _ fs.File = (*openzipFile)(nil)

// Open opens the named file.
func (fsys ZipFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	file := fsys[name]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &openzipFile{path: name, zipFileInfo: zipFileInfo{path.Base(name), file}, ReadCloser: rc}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []zipFileInfo
	var elem string
	var need = make(map[string]bool)
	if name == "." {
		elem = "."
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, zipFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
		prefix := name + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, zipFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, zipFileInfo{name, &zipFile{Mode: fs.ModeDir}})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].name < list[j].name
	})

	if file == nil {
		file = &zipFile{Mode: fs.ModeDir}
	}
	return &zipDir{name, zipFileInfo{elem, file}, list, 0}, nil
}

// fsOnly is a wrapper that hides all but the fs.FS methods,
// to avoid an infinite recursion when implementing special
// methods in terms of helpers that would use them.
// (In general, implementing these methods using the package fs helpers
// is redundant and unnecessary, but having the methods may make
// ZipFS exercise more code paths when used in tests.)
type fsOnly struct{ fs.FS }

func (fsys ZipFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(fsOnly{fsys}, name)
}

func (fsys ZipFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(fsOnly{fsys}, name)
}

func (fsys ZipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(fsOnly{fsys}, name)
}

func (fsys ZipFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(fsOnly{fsys}, pattern)
}

type noSub struct {
	ZipFS
}

func (noSub) Sub() {} // not the fs.SubFS signature

func (fsys ZipFS) Sub(dir string) (fs.FS, error) {
	return fs.Sub(noSub{fsys}, dir)
}

// A zipFileInfo implements fs.FileInfo and fs.DirEntry for a given map file.
type zipFileInfo struct {
	name string
	f    *zipFile
}

func (i *zipFileInfo) Name() string { return i.name }
func (i *zipFileInfo) Size() int64 {
	if i.f == nil || i.f.File == nil {
		return 0
	}
	return int64(i.f.UncompressedSize64)
}
func (i *zipFileInfo) Mode() fs.FileMode          { return i.f.Mode }
func (i *zipFileInfo) Type() fs.FileMode          { return i.f.Mode.Type() }
func (i *zipFileInfo) ModTime() time.Time         { return i.f.ModTime }
func (i *zipFileInfo) IsDir() bool                { return i.f.Mode&fs.ModeDir != 0 }
func (i *zipFileInfo) Sys() interface{}           { return i.f.Sys }
func (i *zipFileInfo) Info() (fs.FileInfo, error) { return i, nil }

// An openzipFile is a regular (non-directory) fs.File open for reading.
type openzipFile struct {
	path string
	zipFileInfo
	io.ReadCloser
}

func (f *openzipFile) Stat() (fs.FileInfo, error) { return &f.zipFileInfo, nil }

func (f *openzipFile) Close() error { return nil }

// A zipDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type zipDir struct {
	path string
	zipFileInfo
	entry  []zipFileInfo
	offset int
}

func (d *zipDir) Stat() (fs.FileInfo, error) { return &d.zipFileInfo, nil }
func (d *zipDir) Close() error               { return nil }
func (d *zipDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *zipDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}
