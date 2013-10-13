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

/*
Package proc is process utilities
*/
package proc

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/golang/glog"
)

// ErrTimedOut is an error for child timeout
var ErrTimedOut = errors.New("Child timed out.")

// WithTimeout starts todo function, executes onTimeout if the todo function
// does not return before timeoutSeconds elapses, and returns todo's returned
// error or ErrTimedOut
func WithTimeout(timeoutSeconds int, todo func() error, onTimeout func() error) error {
	if timeoutSeconds <= 0 {
		return todo()
	}
	errch := make(chan error, 1)
	go func() {
		errch <- todo()
	}()
	timech := time.After(time.Second * time.Duration(timeoutSeconds))
	var err error
	select {
	case err = <-errch:
	case <-timech:
		err = ErrTimedOut
		func() {
			defer func() {
				recover()
			}()
			onTimeout()
		}()
	}
	return err
}

// RunWithTimeout runs cmd, and kills the child on timeout
func RunWithTimeout(timeoutSeconds int, cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid:   true, // to be able to kill all children, too
			Pdeathsig: syscall.SIGKILL,
		}
	}
	return WithTimeout(timeoutSeconds, cmd.Run, newFamilyKiller(cmd))
}

// KillWithChildren kills the process
// and tries to kill its all children (process group)
func KillWithChildren(p *os.Process) (err error) {
	glog.V(1).Infof("killWithChildren(%s)", p)
	if p == nil {
		return
	}
	glog.Infof("killWithChildren p.Pid=%d", p.Pid)
	defer func() {
		if r := recover(); r != nil {
			glog.Warningf("PANIC in kill %s: %s", p, r)
		}
	}()
	defer p.Release()
	if p.Pid == 0 {
		return nil
	}
	defer p.Kill()
	return exec.Command("pkill", "-KILL", "-P", strconv.Itoa(p.Pid)).Run()
}

func groupKill(p *os.Process) error {
	if p == nil {
		return nil
	}
	glog.Infof("groupKill p.Pid=%d", p.Pid)
	defer recover()
	defer p.Kill()
	return exec.Command("kill", "-KILL", "-"+strconv.Itoa(p.Pid)).Run()
}

func simpleKill(p *os.Process) error {
	if p == nil {
		return nil
	}
	glog.Infof("killing %d", p.Pid)
	defer recover()
	return p.Kill()
}

func newFamilyKiller(cmd *exec.Cmd) func() error {
	return func() error {
		if cmd != nil {
			glog.Infof("killing timed out (%d) %s %s", cmd.Process.Pid, cmd.Path, cmd.Args)
			if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
				return groupKill(cmd.Process)
			}
			return KillWithChildren(cmd.Process)
		}
		return nil
	}
}
