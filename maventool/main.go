// Copyright 2025 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Package main of maveontool is
// a Go installable wrapper for any maven-downloadable jar.
//
// This allows us to use it as a go tool (go get -tool github.com/tgulacsi/go/maventool; go tool maventool org.openapitools/openapi-generator-cli ...)
//
// If not there yet, it downloads the jar, then runs it.
// If the first argument is --print, then just prints the jar's path.
// Otherwise, calls java -jar pkg.jar [args...]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/tgulacsi/go/maven"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	args := os.Args[1:]
	justPrint := len(args) > 1 && args[0] == "--print"
	if justPrint {
		args = args[1:]
	}
	pkg, version, _ := strings.Cut(args[0], "@")
	binary, err := maven.Config{}.Get(ctx, pkg, version)
	if err != nil {
		return err
	}

	if justPrint {
		fmt.Println(binary)
		return nil
	}

	cmd := exec.CommandContext(ctx, "java", append(append(make([]string, 0, 2+len(args)-1), "-jar", binary), args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
