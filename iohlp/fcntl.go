// +build posix linux !windows

// Copyright 2021 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

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
