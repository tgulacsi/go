// +build posix

package iohlp

import (
	"os"
	"syscall"
)

func SetDirect(f *os.File) error {
	return Fcntl(uintptr(f.Fd()), syscall.F_SETFL, syscall.O_DIRECT)
}

func Fcntl(fd uintptr, cmd, flags int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, uintptr(cmd), uintptr(flags))
	if errno == 0 {
		return nil
	}
	return errno
}
