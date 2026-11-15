// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0

package sieve

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSIEVEOrder(t *testing.T) {
	ctx := t.Context()
	loads := map[string]int{}
	c := NewContext(3, func(_ context.Context, k string) (string, error) { loads[k]++; return k, nil }, nil)
	take := func(k string) {
		t.Helper()
		lease, err := c.Acquire(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
	}
	take("A")
	take("B")
	take("C")
	take("A") // A visited, B and C unvisited
	take("D") // Hand skips A and evicts B
	take("A")
	if loads["A"] != 1 {
		t.Fatalf("A unexpectedly evicted: %v", loads)
	}
	take("B")
	if loads["B"] != 2 {
		t.Fatalf("B not evicted: %v", loads)
	}
	if c.Len() != 3 {
		t.Fatalf("size = %d", c.Len())
	}
}

func TestPinWaitAndTimeout(t *testing.T) {
	c := NewContext(1, func(_ context.Context, k int) (int, error) { return k, nil }, nil)
	held, err := c.Acquire(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()
	_, err = c.Acquire(ctx, 2)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	lease, err := c.Acquire(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Value != 2 {
		t.Fatalf("wrong value %d", lease.Value)
	}
	_ = lease.Release()
}

func TestShutdownAndRelease(t *testing.T) {
	var mu sync.Mutex
	closed := []int{}
	c := NewContext(2, func(_ context.Context, k int) (int, error) { return k, nil }, func(v int) error {
		mu.Lock()
		defer mu.Unlock()
		closed = append(closed, v)
		return nil
	})
	held, _ := c.Acquire(t.Context(), 1)
	idle, _ := c.Acquire(t.Context(), 2)
	_ = idle.Release()
	_ = c.Shutdown()
	if _, err := c.Acquire(t.Context(), 3); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected closed, got %v", err)
	}
	if len(closed) != 1 || closed[0] != 2 {
		t.Fatalf("idle not closed: %v", closed)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(closed) != 2 {
		t.Fatalf("values not all closed: %v", closed)
	}
}

func TestCloseWaits(t *testing.T) {
	c := NewContext(1, func(_ context.Context, k string) (string, error) { return k, nil }, nil)
	lease, _ := c.Acquire(t.Context(), "a")
	done := make(chan error, 1)
	go func() { done <- c.Close(t.Context()) }()
	select {
	case <-done:
		t.Fatal("Close returned before release")
	case <-time.After(15 * time.Millisecond):
	}
	_ = lease.Release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not finish")
	}
}

func TestWaitingAcquireResumes(t *testing.T) {
	c := NewContext(1, func(_ context.Context, k string) (string, error) { return k, nil }, nil)
	first, err := c.Acquire(t.Context(), "first")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		l   *Lease[string, string]
		err error
	}
	done := make(chan result, 1)
	go func() {
		l, err := c.Acquire(t.Context(), "second")
		done <- result{l, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("got second lease while first pinned: %+v", r)
	case <-time.After(15 * time.Millisecond):
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.l.Value != "second" {
			t.Fatalf("got %q", r.l.Value)
		}
		_ = r.l.Release()
	case <-time.After(time.Second):
		t.Fatal("acquire did not resume")
	}
}

func TestConcurrentAcquireLoadsOnce(t *testing.T) {
	var loads int
	c := NewContext(2, func(_ context.Context, k int) (int, error) { loads++; return k * 2, nil }, nil)
	const n = 30
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := c.Acquire(t.Context(), 7)
			if err != nil {
				t.Error(err)
				return
			}
			if l.Value != 14 {
				t.Errorf("got %d", l.Value)
			}
			if err := l.Release(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if loads != 1 {
		t.Fatalf("loads = %d, want 1", loads)
	}
}

func TestCloseTimeoutStillDisposes(t *testing.T) {
	var disposed bool
	c := NewContext(1, func(_ context.Context, k int) (int, error) { return k, nil }, func(_ int) error { disposed = true; return nil })
	l, _ := c.Acquire(t.Context(), 1)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()
	if err := c.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close error = %v", err)
	}
	if disposed {
		t.Fatal("disposed a pinned value")
	}
	_ = l.Release()
	if !disposed {
		t.Fatal("pinned value not disposed on release")
	}
}
