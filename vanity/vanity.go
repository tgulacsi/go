// Copyright 2018 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	errors "golang.org/x/xerrors"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

var templates *template.Template

func Main() error {
	flagServe := flag.String("http", "", "serve the pages at this address, not generate")
	flagDestDir := flag.String("dest", ".", "destination directory for the generated files")
	flag.Parse()

	templates = template.Must(template.New("").Parse(tmpl))

	repos := make(map[string]Repo)
	if _, err := toml.DecodeFile(flag.Arg(0), &repos); err != nil {
		return err
	}
	for nm, r := range repos {
		if r.VCS == "" {
			r.VCS = "git"
			repos[nm] = r
		}
	}

	if *flagServe != "" {
		for nm, r := range repos {
			http.Handle("/"+nm, r)
		}
		return http.ListenAndServe(*flagServe, nil)
	}

	for nm, r := range repos {
		fh, err := os.Create(filepath.Join(*flagDestDir, nm))
		if err != nil {
			return errors.Errorf("%w", err)
		}
		if err = templates.Execute(fh, r); err != nil {
			fh.Close()
			return err
		}
		if err = fh.Close(); err != nil {
			return errors.Errorf("%s: %w", fh.Name(), err)
		}
	}
	return nil
}

type Repo struct {
	Vanity, Get, Page, VCS string
}

func (repo Repo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

const tmpl = `<!DOCTYPE html>
<html>
    <head>
        <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
        <meta name="go-import" content="{{.Vanity}} {{.VCS}} {{.Get}}">
        <meta http-equiv="refresh" content="0; url={{.Page}}">
    </head>
    <body>
        Nothing to see here; <a href="{{.Page}}">move along</a>.
    </body>
</html>`
