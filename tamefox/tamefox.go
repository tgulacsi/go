// Copyright 2020 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/tgulacsi/go/globalctx"
)

/*

#!/bin/sh
firefox=
swaymsg -m -t subscribe '["window"]' | \
	jq -r --unbuffered '.change +" "+  .container.app_id + " " + (.container.pid | tostring)' | \
	grep --line-buffered '^focus ' | \
	while read -r x app pid; do
		#echo "# x=$x app=$app pid=$pid" >&2
		if [ "$app" = 'firefox' ]; then
			echo "CONT $pid" >&2
			firefox=$pid
			kill -CONT $pid
			pkill -CONT -P $pid
		elif [ -n "$firefox" ]; then
			echo "STOP $firefox" >&2
			pkill -STOP -P $firefox
			kill -STOP $firefox
		fi
	done
*/
func main() {
	if err := Main(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func Main() error {
	flagTimeout := flag.Duration("t", 3*time.Second, "timeout for stop")
	flagProg := flag.String("prog", "firefox", "name of the program")
	flagDepth := flag.Int("depth", 0, "depth of child tree")
	flag.Parse()

	ctx, cancel := globalctx.Wrap(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "swaymsg", "-m", "-t", "subscribe", "[\"window\"]")
	pr, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}

	timeout := *flagTimeout
	var timer *time.Timer
	stopTimer := func() {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	var ff int
	defer func() {
		if ff != 0 {
			kill(ff, false, *flagDepth+1)
		}
	}()
	dec := json.NewDecoder(pr)
	for dec.More() {
		var change Change
		if err = dec.Decode(&change); err != nil {
			return err
		}
		log.Println(change)
		if change.Change != "focus" {
			continue
		}
		if change.Container.AppID == *flagProg {
			ff = change.Container.PID
			kill(ff, false, *flagDepth)
			stopTimer()
			continue
		}
		if timer == nil {
			timer = time.AfterFunc(timeout, func() {
				kill(ff, true, *flagDepth)
			})
			continue
		}
		stopTimer()
		timer.Reset(timeout)
	}
	return nil
}

type Change struct {
	Change    string    `json:"change"`
	Container Container `json:"container"`
}
type Container struct {
	AppID string `json:"app_id"`
	PID   int    `json:"pid"`
}

func kill(pid int, stop bool, depth int) error {
	var firstErr error
	if stop {
		const sig = syscall.SIGSTOP
		log.Println("STOP", pid)
		firstErr = syscall.Kill(pid, sig)
		if err := ckill(pid, sig, nil, depth); err != nil && firstErr == nil {
			firstErr = err
		}
	} else {
		log.Println("CONT", pid)
		const sig = syscall.SIGCONT
		firstErr = ckill(pid, sig, nil, depth)
		if err := syscall.Kill(pid, sig); err != nil && firstErr != nil {
			firstErr = err
		}
	}
	return firstErr
}

func ckill(ppid int, sig syscall.Signal, c map[int][]int, depth int) error {
	if depth == 0 {
		return syscall.Kill(ppid, sig)
	}
	fis, _ := ioutil.ReadDir("/proc")
	if c == nil {
		c = make(map[int][]int, len(fis))
		for _, fi := range fis {
			pid, err := strconv.Atoi(fi.Name())
			if err != nil {
				continue
			}
			ppid, err := getPPid(pid)
			if ppid == 1 {
				continue
			}
			if err != nil {
				return err
			}
			c[ppid] = append(c[ppid], pid)
		}
	}
	var firstErr error
	for _, pid := range c[ppid] {
		if err := ckill(pid, sig, c, depth-1); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := syscall.Kill(pid, sig); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
func pkill(pid int, sig syscall.Signal) error {
	//exec.Command("pkill", "-STOP", "-P", strconv.Itoa(pid)).Run()
	ppid, err := getPPid(pid)
	log.Printf("PPID of %d is %d (%v)", pid, ppid, err)
	if err != nil {
		return err
	}
	if err = syscall.Kill(pid, sig); err != nil {
		return err
	}
	if ppid == 1 {
		return nil
	}
	return pkill(ppid, sig)
}

func getPPid(pid int) (int, error) {
	b, err := ioutil.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	i := bytes.Index(b, []byte("\nPPid:"))
	if i < 0 {
		return 0, err
	}
	b = b[i+7:]
	i = bytes.IndexByte(b, '\n')
	if i >= 0 {
		b = b[:i]
	}
	return strconv.Atoi(string(bytes.TrimSpace(b)))
}
