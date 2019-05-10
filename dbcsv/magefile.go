// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
var Default = Build
var _ = Default

// A build step that requires additional params, or platform specific steps for example
func Build() error {
	fmt.Println("Building...")
	cmd := exec.Command("go", "install", "./csvload", "./csvdbforeach", "./csvdump")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
