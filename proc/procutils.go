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
	"time"
)

// Log is discarded by default
var Log = func(keyvals ...interface{}) error { return nil }

// ErrTimedOut is an error for child timeout
var ErrTimedOut = errors.New("child timed out")

// IntTimeout is the duration to wait before Kill after Int
var IntTimeout = 3 * time.Second

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
		procAttrSetGroup(cmd)
	}
	gcmd := &gCmd{Cmd: cmd}
	if err := gcmd.Start(); err != nil {
		return err
	}
	if timeoutSeconds <= 0 {
		return <-gcmd.done
	}

	select {
	case err := <-gcmd.done:
		return err
	case <-time.After(time.Second * time.Duration(timeoutSeconds)):
		Log("msg", "killing timed out", "pid", cmd.Process.Pid, "path", cmd.Path, "args", cmd.Args)
		if killErr := familyKill(gcmd.Cmd, true); killErr != nil {
			Log("msg", "interrupt", "pid", cmd.Process.Pid)
		}
		select {
		case <-gcmd.done:
		case <-time.After(IntTimeout):
			familyKill(gcmd.Cmd, false)
		}
	}
	return ErrTimedOut
}

// KillWithChildren kills the process
// and tries to kill its all children (process group)
func KillWithChildren(p *os.Process, interrupt bool) (err error) {
	//Log("msg","killWithChildren", "process", p)
	if p == nil {
		return
	}
	Log("msg", "killWithChildren", "pid", p.Pid, "interrupt", interrupt)
	defer func() {
		if r := recover(); r != nil {
			Log("msg", "PANIC in kill", "process", p, "error", r)
		}
	}()
	defer p.Release()
	if p.Pid == 0 {
		return nil
	}
	if interrupt {
		defer p.Signal(os.Interrupt)
		return Pkill(p.Pid, os.Interrupt)
	}
	defer p.Kill()
	return Pkill(p.Pid, os.Kill)
}

func groupKill(p *os.Process, interrupt bool) error {
	if p == nil {
		return nil
	}
	Log("msg", "groupKill", "pid", p.Pid)
	defer recover()
	if interrupt {
		defer p.Signal(os.Interrupt)
		return GroupKill(p.Pid, os.Interrupt)
	}
	defer p.Kill()
	return GroupKill(p.Pid, os.Kill)
}

func familyKill(cmd *exec.Cmd, interrupt bool) error {
	if cmd.SysProcAttr != nil && isGroupLeader(cmd) {
		return groupKill(cmd.Process, interrupt)
	}
	return KillWithChildren(cmd.Process, interrupt)
}

type gCmd struct {
	*exec.Cmd
	done chan error
}

func (c *gCmd) Start() error {
	if err := c.Cmd.Start(); err != nil {
		return err
	}
	c.done = make(chan error, 1)
	go func() { c.done <- c.Cmd.Wait() }()
	return nil
}
