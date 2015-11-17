// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iohlp

import (
	"io"
	"os"
	"path/filepath"
)

// walk recursively descends path, calling w.
func walk(path string, info os.FileInfo, walkFn filepath.WalkFunc, followSymlinks bool) error {
	stat := os.Lstat
	if followSymlinks {
		stat = os.Stat
	}
	err := walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return nil
	}

	c, err := readDirNames(path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for names := range c {
		if names.err != nil {
			return walkFn(path, info, names.err)
		}
		for _, name := range names.names {
			filename := filepath.Join(path, name)
			fileInfo, err := stat(filename)
			if err != nil {
				if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
					return err
				}
			} else {
				err = walk(filename, fileInfo, walkFn, followSymlinks)
				if err != nil {
					if !fileInfo.IsDir() || err != filepath.SkipDir {
						return err
					}
				}
			}
		}
	}
	return nil
}

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn. The files are walked UNORDERED,
// which makes the output undeterministic!
// Walk does not follow symbolic links.
func Walk(root string, walkFn filepath.WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(root, info, walkFn, false)
}

// WalkWithSymlinks walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn. The files are walked UNORDERED,
// which makes the output undeterministic!
// WalkWithSymlinks does follow symbolic links!
func WalkWithSymlinks(root string, walkFn filepath.WalkFunc) error {
	info, err := os.Stat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(root, info, walkFn, true)
}

// readDirNames reads the directory named by dirname and returns
// a channel for future results.
func readDirNames(dirname string) (<-chan dirNames, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	c := make(chan dirNames)

	go func() {
		defer f.Close()
		defer close(c)

		for {
			names, err := f.Readdirnames(1024)
			if err != nil {
				if err == io.EOF {
					if len(names) > 0 {
						c <- dirNames{names: names}
					}
					return
				}
				c <- dirNames{err: err}
				return
			}
			c <- dirNames{names: names}
		}
	}()

	return c, nil
}

type dirNames struct {
	names []string
	err   error
}
