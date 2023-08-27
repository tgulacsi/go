/*
  Copyright 2019, 2020 Tamás Gulácsi

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

package pdf

import (
	"context"
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Log is used for logging.
var Log = func(...interface{}) error { return nil }

var config = model.NewDefaultConfiguration()

func init() {
	config.ValidationMode = model.ValidationNone
}

// MergeFiles merges the given sources into dest.
func MergeFiles(dest string, sources ...string) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %+v", r)
		}
	}()
	err = api.MergeAppendFile(sources, dest, config)
	return err
}

// Split the pdf - each page into different file
func Split(ctx context.Context, destDir, fn string) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %+v", r)
		}
	}()
	err = api.SplitFile(fn, destDir, 1, config)
	return err
}

// PageNum returns the number of pages in the document.
func PageNum(ctx context.Context, fn string) (int, error) {
	fh, err := os.Open(fn)
	if err != nil {
		return 0, err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %+v", r)
		}
	}()
	pdf, err := api.ReadContext(fh, config)
	fh.Close()
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	if pdf.PageCount != 0 {
		return pdf.PageCount, nil
	}
	err = pdfcpu.OptimizeXRefTable(pdf)
	return pdf.PageCount, err
}
