//go:build linux

// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

// Package smbwatch contains a watcher for SMB3 changes.
//
// See https://www.spinics.net/lists/linux-cifs/msg26807.html
package smbwatch

/*
#include <stdbool.h>  // bool
#include <stdint.h>  // uint32_t
#include <stdlib.h>  // malloc
#include <string.h>  // memset

// See MS-SMB2 2.2.35 for a definition of the individual filter flags
struct __attribute__((__packed__)) smb3_notify {
       uint32_t completion_filter;
       bool	watch_tree;
       uint32_t data_len;
       uint8_t	data[];
} __packed;

struct smb3_notify *new_smb3_notify(size_t data_len, bool watch_tree) {
	struct smb3_notify *pnotify;
	pnotify = malloc(sizeof(struct smb3_notify) + data_len);
	memset(pnotify, 0, sizeof(struct smb3_notify) + data_len);

	pnotify->watch_tree = watch_tree;
	pnotify->completion_filter = 0xFFF;
	pnotify->data_len = data_len;
	return pnotify;
}

uint8_t *get_smb3_notify_data(struct smb3_notify *pnotify, uint32_t *data_len) {
	*data_len = pnotify->data_len;
	return pnotify->data;
}
*/
import "C"

import (
	"fmt"
	"os"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// previous ioctl which simply returns when changes occur
	CIFS_IOC_NOTIFY = uint32(0x4005cf09)
	// new ioctl for change notification
	CIFS_IOC_NOTIFY_INFO = uint32(0xc009cf0b)
)

// WaitChange waits ONE change about the given file.
//
// To break the waiting, Close the file.
func WaitChange(fh *os.File) (string, error) {
	pnotify := C.new_smb3_notify(200, true)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL,
		fh.Fd(), uintptr(CIFS_IOC_NOTIFY_INFO), uintptr(unsafe.Pointer(pnotify)),
	); errno != 0 {
		return "", fmt.Errorf("%d returned from ioctl", errno)
	}
	var length C.uint32_t
	data := unsafe.Slice((*byte)(C.get_smb3_notify_data(pnotify, &length)), length)

	if len(data) < 12 {
		return "", nil
	}
	data = data[12:]
	// utf16 -> utf8
	d16 := make([]uint16, len(data)/2)
	var j int
	for i := range d16 {
		d16[i] = uint16(data[i*2+1])<<8 + uint16(data[i*2])
		if d16[i] == 0 {
			j = i + 1
		}
	}
	if j != 0 {
		// skip everything from the last zero byte (splits renames)
		d16 = d16[j:]
	}
	return string(utf16.Decode(d16)), nil
}
