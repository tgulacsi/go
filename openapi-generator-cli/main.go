// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package main of openapi-generator-cli is
// a Go installable wrapper for openapitools.org's
// openapi-generator-cli, or kiota (aka.ms/get/kiota)
//
// This allows us to use it as a go tool (go get -tool github.com/tgulacsi/go/openapi-generator-cli; go tool openapi-generator-cli ...)
//
// If not there yet, it downloads the jar, then runs it.
package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/tgulacsi/go/iohlp"
	"github.com/tgulacsi/go/maven"
)

// https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli
// https://aka.ms/get/kiota/latest/linux-x64.zip
func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

type Type uint8

const (
	OpenAPI Type = iota + 1
	Kiota   Type = iota + 1
)

func (t Type) FileName() string {
	switch t {
	case OpenAPI:
		return "openapi-generator-cli.jar"
	case Kiota:
		return "kiota"
	}
	return ""
}

func (t Type) Command(ctx context.Context, binary string, args []string) *exec.Cmd {
	switch t {
	case Kiota:
		return exec.CommandContext(ctx, binary, args...)
	case OpenAPI:
		jArgs, oArgs := make([]string, 0, len(args)), make([]string, 0, len(args))
		for _, a := range args {
			if strings.HasPrefix(a, "-D") {
				jArgs = append(jArgs, a)
			} else {
				oArgs = append(oArgs, a)
			}
		}
		return exec.CommandContext(ctx, "java",
			append(append(append(
				make([]string, 0, len(jArgs)+2+len(oArgs)),
				jArgs...),
				"-jar", binary),
				oArgs...)...)
	}
	return nil
}

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	args := os.Args[1:]
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		dir = os.ExpandEnv("$HOME/.cache")
	}
	typ := OpenAPI
	if len(args) != 0 {
		switch args[0] {
		case "openapi":
			args = args[1:]
		case "kiota":
			args = args[1:]
			typ = Kiota
		}
	}
	binary := filepath.Join(dir, typ.FileName())

	if fi, err := os.Stat(binary); err != nil || fi.Size() == 0 {
		fh, err := os.Create(binary)
		if err != nil {
			return fmt.Errorf("create %s: %w", binary, err)
		}
		defer fh.Close()
		if err = typ.Download(ctx, fh); err != nil {
			return fmt.Errorf("download: %w", err)
		}
		if err = fh.Close(); err != nil {
			return err
		}
	}

	cmd := typ.Command(ctx, binary, args)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func (t Type) Download(ctx context.Context, w io.Writer) error {
	if t == Kiota {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://aka.ms/get/kiota/latest/linux-x64.zip", nil)
		if err != nil {
			return fmt.Errorf("GET: %w", err)
		}
		log.Println("Downloading " + req.URL.String() + " ...")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("GET %s: %w", req.URL, err)
		}
		sr, err := iohlp.MakeSectionReader(resp.Body, 1<<20)
		resp.Body.Close()
		if err != nil {
			return err
		}
		zr, err := zip.NewReader(sr, sr.Size())
		if err != nil {
			return err
		}
		names := make([]string, 0, len(zr.File))
		for _, f := range zr.File {
			if f.Name == "kiota" {
				r, err := f.Open()
				if err != nil {
					return err
				}
				if _, err = io.Copy(w, r); err != nil {
					return err
				}
				if fh, ok := w.(interface{ Name() string }); ok {
					if err = os.Chmod(fh.Name(), 0755); err != nil {
						return err
					}
				}
				return nil
			}
			names = append(names, f.Name)
		}
		return fmt.Errorf("kiota not found in zip (only %s)", strings.Join(names, ","))
	}

	mConf := maven.Config{}
	metadata, err := mConf.GetMetadata(ctx, "org.openapitools/openapi-generator-cli")
	if err != nil {
		return err
	}

	log.Println("latest:", metadata.Latest)

	return mConf.DownloadJar(ctx, w, metadata, "")
}
