// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package main of openapi-generator-cli is
// a Go installable wrapper for openapitools.org's
// openapi-generator-cli.
//
// This allows us to use it as a go tool (go get -tool github.com/tgulacsi/go/openapi-generator-cli; go tool openapi-generator-cli ...)
//
// If not there yet, it downloads the jar, then runs it.
package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
)

// https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli
func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		dir = os.ExpandEnv("$HOME/.cache")
	}
	jar := filepath.Join(dir, "openapi-generator-cli.jar")
	if fi, err := os.Stat(jar); err != nil || fi.Size() == 0 {
		fh, err := os.Create(jar)
		if err != nil {
			return fmt.Errorf("create %s: %w", jar, err)
		}
		if err := download(ctx, fh); err != nil {
			return fmt.Errorf("download: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "java",
		append(append(make([]string, 0, 2+len(os.Args)-1),
			"-jar", jar),
			os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func download(ctx context.Context, w io.Writer) error {
	baseURL, err := url.Parse("https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/")
	if err != nil {
		return err
	}

	xmlURL := baseURL.JoinPath("./maven-metadata.xml")
	req, err := http.NewRequestWithContext(ctx, "GET", xmlURL.String(), nil)
	if err != nil {
		return fmt.Errorf("GET %s: %w", xmlURL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", req.URL, err)
	}
	var metadata struct {
		GroupID     string   `xml:"groupId"`
		ArtifactID  string   `xml:"artifactId"`
		Latest      string   `xml:"versioning>latest"`
		Release     string   `xml:"versioning>release"`
		Versions    []string `xml:"versioning>versions>version"`
		LastUpdated string   `xml:"versioning>lastUpdated"`
	}
	err = xml.NewDecoder(resp.Body).Decode(&metadata)
	resp.Body.Close()
	if err != nil {
		return err
	}
	log.Println("latest:", metadata.Latest)

	jarURL := baseURL.JoinPath("./" + metadata.Latest + "/openapi-generator-cli-" + metadata.Latest + ".jar")
	if req, err = http.NewRequestWithContext(ctx, "GET", jarURL.String(), nil); err != nil {
		return fmt.Errorf("%s: %w", jarURL, err)
	}
	log.Println("Downloading " + jarURL.String() + " ...")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		return fmt.Errorf("GET %s: %w", req.URL, err)
	}
	_, err = io.Copy(w, resp.Body)
	resp.Body.Close()
	return err
}
