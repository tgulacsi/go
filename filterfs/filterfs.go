// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package filterfs

import (
	"io/fs"
	"path"
)

var (
	_ fs.FS         = oneFileFS{}
	_ fs.ReadDirFS  = oneFileFS{}
	_ fs.ReadFileFS = oneFileFS{}
	_ fs.ReadLinkFS = oneFileFS{}
	_ fs.GlobFS     = oneFileFS{}

	eNE = fs.ErrNotExist
)

type oneFileFS struct {
	FS   fs.FS
	Name string
}

// NewOneFileFS returns an fs.FS that is constrained to the one given name.
func NewOneFileFS(fsys fs.FS, name string) oneFileFS {
	return oneFileFS{FS: fsys, Name: name}
}

func (ofs oneFileFS) theFileName(name string) error {
	if !fs.ValidPath(name) {
		return ofs.pathError(name, "open", fs.ErrInvalid)
	}
	if name != ofs.Name {
		return ofs.pathError(name, "open", eNE)
	}
	return nil
}
func (ofs oneFileFS) Open(name string) (fs.File, error) {
	if name != "." && name != "/" {
		if err := ofs.theFileName(name); err != nil {
			return nil, err
		}
	}
	f, err := ofs.FS.Open(name)
	if err != nil {
		return f, err
	}
	d, ok := f.(fs.ReadDirFile)
	if !ok {
		return f, err
	}
	return oneFileDir{ReadDirFile: d, Name: ofs.Name}, err
}

func (ofs oneFileFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." || name == "/" {
		ds, err := fs.ReadDir(ofs.FS, name)
		for _, d := range ds {
			if d.Name() == ofs.Name {
				return []fs.DirEntry{d}, nil
			}
		}
		return nil, err
	}
	return nil, ofs.pathError(name, "ReadDir", eNE)
}

func (ofs oneFileFS) ReadFile(name string) ([]byte, error) {
	if err := ofs.theFileName(name); err != nil {
		return nil, err
	}
	return fs.ReadFile(ofs.FS, name)
}

func (ofs oneFileFS) ReadLink(name string) (string, error) {
	if err := ofs.theFileName(name); err != nil {
		return "", err
	}
	return fs.ReadLink(ofs.FS, name)
}

func (ofs oneFileFS) Lstat(name string) (fs.FileInfo, error) {
	if err := ofs.theFileName(name); err != nil {
		return nil, err
	}
	return fs.Lstat(ofs.FS, name)
}

func (ofs oneFileFS) Glob(pattern string) ([]string, error) {
	list := make([]string, 0, 2)
	for _, nm := range []string{".", ofs.Name} {
		if m, err := path.Match(pattern, nm); err != nil {
			return list, err
		} else if m {
			list = append(list, nm)
		}
	}
	return list, nil
}

func (ofs oneFileFS) pathError(name, op string, err error) *fs.PathError {
	if err == nil {
		err = eNE
	}
	return &fs.PathError{Path: name, Op: op, Err: err}
}

type oneFileDir struct {
	fs.ReadDirFile
	Name string
}

func (ofd oneFileDir) ReadDir(n int) ([]fs.DirEntry, error) {
	dd, err := ofd.ReadDirFile.ReadDir(n)
	for i, d := range dd {
		if d.Name() == ofd.Name {
			return dd[i : i+1], err
		}
	}
	return nil, err
}
