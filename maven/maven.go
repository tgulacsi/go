// Copyright 2025 Tamás Gulácsi.
//
// SPDX-License-Identifier: Apache-2.0

// Package maven helps parse maven metadata
// and download jars.
package maven

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

var DefaultRepoURL = "https://repo1.maven.org/maven2"

type Metadata struct {
	GroupID     string   `xml:"groupId"`
	ArtifactID  string   `xml:"artifactId"`
	Latest      string   `xml:"versioning>latest"`
	Release     string   `xml:"versioning>release"`
	Versions    []string `xml:"versioning>versions>version"`
	LastUpdated string   `xml:"versioning>lastUpdated"`

	Org, Pkg string
	BaseURL  *url.URL `xml:"-"`
}

// Config for repo/HTTP client. Empty config is usable.
type Config struct {
	RepoURL, CacheDir string
	HTTPClient        *http.Client
}

// GetMetadata of the package. Emtpy config is usable.
//
// The pkg must be in com.github.user/package-name format.
func (conf Config) GetMetadata(ctx context.Context, pkg string) (Metadata, error) {
	if conf.RepoURL == "" {
		conf.RepoURL = DefaultRepoURL
	}
	var meta Metadata
	repoURL, err := url.Parse(conf.RepoURL)
	if err != nil {
		return meta, err
	}

	org, pkg, _ := strings.Cut(pkg, "/")
	baseURL := repoURL.JoinPath(append(append(append(
		make([]string, 0, 1+strings.Count(org, ".")+1),
		"."),
		strings.Split(org, ".")...),
		pkg,
	)...)
	xmlURL := baseURL.JoinPath("./maven-metadata.xml")
	req, err := http.NewRequestWithContext(ctx, "GET", xmlURL.String(), nil)
	if err != nil {
		return meta, fmt.Errorf("GET %s: %w", xmlURL, err)
	}
	meta.Pkg, meta.BaseURL = pkg, baseURL
	if conf.HTTPClient == nil {
		conf.HTTPClient = http.DefaultClient
	}
	log.Println("Downloading", req.URL.String(), "...")
	resp, err := conf.HTTPClient.Do(req)
	if err != nil {
		return meta, fmt.Errorf("GET %s: %w", req.URL, err)
	}
	err = xml.NewDecoder(resp.Body).Decode(&meta)
	resp.Body.Close()
	return meta, err
}

// DownloadJar the given version (latest if not given.)
func (conf Config) DownloadJar(ctx context.Context, w io.Writer, meta Metadata, version string) error {
	if version == "" {
		if version = meta.Latest; version == "" {
			return fmt.Errorf("%+v: empty version (Latest)", meta)
		}
	}
	if meta.Pkg == "" {
		return fmt.Errorf("%+v: empty Pkg", meta)
	}

	jarURL := meta.BaseURL.JoinPath(".", version, meta.Pkg+"-"+version+".jar")
	req, err := http.NewRequestWithContext(ctx, "GET", jarURL.String(), nil)
	if err != nil {
		return fmt.Errorf("GET: %w", err)
	}
	log.Println("Downloading " + req.URL.String() + " ...")
	if conf.HTTPClient == nil {
		conf.HTTPClient = http.DefaultClient
	}
	resp, err := conf.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", req.URL, err)
	}
	_, err = io.Copy(w, resp.Body)
	resp.Body.Close()
	return err
}

// Get check whether the given version exists in the cache,
// - if not, then downloads the metadata, then the given/latest jar,
// - stores it in the cache
// gives its path back from the cache.
func (conf Config) Get(ctx context.Context, pkg, version string) (string, error) {
	if conf.CacheDir == "" {
		conf.CacheDir = os.Getenv("XDG_CACHE_HOME")
		if conf.CacheDir == "" {
			conf.CacheDir = os.ExpandEnv("$HOME/.cache")
		}
		conf.CacheDir = filepath.Join(conf.CacheDir, "maven")
	}

	F := func(pkg, version string) string {
		return filepath.Join(conf.CacheDir, pkg+"-"+version+".jar")
	}
	isOK := func(fn string) error {
		fi, err := os.Stat(fn)
		if err != nil {
			return err
		} else if fi.Size() < 1024 {
			return fmt.Errorf("too small: %s.size=%d", fi.Name(), fi.Size())
		}
		return nil
	}
	if version != "" {
		_, pkg, _ := strings.Cut(pkg, "/")
		fn := F(pkg, version)
		if isOK(fn) == nil {
			return fn, nil
		}
	}
	meta, err := conf.GetMetadata(ctx, pkg)
	if err != nil {
		return "", err
	}
	if version == "" {
		version = meta.Latest
	}
	fn := F(meta.Pkg, version)
	if isOK(fn) == nil {
		return fn, nil
	}
	fh, err := renameio.NewPendingFile(fn, renameio.WithPermissions(0644))
	if err != nil {
		return "", err
	}
	defer fh.Close()
	if err := conf.DownloadJar(ctx, fh, meta, version); err != nil {
		return "", err
	}
	return fn, fh.CloseAtomicallyReplace()
}
