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
	"fmt"
	"io"
	"strings"
)

// AppendZip appends the contents of src to the given ZipWriter.
// Does NOT close the ZipWriter, it is the caller's responsibility!
func AppendZip(zw ZipWriter, zr *zip.ReadCloser) error {
	for _, f := range zr.File {
		w, err := zw.CreateHeader(&f.FileHeader)
		if err != nil {
			return fmt.Errorf("error creating header for %q: %v", f.FileHeader.Name, err)
		}
		src, err := f.Open()
		if err != nil {
			return fmt.Errorf("error opening %s: %v", f.Name, err)
		}
		// We always get "zip: checksum error", but the resulting file is OK.
		if _, err = io.Copy(w, src); err != nil && !strings.HasSuffix(err.Error(), "checksum error") {
			src.Close()
			return fmt.Errorf("error copying from %s: %v", f.Name, err)
		}
		src.Close()
	}
	return nil
}

// AppendZipFile appends the contents of the source zip file to the given ZipWriter.
// For details see AppendZip.
func AppendZipFile(zw ZipWriter, source string) error {
	zr, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("cannot open source zip %q: %v", source, err)
	}
	if len(zr.File) == 0 {
		zr.Close()
		return nil
	}

	err = AppendZip(zw, zr)
	zr.Close()
	return err
}
