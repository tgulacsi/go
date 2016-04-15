/*
Copyright 2013 Tamás Gulácsi

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

package proc

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func procAttrSetGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func isGroupLeader(c *exec.Cmd) bool {
	return c.SysProcAttr != nil &&
		c.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP > 0
}

// Pkill kills the process with the given pid
func Pkill(pid int, signal os.Signal) error {
	if signal == os.Kill {
		return exec.Command("taskkill", "/f", "/pid", strconv.Itoa(pid)).Run()
	}
	return exec.Command("taskkill", "/pid", strconv.Itoa(pid)).Run()
}

// GroupKill kills the process group lead by the given pid
func GroupKill(pid int, signal os.Signal) error {
	if signal == os.Kill {
		return exec.Command("taskkill", "/f", "/t", "/pid", strconv.Itoa(pid)).Run()
	}
	return exec.Command("taskkill", "/t", "/pid", strconv.Itoa(pid)).Run()
}

// Command returns an *exec.Cmd for calling the given cmd with args in cmd /c,
// with all the needed escapes - see
// https://blogs.msdn.microsoft.com/twistylittlepassagesallalike/2011/04/23/everyone-quotes-command-line-arguments-the-wrong-way/
func Command(cmd string, args ...string) *exec.Cmd {
	var buf bytes.Buffer
	for _, a := range args {
		io.WriteString(&buf, replShellMeta.Replace(syscall.EscapeArg(a)))
	}
	return exec.Command("cmd", "/c", buf.String())
}

// https://blogs.msdn.microsoft.com/twistylittlepassagesallalike/2011/04/23/everyone-quotes-command-line-arguments-the-wrong-way/
var replShellMeta = strings.NewReplacer(
	`(`, `^(`,
	`)`, `^)`,
	`%`, `^%`,
	`!`, `^!`,
	`^`, `^^`,
	`"`, `^"`,
	`<`, `^<`,
	`>`, `^>`,
	`&`, `^&`,
	`|`, `^|`,
)
