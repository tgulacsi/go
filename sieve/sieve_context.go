// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0

// Package sieve provides a bounded, generic SIEVE cache for leased resources.
package sieve

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrClosed means the cache has already been closed.
	ErrClosed = errors.New("sieve: closed")
	// ErrFull means every cached entry is currently pinned by a lease.
	ErrFull = errors.New("sieve: all entries are in use")
)

type (
	// Loader constructs a value for a missing key. If it returns an error, it
	// must release any resources allocated before the error.
	Loader[K comparable, V any] func(context.Context, K) (V, error)

	// Disposer releases a value when evicted or when the cache is shut down.
	// It may be nil if values require no cleanup.
	Disposer[V any] func(V) error

	// LDCache is a bounded SIEVE cache. New entries go to the front; hits only
	// mark an entry as visited. Acquired entries cannot be evicted.
	//
	// It differ from Cache in that it has a Loader and a Disposer,
	// the entries are created by the provided Loader,
	// and the eviction is notified with the Disposer.
	LDCache[K comparable, V any] struct {
		mu       sync.Mutex
		entries  map[K]*ldEntry[K, V]
		queue    list.List
		hand     *list.Element
		capacity int
		load     Loader[K, V]
		dispose  Disposer[V]
		changed  chan struct{}
		closing  bool
		closeErr error
	}

	ldEntry[K comparable, V any] struct {
		key     K
		value   V
		refs    int
		visited bool
		node    *list.Element
	}

	// Lease pins Value until Release. Do not use Value after Release, or copy a
	// Lease after first use. Release is safe to call more than once.
	Lease[K comparable, V any] struct {
		Value V
		cache leaseReleaser[K, V]
		entry *ldEntry[K, V]
		once  sync.Once
		err   error
	}

	leaseReleaser[K comparable, V any] interface {
		releaseLease(e *ldEntry[K, V]) error
	}
)

// Release drops a pin. After Close, the last Release also disposes the value.
// It returns any disposal error associated with this final release.
func (l *Lease[K, V]) Release() error {
	l.once.Do(func() {
		l.err = l.cache.releaseLease(l.entry)
	})
	return l.err
}

// NewLDCache creates a cache with room for capacity values.
// The loader and disposer run while the cache lock is held. They must not
// call back into this cache; prefer fast, context-aware loaders.
func NewLDCache[K comparable, V any](capacity int, load Loader[K, V], dispose Disposer[V]) *LDCache[K, V] {
	if capacity <= 0 {
		panic("sieve: capacity must be positive")
	}
	if load == nil {
		panic("sieve: loader must not be nil")
	}
	return &LDCache[K, V]{
		entries:  make(map[K]*ldEntry[K, V]),
		capacity: capacity,
		load:     load,
		dispose:  dispose,
		changed:  make(chan struct{}),
	}
}

func (c *LDCache[K, V]) signalLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}

// victimLocked finds an unpinned, unvisited entry, giving a second chance
// to visited entries. Two sweeps suffice even if all candidates were visited.
func (c *LDCache[K, V]) victimLocked() *ldEntry[K, V] {
	n := c.queue.Len()
	for scanned := 0; scanned < 2*n; scanned++ {
		if c.hand == nil {
			c.hand = c.queue.Back()
		}
		node := c.hand
		c.hand = node.Prev()
		e := node.Value.(*ldEntry[K, V])
		if e.refs != 0 {
			continue
		}
		if e.visited {
			e.visited = false
			continue
		}
		return e
	}
	return nil
}

func (c *LDCache[K, V]) disposeLocked(value V) error {
	if c.dispose != nil {
		return c.dispose(value)
	}
	return nil
}

// Acquire returns a pinned value. On a miss it loads the value once under
// the cache mutex. If all slots are pinned, Acquire waits for a release,
// shutdown, or context cancellation.
func (c *LDCache[K, V]) Acquire(ctx context.Context, key K) (*Lease[K, V], error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.closing {
			c.mu.Unlock()
			return nil, ErrClosed
		}
		if e := c.entries[key]; e != nil {
			e.refs++
			e.visited = true
			c.mu.Unlock()
			return &Lease[K, V]{Value: e.value, cache: c, entry: e}, nil
		}

		if len(c.entries) == c.capacity {
			victim := c.victimLocked()
			if victim == nil {
				changed := c.changed
				c.mu.Unlock()
				select {
				case <-changed:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			delete(c.entries, victim.key)
			c.queue.Remove(victim.node)
			if err := c.disposeLocked(victim.value); err != nil {
				c.mu.Unlock()
				return nil, fmt.Errorf("sieve: evict: %w", err)
			}
		}

		value, err := c.load(ctx, key)
		if err != nil {
			c.mu.Unlock()
			return nil, err
		}
		e := &ldEntry[K, V]{key: key, value: value, refs: 1}
		e.node = c.queue.PushFront(e)
		c.entries[key] = e
		c.mu.Unlock()
		return &Lease[K, V]{Value: value, cache: c, entry: e}, nil
	}
}

func (c *LDCache[K, V]) releaseLease(e *ldEntry[K, V]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	e.refs--
	if e.refs != 0 {
		return nil
	}
	var err error
	if c.closing {
		delete(c.entries, e.key)
		c.queue.Remove(e.node)
		err = c.disposeLocked(e.value)
		c.closeErr = errors.Join(c.closeErr, err)
	}
	c.signalLocked()
	return err
}

// Len returns the number of cached values, including pinned values.
func (c *LDCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Shutdown rejects new acquisitions and disposes idle entries immediately.
// Pinned entries are disposed on their final release.
func (c *LDCache[K, V]) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdownLocked()
	return c.closeErr
}

func (c *LDCache[K, V]) shutdownLocked() {
	if c.closing {
		return
	}
	c.closing = true
	c.hand = nil
	for key, e := range c.entries {
		if e.refs != 0 {
			continue
		}
		delete(c.entries, key)
		c.queue.Remove(e.node)
		c.closeErr = errors.Join(c.closeErr, c.disposeLocked(e.value))
	}
	c.signalLocked()
}

// Close begins shutdown and waits for all leases to be released.
// If ctx expires, outstanding values are still disposed on their release.
func (c *LDCache[K, V]) Close(ctx context.Context) error {
	c.mu.Lock()
	c.shutdownLocked()
	for len(c.entries) != 0 {
		if err := ctx.Err(); err != nil {
			result := errors.Join(c.closeErr, err)
			c.mu.Unlock()
			return result
		}
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
		}
		c.mu.Lock()
	}
	result := c.closeErr
	c.mu.Unlock()
	return result
}
