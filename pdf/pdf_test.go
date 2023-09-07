// Copyright 2023 Tamás Gulácsi. All rights reserved.

package pdf_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tgulacsi/go/pdf"
)

func TestMergePDF(t *testing.T) {
	dis, err := os.ReadDir("testdata")
	if len(dis) == 0 {
		t.Fatal(err)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	destfh, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer destfh.Close()
	defer os.Remove(destfh.Name())
	if err := pdf.MergeFiles(ctx, destfh.Name(), pdfs...); err != nil {
		t.Fatal(err)
	}
}
