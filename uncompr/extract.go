// Copyright 2017 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package uncompr

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tgulacsi/go/temp"
	errors "golang.org/x/xerrors"
)

// Lister can list the archive's contents.
type Lister interface {
	List() []Extracter
	Close() error
}

// Extracter can extracts its file's content.
type Extracter interface {
	Name() string
	Open() (io.ReadCloser, error)
	Close() error
}

type zipLister struct {
	*zip.Reader
}

// List of zipLister implements List for zip archives.
func (zl zipLister) List() []Extracter {
	ex := make([]Extracter, len(zl.Reader.File))
	for i, f := range zl.File {
		ex[i] = zipExtracter{f}
	}
	return ex
}

// Close for zipLister does nothing.
func (zl zipLister) Close() error { return nil }

type zipExtracter struct {
	*zip.File
}

// Open of zipExtracter is zip.File.Open
func (ze zipExtracter) Open() (io.ReadCloser, error) {
	return ze.File.Open()
}

// Name returns the archived item's name.
func (ze zipExtracter) Name() string {
	return ze.File.Name
}

// Close of zipExtracter does nothing.
func (ze zipExtracter) Close() error { return nil }

// NewZipLister slurps the reader and returns a Listere for the zip.
func NewZipLister(r io.Reader) (Lister, error) {
	rsc, err := temp.MakeReadSeekCloser("", r)
	if err != nil {
		return nil, err
	}
	n, err := rsc.Seek(0, 2)
	if err != nil {
		return nil, err
	}
	if _, err = rsc.Seek(0, 0); err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(rsc, n)
	if err != nil {
		return nil, err
	}
	return zipLister{zr}, nil
}

type rarLister struct {
	dir   string
	files []string
}

// NewRarLister copies the contents of the io.Reader into "rar.rar"
// under a temp directory, which is used for extraction, too.
func NewRarLister(r io.Reader) (Lister, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	const rarName = "rar.rar"
	rl := rarLister{dir: tempdir}
	fh, err := os.Create(filepath.Join(rl.dir, rarName))
	if err != nil {
		os.RemoveAll(tempdir)
		return nil, errors.Errorf("create %s: %w", rarName, err)
	}
	if _, err = io.Copy(fh, r); err != nil {
		os.RemoveAll(tempdir)
		return nil, errors.Errorf("copy to %s: %w", fh.Name(), err)
	}
	if err = fh.Close(); err != nil {
		os.RemoveAll(tempdir)
		return nil, errors.Errorf("close %s: %w", fh.Name(), err)
	}

	cmd := exec.Command("unrar", "e", "-ep", rarName)
	cmd.Dir = tempdir
	if err = cmd.Run(); err != nil {
		os.RemoveAll(tempdir)
		return nil, errors.Errorf("%q @%q: %w", cmd.Args, cmd.Dir, err)
	}
	os.Remove(fh.Name())
	if err = filepath.Walk(
		tempdir,
		func(path string, info os.FileInfo, err error) error {
			if info.Mode().IsRegular() {
				rl.files = append(rl.files, path)
			}
			return nil
		},
	); err != nil {
		os.RemoveAll(tempdir)
		return nil, errors.Errorf("walk %s: %w", tempdir, err)
	}

	return rl, nil
}

// Close of rarLister deletes the temp dir.
func (rl rarLister) Close() error {
	if rl.dir == "" { // already closed
		return nil
	}
	rl.dir = ""
	return os.RemoveAll(rl.dir)
}

// List lists the rar archive's contents (only files).
func (rl rarLister) List() []Extracter {
	ex := make([]Extracter, len(rl.files))
	for i, path := range rl.files {
		ex[i] = rarExtracter{path: path}
	}
	return ex
}

type rarExtracter struct {
	path string
}

// Open extracts the file from the rar archive.
func (re rarExtracter) Open() (io.ReadCloser, error) {
	fh, err := os.Open(re.path)
	if err != nil {
		return nil, errors.Errorf("open %s: %w", re.path, err)
	}
	return unlinkCloser{fh}, nil
}

// Close deletes the underlying file.
func (re rarExtracter) Close() error {
	os.Remove(re.path)
	return nil
}

// Name returns the atchive item's name.
func (re rarExtracter) Name() string {
	return filepath.Base(re.path)
}

type unlinkCloser struct {
	*os.File
}

// Close of unlinkCloser deletes the underlying file on Close.
func (u unlinkCloser) Close() error {
	err := u.File.Close()
	err2 := os.Remove(u.File.Name())
	if err2 != nil && err == nil {
		err = err2
	}
	return err
}
