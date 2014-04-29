// +build unsafe

// Copyright (c) 2014, Tamás Gulácsi
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package azip

import (
	"archive/zip"
	"io"
	"unsafe"
)

// These are dirty hacks - to work it assumes that the archive/zip 's
// structures looks like these.

// localFile is a copy of archive/zip 's File.
type localFile struct {
	zip.FileHeader
	zipr         io.ReaderAt
	zipsize      int64
	headerOffset int64
}

// getHeaderOffset returns the header offset of the file (only data offset is published).
func getHeaderOffset(f *zip.File) int64 {
	return (*localFile)(unsafe.Pointer(f)).headerOffset
}

// localWriter is an even dirtier hack: it assmes that the archive/zip 's
// Writer struct's first element is an *zip.countWriter.
type localWriter struct {
	cw *localCountWriter
}

// localCountWriter is the copy of the archive/zip 's countWriter
type localCountWriter struct {
	w     io.Writer
	count int64
}

// setCountOfWriter sets the zip.Writer's underlying countWriter's count.
// Returns the previously set count.
func setCountOfWriter(w *zip.Writer, count int64) int64 {
	cw := ((*localWriter)(unsafe.Pointer(w))).cw
	oldCount := cw.count
	cw.count = count
	return oldCount
}

func init() {
	NewZipWriterOffset = func(w io.Writer, offset int64) ZipWriter {
		zw := zip.NewWriter(w)
	}
	zw.setCountOfWriter(offset)
	return zw
}
