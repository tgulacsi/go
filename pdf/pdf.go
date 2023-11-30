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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/google/renameio/v2"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var config = model.NewDefaultConfiguration()

func init() {
	config.ValidationMode = model.ValidationNone
}

var skipPdfunite, skipPdftk, skipGs int32

var errNotInstalled = errors.New("not installed")

func fileSize(fn string) int64 {
	if fi, err := os.Stat(fn); err == nil {
		size := fi.Size()
		slog.Debug("dest", slog.String("file", fn), slog.Int64("size", size))
		return size
	}
	return 0
}

// MergeFiles merges the given sources into dest.
func MergeFiles(ctx context.Context, dest string, sources ...string) (err error) {
	if len(sources) == 0 {
		slog.Warn("MergeFiles", "sources", len(sources))
		return nil
	}

	_ = os.Remove(dest)
	destfh, err := renameio.NewPendingFile(dest)
	if err != nil {
		return err
	}
	defer destfh.Cleanup()

	if len(sources) == 1 {
		fh, err := os.Open(sources[0])
		if err != nil {
			return fmt.Errorf("%s: %w", sources[0], err)
		}
		defer fh.Close()
		if _, err = io.Copy(destfh, fh); err != nil {
			return fmt.Errorf("%s: %w", fh.Name(), err)
		}
		return destfh.CloseAtomicallyReplace()
	}

	if err = api.MergeAppendFile(sources, dest, config); err == nil {
		size := fileSize(dest)
		if size > 5 {
			return nil
		}
	}
	slog.Warn("MergeAppendFile", "error", err)

	if err := mergePDFFiles(ctx, destfh, sources...); err != nil {
		return err
	}
	return destfh.CloseAtomicallyReplace()
}

// mergePDFFiles merges the given PDF files, writes the merged
// into the given writer. Uses "pdfunite", from poppler-tools.
func mergePDFFiles(ctx context.Context, w io.Writer, sourceFile ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var lastErr error
	var hasAny bool
	for _, P := range []struct {
		Merge func(context.Context, io.Writer, ...string) error
		Skip  *int32
		Name  string
	}{
		{Name: "pdfunite", Merge: mergePDFFilesPdfunite, Skip: &skipPdfunite},
		{Name: "pdftk", Merge: mergePDFFilesPdftk, Skip: &skipPdftk},
		{Name: "gs", Merge: mergePDFFilesGs, Skip: &skipGs},
	} {
		if P.Skip == nil || atomic.LoadInt32(P.Skip) == 0 {
			hasAny = true
			if err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("panic", "error", r)
						err = fmt.Errorf("PANIC: %+v", r)
					}
				}()
				if tr, ok := w.(interface {
					Truncate(int64) error
					io.Seeker
				}); ok {
					if err = tr.Truncate(0); err != nil {
						return err
					}
				}
				if sk, ok := w.(io.Seeker); ok {
					if _, err = sk.Seek(0, 0); err != nil {
						return err
					}
				}
				cw := countingWriter{w: w}
				err = P.Merge(ctx, &cw, sourceFile...)
				slog.Info("check tools",
					slog.String("program", P.Name),
					slog.Int64("size", cw.n),
					slog.Any("source", sourceFile),
					slog.Any("error", err))
				if err == nil && cw.n > 5 {
					return nil
				} else if err != nil && errors.Is(err, errNotInstalled) {
					if P.Skip != nil {
						atomic.StoreInt32(P.Skip, 1)
					}
				}
				return err
			}(); err != nil {
				lastErr = err
			} else {
				return nil
			}
		}
	}
	if lastErr == nil && !hasAny {
		return errNotInstalled
	}
	return lastErr
}

func mergePDFFilesPdfunite(ctx context.Context, w io.Writer, sourceFile ...string) error {
	args := make([]string, 0, len(sourceFile)+1)
	args = append(args, sourceFile...)
	dst, err := os.CreateTemp("", ".pdf")
	if err != nil {
		return err
	}
	dst.Close()
	defer os.Remove(dst.Name())
	args = append(args, dst.Name())

	var errBuf bytes.Buffer
	cmd := exec.Command("pdfunite", args...)
	cmd.Stdout = &errBuf
	cmd.Stderr = &errBuf
	if err = cmd.Run(); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return errNotInstalled
		}
		return fmt.Errorf("%s: %w", errBuf.String(), err)
	}
	fh, err := os.Open(dst.Name())
	if err != nil {
		return err
	}
	_, err = io.Copy(w, fh)
	return err
}

func mergePDFFilesPdftk(ctx context.Context, w io.Writer, sourceFile ...string) error {
	outFh, err := os.CreateTemp("", "mergepdf-out-")
	if err != nil {
		return fmt.Errorf("mergePDFFiles out: %w", err)
	}
	defer os.Remove(outFh.Name())
	defer outFh.Close()
	args := make([]string, 0, len(sourceFile)+3)
	args = append(args, sourceFile...)
	args = append(args, "cat", "output", outFh.Name())
	var errBuf bytes.Buffer
	cmd := exec.Command("pdftk", args...)
	cmd.Stdout = &errBuf
	cmd.Stderr = &errBuf
	if err = cmd.Run(); err != nil {
		errBuf.WriteByte('\n')
		cmdLs := exec.Command("file", sourceFile...)
		cmdLs.Stdout = &errBuf
		cmdLs.Stderr = &errBuf
		if errLs := cmdLs.Run(); errLs != nil {
			slog.Error("mergePDFFilesPdftk", "args", cmdLs.Args, "error", errLs)
		}
		return fmt.Errorf("%q: %s: %w", cmd.Args, errBuf.Bytes(), err)
	}
	if _, err = io.Copy(w, outFh); err != nil {
		return fmt.Errorf("%s: %w", outFh.Name(), err)
	}
	return nil
}

func mergePDFFilesGs(ctx context.Context, w io.Writer, sourceFile ...string) error {
	args := make([]string, 0, 5+len(sourceFile))
	//gs -dBATCH -dNOPAUSE -q -sDEVICE=pdfwrite -sOutputFile=finished.pdf file1.pdf file2.pdf
	args = append(args, "-dBATCH", "-dNOPAUSE", "-q", "-sDEVICE=pdfwrite", "-sOutputFile=-")
	args = append(args, sourceFile...)
	var buf, errBuf bytes.Buffer
	cmd := exec.Command("gs", args...)
	cmd.Stdout = &buf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		slog.Error("gs", "args", args, "error", err)
		if strings.Contains(err.Error(), "executable file not found") {
			return errNotInstalled
		}
		return fmt.Errorf("%s: %w", errBuf.String(), err)
	}
	_, err := io.Copy(w, bytes.NewReader(buf.Bytes()))
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

type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}
