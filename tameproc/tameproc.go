// Copyright 2020, 2023 Tamás Gulácsi. All rights reserved.

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var self = os.Getpid()

func main() {
	if err := Main(); err != nil {
		slog.Error("MAIN", "error", err)
		os.Exit(1)
	}
}

func Main() error {
	flagTimeout := flag.Duration("t", 10*time.Second, "timeout for stop")
	flagStopDepth := flag.Int("stop-depth", 1, "STOP depth of child tree")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagSetName := flag.Bool("set-name", true, "set process name to tameproc-<prog>")
	flag.Parse()

	prog := flag.Arg(0)
	if *flagSetName {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		return syscall.Exec(executable,
			append(append(make([]string, 0, len(os.Args)),
				filepath.Base(os.Args[0])+"-"+prog,
				"-set-name=false"), os.Args[1:]...),
			os.Environ(),
		)
	}

	if *flagVerbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)

	sigCh := make(chan os.Signal, runtime.GOMAXPROCS(-1))
	// USR1=CONT USR2=STOP
	signal.Notify(sigCh, syscall.SIGCONT, syscall.SIGUSR1)

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

	var lastPID uint32 // cat /proc/sys/kernel/pid_max

	defer func() { kill(prog, int(atomic.LoadUint32(&lastPID)), false, 999) }()
	grp.Go(func() error {
	Loop:
		for {
			select {
			case <-ctx.Done():
				return nil
			case sig, ok := <-sigCh:
				if !ok {
					return nil
				}
				slog.Warn("signal", "sig", sig)

				switch sig {
				case syscall.SIGCONT: // CONT
					if pid, err := kill(prog, int(atomic.LoadUint32(&lastPID)), false, 999); err != nil {
						slog.Error("CONT", slog.String("prog", prog), slog.Uint64("pid", uint64(atomic.LoadUint32(&lastPID))), "error", err)
					} else {
						atomic.StoreUint32(&lastPID, uint32(pid))
					}
					stopTimer()
					continue Loop

				case syscall.SIGUSR1: // STOP
					if timer == nil {
						timer = time.AfterFunc(timeout, func() {
							if pid, err := kill(prog, int(atomic.LoadUint32(&lastPID)), true, *flagStopDepth); err != nil {
								slog.Error("STOP", slog.String("prog", prog), slog.Uint64("pid", uint64(atomic.LoadUint32(&lastPID))), "error", err)
							} else {
								atomic.StoreUint32(&lastPID, uint32(pid))
							}
						})
						continue Loop
					}
					stopTimer()
					timer.Reset(timeout)
				}
			}

		}
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

func kill(prog string, pid int, stop bool, depth int) (int, error) {
	if pid == 0 || pid == self {
		if newPid, err := findProg(prog); err != nil {
			return pid, err
		} else {
			pid = int(newPid)
		}
		if pid == 0 || pid == self {
			return 0, nil
		}
	}
	var firstErr error
	if stop {
		const sig = syscall.SIGSTOP
		slog.Info("STOP", "pid", pid)
		firstErr = syscall.Kill(pid, sig)
		if err := ckill(pid, sig, nil, depth); err != nil && firstErr == nil {
			firstErr = err
		}
	} else {
		slog.Info("CONT", "pid", pid)
		const sig = syscall.SIGCONT
		firstErr = ckill(pid, sig, nil, depth)
		if err := syscall.Kill(pid, sig); err != nil && firstErr != nil {
			firstErr = err
		}
	}
	return pid, firstErr
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

func findProg(prog string) (uint32, error) {
	dis, err := os.ReadDir("/proc")
	if err != nil && len(dis) == 0 {
		return 0, err
	}
	for _, di := range dis {
		pid, err := strconv.ParseUint(di.Name(), 10, 32)
		if err != nil {
			continue
		}
		name, err := os.Readlink(filepath.Join("/proc", di.Name(), "exe"))
		if err != nil {
			continue
		}
		if _, base := filepath.Split(name); base == prog {
			return uint32(pid), nil
		}
	}
	return 0, errNotFound
}

var errNotFound = errors.New("not found")
