// +build posix linux !windows

package iohlp

import (
	"os"
	"syscall"
)

// SetDirect sets the O_DIRECT flag on the os.File.
func SetDirect(f *os.File) error {
	return Fcntl(uintptr(f.Fd()), syscall.F_SETFL, syscall.O_DIRECT)
}

// Fcntl calls the fcntl command with the flags.
func Fcntl(fd uintptr, cmd, flags int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, uintptr(cmd), uintptr(flags))
	if errno == 0 {
		return nil
	}
	return errno
}
