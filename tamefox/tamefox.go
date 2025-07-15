// Copyright 2020, 2025 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	// "github.com/godbus/dbus/v5"
	"github.com/joshuarubin/go-sway"
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
	flagProg := flag.String("prog", "^(firefox(-esr)?|[lL]ibre[Ww]olf|vivaldi(-stable)?|[Jj]oplin)$", "name of the program, as regexp")
	flagStopDepth := flag.Int("stop-depth", 1, "STOP depth of child tree")
	flagAC := flag.String("ac", "/sys/class/power_supply/AC/online", "check AC (non-battery) here")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	if !*flagVerbose {
		log.SetOutput(io.Discard)
	}
	var (
		onACmu sync.Mutex
		onACb  bool
		onACt  time.Time
	)
	onAC := func() bool {
		if *flagAC == "" {
			return false
		}
		onACmu.Lock()
		defer onACmu.Unlock()
		now := time.Now()
		if onACt.IsZero() || onACt.Before(now.Add(-*flagTimeout)) {
			b, err := os.ReadFile(*flagAC)
			if err != nil {
				log.Printf("ERR Read %s: %w", *flagAC, err)
				return onACb
			}
			b = bytes.TrimSpace(b)
			onACb = bytes.Equal(bytes.TrimSpace(b), []byte("1"))
		}
		return onACb
	}

	rProg := regexp.MustCompile(*flagProg)

	// iic, err := newIdleInhibitChecker()
	// if err != nil {
	// 	return err
	// }
	// defer iic.Close()
	// if ok, err := iic.isInhibited(); err != nil {
	// 	log.Printf("error isInhibited: %+v", err)
	// } else {
	// 	log.Printf("idle inhibited: %t", ok)
	// }

	ctx, cancel := globalctx.Wrap(context.Background())
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
	client, err := sway.New(ctx)
	if err != nil {
		return err
	}

	timeout := *flagTimeout
	stopTimer := func(timer *time.Timer) {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	var mu sync.RWMutex
	ff := make(map[int]*time.Timer)
	defer func() {
		mu.RLock()
		defer mu.RUnlock()
		for pid, timer := range ff {
			kill(pid, false, 999)
			stopTimer(timer)
		}
	}()

	newTimer := func(pid int) *time.Timer {
		if onAC() {
			return nil
		}
		if t := ff[pid]; t != nil {
			return t
		}
		log.Printf("new timer for %d", pid)
		return time.AfterFunc(timeout, func() {
			// if inhibited, err := iic.isInhibited(); err != nil {
			// 	log.Printf("error isInhibited: %+v", err)
			// } else if inhibited {
			// 	log.Printf("idle is inhibited, skip stop")
			// } else {
			if node, err := client.GetTree(ctx); err != nil {
				log.Printf("ERR GetTree: %+v", err)
			} else {
				if node = node.TraverseNodes(func(n *sway.Node) bool {
					return n != nil && n.InhibitIdle != nil && *n.InhibitIdle
				}); node != nil {
					log.Printf("idle is inhibited by %+v", node)
					return
				}
			}
			kill(pid, true, *flagStopDepth)
		})
	}

	// Fill ff
	node, err := client.GetTree(ctx)
	if err != nil {
		return err
	}
	node.TraverseNodes(func(node *sway.Node) bool {
		name, pid := nodeIDs(node)
		if name != "" && pid != 0 && rProg.MatchString(name) {
			mu.Lock()
			ff[pid] = newTimer(pid)
			mu.Unlock()
		}
		return false
	})

	var lastFF int
	if err := sway.Subscribe(ctx,
		windowEventHandler{
			EventHandler: sway.NoOpEventHandler(),
			WindowEventHandler: func(ctx context.Context, evt sway.WindowEvent) {
				switch evt.Change {
				case "new", "close", "focus":
				default:
					return
				}
				log.Printf("Event: %+v", evt)
				name, pid := nodeIDs(&evt.Container)
				if name == "" || pid == 0 {
					return
				}
				isFF := rProg.MatchString(name)
				mu.Lock()
				defer mu.Unlock()
				if pid != 0 && ff[pid] != nil {
					kill(pid, false, 999)
				}
				switch evt.Change {
				case "new":
					if isFF {
						ff[pid] = newTimer(pid)
					}
				case "close":
					if isFF {
						if timer := ff[pid]; timer != nil {
							stopTimer(timer)
						}
						delete(ff, pid)
					}
				case "focus":
					if isFF {
						if timer := ff[pid]; timer != nil {
							log.Printf("stopTimer %d", pid)
							stopTimer(timer)
						}
					}
					if lastFF != 0 && lastFF != pid {
						if t := ff[lastFF]; t != nil {
							log.Printf("%d resetTimer %d", pid, lastFF)
							t.Reset(timeout)
						}
					}
				}
				if isFF {
					lastFF = pid
				}
				return
			},
		},
		sway.EventTypeWindow,
	); err != nil {
		return err
	}

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
	Name  string `json:"name"`
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

/*
type idleInhibitChecker struct {
	dbusConn *dbus.Conn
}

func newIdleInhibitChecker() (*idleInhibitChecker, error) {
	conn, err := dbus.ConnectSystemBus()
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
	// auto reply = g_dbus_connection_call_with_unix_fd_list_sync(
	//      bus.get(), "org.freedesktop.login1", "/org/freedesktop/login1",
	//      "org.freedesktop.login1.Manager", "Inhibit",
	//      g_variant_new("(ssss)", inhibitors.c_str(), "waybar", "Asked by user", "block"),
	//      G_VARIANT_TYPE("(h)"), G_DBUS_CALL_FLAGS_NONE, -1, nullptr, &fd_list, nullptr, &error);

	// ListInhibitors(out a(ssssuu) inhibitors);
	//ListInhibitors() lists all currently active inhibitors. It returns an array of structures consisting of what, who, why, mode, uid (user ID), and pid (process ID).
	var inhibitors []struct {
		What, Who, Why, Mode string
		UID, PID             uint
	}
	obj := i.dbusConn.Object("org.freedesktop.login1", "/org/freedesktop/login1")
	if v, err := obj.GetProperty("org.freedesktop.login1.Manager.BlockInhibited"); err != nil {
		log.Printf("GetProperty: %+v", err)
	} else {
		log.Println("BlockInhibited:", v.String())
	}
	if err := obj.Call(
		"org.freedesktop.login1.Manager.ListInhibitors",
		dbus.FlagNoAutoStart,
	).Store(
		&inhibitors,
	); err != nil {
		log.Printf("calling %+v: %+v", obj, err)
	}
	log.Println("inhibitors:", inhibitors)
	for _, in := range inhibitors {
		if in.What == "idle" {
			return true, nil
		}
	}
	return false, nil
}
*/

type windowEventHandler struct {
	sway.EventHandler
	WindowEventHandler func(context.Context, sway.WindowEvent)
}

func (we windowEventHandler) Window(ctx context.Context, evt sway.WindowEvent) {
	f := we.WindowEventHandler
	if f == nil {
		f = we.EventHandler.Window
	}
	f(ctx, evt)
}

func nodeIDs(node *sway.Node) (string, int) {
	if node == nil {
		return "", 0
	}
	var pid int
	if node.PID != nil {
		pid = int(*node.PID)
	}
	if node.AppID != nil && *node.AppID != "" {
		return *node.AppID, pid
	}
	return node.Name, pid
}
