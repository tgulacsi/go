// +build !windows

/*
Copyright 2015 Tamás Gulácsi

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
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func isGroupLeader(c *exec.Cmd) bool {
	return c.SysProcAttr != nil && c.SysProcAttr.Setpgid
}

// Pkill kills the process with the given pid, or just -INT if interrupt is true.
func Pkill(pid int, signal os.Signal) error {
	signum := signal.(syscall.Signal)

	var err error
	defer func() {
		if r := recover(); r == nil && err == nil {
			return
		}
		err = exec.Command("pkill", "-"+strconv.Itoa(int(signum)),
			"-P", strconv.Itoa(pid)).Run()
	}()
	err = syscall.Kill(pid, signum)
	return err
}

// GroupKill kills the process group lead by the given pid
func GroupKill(pid int, signal os.Signal) error {
	return Pkill(-pid, signal)
}
