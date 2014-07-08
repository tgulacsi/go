// Copyright 2014 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package uncompr

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tgulacsi/go/temp"
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
	rar   string
	files []string
}

// NewRarLister copies the contents of the io.Reader into "rar.rar"
// under a temp directory, which is used for extraction, too.
func NewRarLister(r io.Reader) (Lister, error) {
	tempdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	rl := rarLister{rar: filepath.Join(tempdir, "rar.rar")}
	fh, err := os.Create(rl.rar)
	if err != nil {
		os.RemoveAll(tempdir)
		return nil, err
	}
	if _, err = io.Copy(fh, r); err != nil {
		os.RemoveAll(tempdir)
		return nil, err
	}
	if err = fh.Close(); err != nil {
		os.RemoveAll(tempdir)
		return nil, err
	}
	b, err := exec.Command("unrar", "l", rl.rar).Output()
	if err != nil {
		os.RemoveAll(tempdir)
		return nil, err
	}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("    ..A.... ")) {
			continue
		}
		j := bytes.LastIndex(line, []byte(" "))
		rl.files = append(rl.files, string(line[j+1:]))
	}

	return rl, nil
}

// Close of rarLister deletes the temp dir.
func (rl rarLister) Close() error {
	if rl.rar == "" { // already closed
		return nil
	}
	dir := filepath.Dir(rl.rar)
	rl.rar = ""
	return os.RemoveAll(dir)
}

// List lists the rar archive's contents (only files).
func (rl rarLister) List() []Extracter {
	ex := make([]Extracter, len(rl.files))
	for i, nm := range rl.files {
		ex[i] = rarExtracter{rar: rl.rar, name: nm}
	}
	return ex
}

type rarExtracter struct {
	rar, name string
}

// Open extracts the file from the rar archive.
func (re rarExtracter) Open() (io.ReadCloser, error) {
	cmd := exec.Command("unrar", "e", re.rar, re.name)
	cmd.Dir = filepath.Dir(re.rar)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	fn := filepath.Join(cmd.Dir, filepath.Base(re.name))
	fh, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	return unlinkCloser{fh}, nil
}

// Close deletes the underlying file.
func (re rarExtracter) Close() error {
	return os.Remove(filepath.Join(filepath.Dir(re.rar), re.name))
}

// Name returns the atchive item's name.
func (re rarExtracter) Name() string {
	return re.name
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
