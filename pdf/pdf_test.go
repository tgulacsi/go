// Copyright 2023 Tamás Gulácsi. All rights reserved.

package pdf_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/go/pdf"
)

func TestMergePDF(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	slog.SetDefault(logger)
	ctx := zlog.NewSContext(context.Background(), logger)
	dis, err := os.ReadDir("testdata")
	if len(dis) == 0 {
		t.Fatalf("read testdata: %+v", err)
	}
	pdfs := make([]string, 0, len(dis))
	for _, di := range dis {
		if strings.HasSuffix(di.Name(), ".pdf") {
			pdfs = append(pdfs, filepath.Join("testdata", di.Name()))
		}
	}
	if len(pdfs) == 0 {
		t.Skip("no pdfs")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	destfh, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		t.Fatalf("CreateTemp: %+v", err)
	}
	defer destfh.Close()
	defer os.Remove(destfh.Name())
	if err := pdf.MergeFiles(ctx, destfh.Name(), pdfs...); err != nil {
		t.Fatalf("MergeFiles: %+v", err)
	}
}
