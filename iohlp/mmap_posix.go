// +build posix linux !windows

/*
Copyright 2019 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iohlp

import (
	"syscall"
)

func (r *ReaderAt) mmap(fd uintptr, size int) error {
	var err error
	r.data, err = syscall.Mmap(int(fd), 0, size,
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	return err
}

func (r *ReaderAt) munmap() error {
	if r == nil || r.data == nil {
		return nil
	}
	data := r.data
	r.data = nil
	return syscall.Munmap(data)
}
