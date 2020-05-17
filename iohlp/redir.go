package iohlp

import (
	"os"
	"syscall"
)

// RedirFd redirects the fd, returns a reader pipe for the written data,
// and a cleanup function that reverses the effect of this function
// (closes the writer and redirects the fd).
func RedirFd(fd int) (*os.File, func() error, error) {
	// Clone File to origFile.
	origFd, err := syscall.Dup(fd)
	if err != nil {
		return nil, nil, err
	}

	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}

	// Clone the pipe's writer to the actual fd; from this point
	// on, writes to fd will go to w.
	if err = syscall.Dup2(int(w.Fd()), fd); err != nil {
		return nil, nil, err
	}

	return r, func() error {
		// Close writer
		w.Close()
		syscall.Close(fd)
		// Copy back
		err := syscall.Dup2(origFd, fd)
		syscall.Close(origFd)
		return err
	}, nil
}
