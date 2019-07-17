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

package pdf

import (
	"context"

	"github.com/pkg/errors"

	"github.com/hhrutter/pdfcpu/pkg/api"
	"github.com/hhrutter/pdfcpu/pkg/pdfcpu"
)

// Log is used for logging.
var Log = func(...interface{}) error { return nil }

// MergeFiles merges the given sources into dest.
func MergeFiles(dest string, sources ...string) error {
	err := api.MergeFile(sources, dest, pdfcpu.NewDefaultConfiguration())
	return err
}

// Split the pdf - each page into different file
func Split(ctx context.Context, destDir, fn string) error {
	err := api.SplitFile(fn, destDir, 1, pdfcpu.NewDefaultConfiguration())
	return err
}

// PageNum returns the number of pages in the document.
func PageNum(ctx context.Context, fn string) (int, error) {
	pdf, err := api.ReadContextFile(fn)
	if err != nil {
		return 0, errors.Wrap(err, "read")
	}
	if pdf.PageCount != 0 {
		return pdf.PageCount, nil
	}
	err = pdfcpu.OptimizeXRefTable(pdf)
	return pdf.PageCount, errors.Wrap(err, "optimize")
}
