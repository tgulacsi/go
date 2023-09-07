/*
  Copyright 2019, 2023 Tamás Gulácsi

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
	"log/slog"
	"os"
	"os/exec"

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
func MergeFiles(ctx context.Context, dest string, sources ...string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic", "error", r)
			err = fmt.Errorf("PANIC: %+v", r)
		}
	}()
	_ = os.Remove(dest)
	fileSize := func(fn string) int64 {
		if fi, err := os.Stat(fn); err == nil {
			size := fi.Size()
			slog.Info("dest", slog.String("file", fn), slog.Int64("size", size))
			return size
		}
		return 0
	}

	if err = api.MergeAppendFile(sources, dest, config); err == nil {
		size := fileSize(dest)
		if size > 5 {
			return nil
		}
	}
	slog.Warn("MergeAppendFile", "error", err)

	_ = os.Remove(dest)
	if path, pathErr := exec.LookPath("pdfunite"); pathErr != nil {
		slog.Warn("pdfunite is not found")
		return err
	} else if err := exec.CommandContext(ctx,
		path,
		append(append(make([]string, 0, len(sources)+1),
			sources...), dest)...,
	).Run(); err == nil {
		if fileSize(dest) > 5 {
			return nil
		}
		slog.Warn("empty", "file", dest)
	} else {
		slog.Warn(path, "error", err)
	}
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
