// Copyright 2020, 2025 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	// "github.com/godbus/dbus/v5"
	"github.com/joshuarubin/go-sway"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
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
	systemctl := func(ctx context.Context, todo string) error {
		args := []string{"--user", todo, "tamefox.service"}
		if todo == "daemon-reload" {
			args = args[:len(args)-1]
		}
		cmd := exec.CommandContext(ctx, "systemctl", args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	startCmd := ff.Command{Name: "start",
		Exec: func(ctx context.Context, args []string) error {
			return systemctl(ctx, "restart")
		},
	}
	stopCmd := ff.Command{Name: "stop",
		Exec: func(ctx context.Context, args []string) error {
			return systemctl(ctx, "stop")
		},
	}
	installCmd := ff.Command{Name: "install",
		Exec: func(ctx context.Context, args []string) error {
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			dn := os.Getenv("XDG_CONFIG_HOME")
			if dn == "" {
				dn = os.ExpandEnv("$HOME/.config")
			}
			fn := filepath.Join(dn, "systemd", "user", "tamefox.service")
			if _, err := os.Stat(fn); err != nil {
				if err = os.WriteFile(fn, []byte(`[Unit]
Description=TameFox - stop power hungry browser out of focus

[Service]
ExecStart=`+executable+` --verbose
Restart=always
RestartSec=1s

[Install]
WantedBy=graphical-session.target`),
					0644,
				); err != nil {
					return err
				}
			}
			return systemctl(ctx, "daemon-reload")
		},
	}
	FS := ff.NewFlagSet("app")
	flagTimeout := FS.Duration('t', "timeput", 10*time.Second, "timeout for stop")
	flagProg := FS.String('p', "prog", "^(firefox(-esr)?|[lL]ibre[Ww]olf|vivaldi(-stable)?|[Jj]oplin)$", "name of the program, as regexp")
	flagStopDepth := FS.Int(0, "stop-depth", 1, "STOP depth of child tree")
	flagAC := FS.String(0, "ac", "/sys/class/power_supply/AC/online", "check AC (non-battery) here")
	flagVerbose := FS.Bool('v', "verbose", "verbose logging")

	app := ff.Command{Name: "tamefox",
		Subcommands: []*ff.Command{&startCmd, &stopCmd, &installCmd},
		Exec: func(ctx context.Context, args []string) error {
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

			grp, ctx := errgroup.WithContext(ctx)
			client, err := sway.New(ctx)
			if err != nil {
				return err
			}

			timeout := *flagTimeout
			var mu sync.RWMutex
			ff := make(map[int]*Timer)
			defer func() {
				mu.RLock()
				defer mu.RUnlock()
				for pid, timer := range ff {
					kill(pid, false, 999)
					timer.Stop()
				}
			}()

			newTimer := func(pid int) *Timer {
				if onAC() {
					return nil
				}
				if t := ff[pid]; t != nil {
					return t
				}
				log.Printf("new timer for %d", pid)
				return newAfterFunc(timeout, func() {
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
						switch evt.Change {
						case "new":
							if isFF {
								ff[pid] = newTimer(pid)
							}
						case "close":
							if isFF {
								kill(pid, false, 999)
								ff[pid].Stop()
								delete(ff, pid)
							}
						case "focus":
							if isFF {
								kill(pid, false, 999)
								timer := ff[pid]
								if timer == nil { // new does not have name, so this may be the first encounter
									timer = newTimer(pid)
									ff[pid] = timer
								} else {
									log.Printf("stopTimer %d", pid)
								}
								timer.Stop()
							}
							if lastFF != 0 && lastFF != pid {
								if t := ff[lastFF]; t != nil {
									if t.IsActive() {
										log.Printf("timer is active for %d", lastFF)
									} else {
										log.Printf("%d resetTimer %d", pid, lastFF)
										t.Reset(timeout)
									}
								}
							} else {
								log.Printf("%d lastFF=%d", pid, lastFF)
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
		},
	}

	if err := app.Parse(os.Args[1:]); err != nil {
		ffhelp.Command(&app).WriteTo(os.Stderr)
		if errors.Is(err, ff.ErrHelp) {
			return nil
		}
		return err
	}

	if !*flagVerbose {
		log.SetOutput(io.Discard)
	}
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

	return app.Run(ctx)
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

type Timer struct {
	timer  *time.Timer
	active atomic.Bool
}

func newAfterFunc(timeout time.Duration, f func()) *Timer {
	var t Timer
	t.active.Store(true)
	t.timer = time.AfterFunc(timeout, func() { t.active.Store(false); f() })
	return &t
}
func (t *Timer) IsActive() bool { return t.active.Load() }
func (t *Timer) Reset(timeout time.Duration) {
	t.active.Store(true)
	t.timer.Reset(timeout)
}
func (t *Timer) Stop() {
	if t == nil {
		return
	}
	t.active.Store(false)
	if t.timer != nil && !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
}
