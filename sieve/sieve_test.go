// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0

package sieve

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSIEVESecondChance(t *testing.T) {
	loads := map[string]int{}
	c := New(3, func(k string) (string, error) {
		loads[k]++
		return k, nil
	}, nil)
	getAndRelease := func(key string) {
		t.Helper()
		l, err := c.Acquire(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.Release(); err != nil {
			t.Fatal(err)
		}
	}

	getAndRelease("A")
	getAndRelease("B")
	getAndRelease("C")
	getAndRelease("A") // Give the oldest entry a second chance.
	getAndRelease("D") // Should evict B instead of A.
	getAndRelease("A")
	if loads["A"] != 1 {
		t.Fatalf("A was evicted despite visited bit: loads=%v", loads)
	}
	getAndRelease("B")
	if loads["B"] != 2 {
		t.Fatalf("B should have been evicted: loads=%v", loads)
	}
	if c.Len() != 3 {
		t.Fatalf("got %d entries, want 3", c.Len())
	}
}

func TestFullReturnsImmediatelyButHitStillWorks(t *testing.T) {
	var loads int
	c := New(1, func(k int) (int, error) {
		loads++
		return k * 10, nil
	}, nil)

	held, err := c.Acquire(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Acquire(2); !errors.Is(err, ErrFull) {
		t.Fatalf("expected ErrFull, got %v", err)
	}
	if loads != 1 || c.Len() != 1 {
		t.Fatalf("failed Acquire changed cache: loads=%d, len=%d", loads, c.Len())
	}

	duplicate, err := c.Acquire(1)
	if err != nil || duplicate.Value != 10 {
		t.Fatalf("acquiring existing pinned key: lease=%+v err=%v", duplicate, err)
	}
	_ = held.Release()
	if _, err := c.Acquire(2); !errors.Is(err, ErrFull) {
		t.Fatalf("expected ErrFull while duplicate lease is pinned, got %v", err)
	}
	_ = duplicate.Release()

	next, err := c.Acquire(2)
	if err != nil {
		t.Fatal(err)
	}
	if next.Value != 20 {
		t.Fatalf("value=%d, want 20", next.Value)
	}
	_ = next.Release()
}

func TestCloseDoesNotWaitAndDisposesOnLastRelease(t *testing.T) {
	var disposedMu sync.Mutex
	var disposed []int
	c := New(2, func(k int) (int, error) { return k, nil }, func(v int) error {
		disposedMu.Lock()
		disposed = append(disposed, v)
		disposedMu.Unlock()
		return nil
	})
	held, _ := c.Acquire(1)
	idle, _ := c.Acquire(2)
	_ = idle.Release()

	done := make(chan error, 1)
	go func() { done <- c.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close waited for a pinned lease")
	}
	if _, err := c.Acquire(3); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}

	disposedMu.Lock()
	if len(disposed) != 1 || disposed[0] != 2 {
		t.Fatalf("idle value not disposed: %v", disposed)
	}
	disposedMu.Unlock()

	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	disposedMu.Lock()
	if len(disposed) != 2 || disposed[1] != 1 {
		t.Fatalf("pinned value not disposed on last release: %v", disposed)
	}
	disposedMu.Unlock()
	if c.Len() != 0 {
		t.Fatalf("closed cache has %d entries", c.Len())
	}
}

func TestConcurrentSameKeyLoadsOnce(t *testing.T) {
	var loads atomic.Int64
	c := New(2, func(k int) (int, error) {
		loads.Add(1)
		return k * 2, nil
	}, nil)
	const n = 50
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := c.Acquire(7)
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
	if loads.Load() != 1 {
		t.Fatalf("loaded %d times, want 1", loads.Load())
	}
}

func TestErrorsAndCloseErrorAccumulation(t *testing.T) {
	loadErr := errors.New("load failed")
	c := New(1, func(k string) (string, error) {
		if k == "bad" {
			return "", loadErr
		}
		return k, nil
	}, nil)
	if _, err := c.Acquire("bad"); !errors.Is(err, loadErr) {
		t.Fatalf("got %v, want loader error", err)
	}
	if c.Len() != 0 {
		t.Fatal("failed load should not be cached")
	}

	disposeErr := errors.New("dispose failed")
	d := New(1, func(k int) (int, error) { return k, nil }, func(int) error { return disposeErr })
	l, _ := d.Acquire(1)
	if err := d.Close(); err != nil {
		t.Fatalf("pinned value should not be disposed yet: %v", err)
	}
	if err := l.Release(); !errors.Is(err, disposeErr) {
		t.Fatalf("got %v from Release, want dispose error", err)
	}
	if err := d.Close(); !errors.Is(err, disposeErr) {
		t.Fatalf("second Close should report late dispose error; got %v", err)
	}
}

func TestEvictionDisposalError(t *testing.T) {
	disposeErr := errors.New("can't dispose")
	c := New(1, func(k int) (int, error) { return k, nil }, func(int) error { return disposeErr })
	l, _ := c.Acquire(1)
	_ = l.Release()
	if _, err := c.Acquire(2); !errors.Is(err, disposeErr) {
		t.Fatalf("expected eviction error, got %v", err)
	}
	if c.Len() != 0 {
		t.Fatalf("expected failed evicted entry to be removed, len=%d", c.Len())
	}
}
