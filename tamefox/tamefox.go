// Copyright 2020, 2023 Tamás Gulácsi. All rights reserved.

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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/rajveermalviya/go-wayland/wayland/staging/ext-idle-notify-v1"
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
	flagProg := flag.String("prog", "firefox", "name of the program")
	flagStopDepth := flag.Int("stop-depth", 1, "STOP depth of child tree")
	flagAC := flag.String("ac", "/sys/class/power_supply/AC/online", "check AC (non-battery) here")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagIdle := flag.Bool("idle", false, "use wayland idle notification")
	flag.Parse()

	if !*flagVerbose {
		log.SetOutput(io.Discard)
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

	var timer *time.Timer
	var timeout time.Duration
	stopTimer := func() {}
	var ff int
	defer func() {
		if ff != 0 {
			kill(ff, false, 999)
		}
	}()

	if *flagIdle {
		disp, err := client.Connect("")
		if err != nil {
			return err
		}
		defer disp.Context().Close()
		defer disp.Destroy()
		// notif := ext_idle_notify.NewIdleNotifier(disp.Context())
		// defer notif.Destroy()
		seat := client.NewSeat(disp.Context())
		// n, err := notif.GetIdleNotification(1000, seat)
		n := ext_idle_notify.NewIdleNotification(disp.Context())
		// if err != nil {
		// log.Printf("ERROR: %+v", err)
		// return err
		// }
		defer n.Destroy()
		log.Println("seat:", seat, "n:", n)
		n.SetIdledHandler(func(ext_idle_notify.IdleNotificationIdledEvent) {
			log.Println("Idle")
			kill(ff, true, *flagStopDepth)
		})
		n.SetResumedHandler(func(ext_idle_notify.IdleNotificationResumedEvent) {
			log.Println("Resume")
			kill(ff, false, 999)
		})
		n.Context().Register(n)
	} else {
		timeout = *flagTimeout
		stopTimer = func() {
			if timer != nil && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
	}

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
			if strings.EqualFold(change.Container.AppID, *flagProg) ||
				(*flagProg == "vivaldi" &&
					strings.EqualFold(change.Container.AppID, "vivaldi-stable")) ||
				(*flagProg == "firefox" &&
					strings.EqualFold(change.Container.AppID, "firefox-esr")) {
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
			if timer == nil && timeout != 0 {
				timer = time.AfterFunc(timeout, func() {
					kill(ff, true, *flagStopDepth)
				})
				continue
			}
			stopTimer()
			if timer != nil {
				timer.Reset(timeout)
			}
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
