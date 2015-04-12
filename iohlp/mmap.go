package iohlp

import (
	"errors"
	"os"
	"runtime"
	"syscall"
)

const MaxInt = int64(int(^uint(0) >> 1))

// MmapFile returns the mmap of the given path.
func MmapFile(fn string) ([]byte, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	p, err := Mmap(f)
	f.Close()
	return p, err
}

// Mmap returns a mmap of the given File
func Mmap(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxInt {
		return nil, errors.New("file too big to Mmap")
	}
	p, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(&p[0], func(_ interface{}) error { return syscall.Munmap(p) })
	return p, nil

}
