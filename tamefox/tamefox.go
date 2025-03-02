// Copyright 2020, 2025 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tgulacsi/go/globalctx"
	"golang.org/x/sync/errgroup"
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
		log.SetOutput(os.Stderr)
		log.Fatalf("%+v", err)
	}
}

var self = os.Getpid()

func Main() error {
	flagTimeout := flag.Duration("t", 10*time.Second, "timeout for stop")
	flagProg := flag.String("prog", "^(firefox(-esr)?|LibreWolf|vivaldi(-stable)?)$", "name of the program, as regexp")
	flagStopDepth := flag.Int("stop-depth", 1, "STOP depth of child tree")
	flagAC := flag.String("ac", "/sys/class/power_supply/AC/online", "check AC (non-battery) here")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if !*flagVerbose {
		log.SetOutput(io.Discard)
	}

	rProg := regexp.MustCompile(*flagProg)

	iic, err := newIdleInhibitChecker()
	if err != nil {
		return err
	}
	defer iic.Close()
	if ok, err := iic.isInhibited(); err != nil {
		log.Printf("error isInhibited: %+v", err)
	} else {
		log.Printf("idle inhibited: %t", ok)
	}

	ctx, cancel := globalctx.Wrap(context.Background())
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
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
			kill(ff, false, 999)
		}
	}()
	grp.Go(func() error {
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
			if rProg.MatchString(change.Container.AppID) {
				ff = change.Container.PID
				kill(ff, false, 999)
				stopTimer()
				continue
			}
			kill(change.Container.PID, false, 0)

			if *flagAC != "" {
				b, err := os.ReadFile(*flagAC)
				if err != nil {
					return err
				}
				b = bytes.TrimSpace(b)
				if bytes.Equal(bytes.TrimSpace(b), []byte("1")) {
					log.Println("on AC, skip STOP")
					continue
				}
			}
			if timer == nil {
				timer = time.AfterFunc(timeout, func() {
					if inhibited, err := iic.isInhibited(); err != nil {
						log.Printf("error isInhibited: %+v", err)
					} else if inhibited {
						log.Printf("idle is inhibited, skip stop")
					} else {
						kill(ff, true, *flagStopDepth)
					}
				})
				continue
			}
			stopTimer()
			timer.Reset(timeout)
		}
		return nil
	})

	grp.Go(func() error {
		ticker := time.NewTicker(time.Minute)
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if os.Getppid() == 1 {
					return errors.New("parent is 1")
				}
			}
		}
	})
	return grp.Wait()
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
	if pid == 0 || pid == self {
		return nil
	}
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
	dis, _ := os.ReadDir("/proc")
	if c == nil {
		c = make(map[int][]int, len(dis))
		for _, di := range dis {
			pid, err := strconv.Atoi(di.Name())
			if err != nil || pid == 0 {
				continue
			}
			ppid, err := getPPid(pid)
			if ppid == 1 || ppid == 0 {
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
		if pid == 0 || pid == self {
			continue
		}
		if err := syscall.Kill(pid, sig); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func getPPid(pid int) (int, error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
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

type idleInhibitChecker struct {
	dbusConn *dbus.Conn
}

func newIdleInhibitChecker() (*idleInhibitChecker, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	return &idleInhibitChecker{dbusConn: conn}, nil
}

func (i *idleInhibitChecker) Close() error {
	conn := i.dbusConn
	i.dbusConn = nil
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func (i *idleInhibitChecker) isInhibited() (bool, error) {
	const listNames = "org.freedesktop.ScreenSaver"
	var activeNames []dbus.Sender
	if err := i.dbusConn.BusObject().
		Call(listNames, dbus.Flags(dbus.NameFlagDoNotQueue)).
		Store(
			&activeNames,
		); err != nil {
		log.Printf("calling %q: %+v", listNames, err)
	}
	log.Println("inhibitors:", activeNames)
	return len(activeNames) != 0, nil
}
