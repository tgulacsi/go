/*
Copyright 2014 Tamás Gulácsi.

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

package punchhole

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

const (
	//http://source.winehq.org/source/include/winnt.h#L4605
	//file_read_data  = 1
	//file_write_data = 2

	// METHOD_BUFFERED	0
	method_buffered = 0
	// FILE_ANY_ACCESS   0
	file_any_access = 0
	// FILE_DEVICE_FILE_SYSTEM   0x00000009
	file_device_file_system = 0x00000009
	// FILE_SPECIAL_ACCESS   (FILE_ANY_ACCESS)
	file_special_access = file_any_access
	//file_read_access    = file_read_data
	//file_write_access = file_write_data

	// http://source.winehq.org/source/include/winioctl.h
	// #define CTL_CODE 	(  	DeviceType,
	//		Function,
	//		Method,
	//		Access  		 )
	//    ((DeviceType) << 16) | ((Access) << 14) | ((Function) << 2) | (Method)

	// FSCTL_SET_COMPRESSION   CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 16, METHOD_BUFFERED, FILE_READ_DATA | FILE_WRITE_DATA)
	//fsctl_set_compression = (file_device_file_system << 17) | ((file_read_access | file_write_access) << 14) | (16 << 2) | method_buffered
	// FSCTL_SET_SPARSE   CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 49, METHOD_BUFFERED, FILE_SPECIAL_ACCESS)
	fsctl_set_sparse = (file_device_file_system << 16) | (file_special_access << 14) | (49 << 2) | method_buffered
	// FSCTL_SET_ZERO_DATA   CTL_CODE(FILE_DEVICE_FILE_SYSTEM, 50, METHOD_BUFFERED, FILE_WRITE_DATA)
	fsctl_set_zero_data = (file_device_file_system << 16) | (file_write_data << 14) | (50 << 2) | method_buffered
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procDeviceIOControl = modkernel32.NewProc("DeviceIoControl")

	sparseFilesMu sync.Mutex
	sparseFiles   map[*os.File]struct{}
)

func init() {
	PunchHole = punchHoleWindows

	// sparseFiles is an fd set for already "sparsed" files - according to
	// msdn.microsoft.com/en-us/library/windows/desktop/aa364225(v=vs.85).aspx
	// the file handles are unique per process.
	sparseFiles = make(map[uintptr]struct{})
}

// http://msdn.microsoft.com/en-us/library/windows/desktop/aa364411%28v=vs.85%29.aspx
// typedef struct _FILE_ZERO_DATA_INFORMATION {
//  LARGE_INTEGER FileOffset;
//  LARGE_INTEGER BeyondFinalZero;
//} FILE_ZERO_DATA_INFORMATION, *PFILE_ZERO_DATA_INFORMATION;
type fileZeroDataInformation struct {
	FileOffset, BeyondFinalZero int64
}

// puncHoleWindows punches a hole into the given file starting at offset,
// measuring "size" bytes
// (http://msdn.microsoft.com/en-us/library/windows/desktop/aa364597%28v=vs.85%29.aspx)
func punchHoleWindows(file *os.File, offset, size int64) (err error) {
	if err := ensureFileSparse(file); err != nil {
		return err
	}

	lpInBuffer := fileZeroDataInformation{
		FileOffset:      offset,
		BeyondFinalZero: offset + size}
	lpBytesReturned := make([]byte, 8)
	// BOOL
	// WINAPI
	// DeviceIoControl( (HANDLE) hDevice,              // handle to a file
	//                  FSCTL_SET_ZERO_DATA,           // dwIoControlCode
	//                  (LPVOID) lpInBuffer,           // input buffer
	//                  (DWORD) nInBufferSize,         // size of input buffer
	//                  NULL,                          // lpOutBuffer
	//                  0,                             // nOutBufferSize
	//                  (LPDWORD) lpBytesReturned,     // number of bytes returned
	//                  (LPOVERLAPPED) lpOverlapped ); // OVERLAPPED structure
	r1, _, e1 := syscall.Syscall9(procDeviceIOControl.Addr(), 8,
		file.Fd(),
		uintptr(fsctl_set_zero_data),
		uintptr(unsafe.Pointer(&lpInBuffer)),
		16,
		0,
		0,
		uintptr(unsafe.Pointer(&lpBytesReturned[0])),
		0,
		0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// // http://msdn.microsoft.com/en-us/library/windows/desktop/cc948908%28v=vs.85%29.aspx
// type fileSetSparseBuffer struct {
//	 SetSparse bool
// }

func ensureFileSparse(file *os.File) (err error) {
	sparseFilesMu.Lock()
	fd := file.Fd()
	if _, ok := sparseFiles[fd]; ok {
		sparseFilesMu.Unlock()
		return nil
	}

	//lpInBuffer := fileSetSparseBuffer{SetSparse: true}
	lpBytesReturned := make([]byte, 8)
	// BOOL
	// WINAPI
	// DeviceIoControl( (HANDLE) hDevice,                      // handle to a file
	//                  FSCTL_SET_SPARSE,                      // dwIoControlCode
	//                  (PFILE_SET_SPARSE_BUFFER) lpInBuffer,  // input buffer
	//                  (DWORD) nInBufferSize,                 // size of input buffer
	//                  NULL,                                  // lpOutBuffer
	//                  0,                                     // nOutBufferSize
	//                  (LPDWORD) lpBytesReturned,             // number of bytes returned
	//                  (LPOVERLAPPED) lpOverlapped );         // OVERLAPPED structure
	r1, _, e1 := syscall.Syscall9(procDeviceIOControl.Addr(), 8,
		fd,
		uintptr(fsctl_set_sparse),
		// If the lpInBuffer parameter is NULL, the operation will behave the same as if the SetSparse member of the FILE_SET_SPARSE_BUFFER structure were TRUE. In other words, the operation sets the file to a sparse file.
		0, // uintptr(unsafe.Pointer(&lpInBuffer)),
		0, // 1,
		0,
		0,
		uintptr(unsafe.Pointer(&lpBytesReturned[0])),
		0,
		0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	} else {
		sparseFiles[fd] = struct{}{}
	}
	sparseFilesMu.Unlock()
	return err
}
