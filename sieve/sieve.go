// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0

// Package sieve implements a bounded, generic SIEVE resource cache.
//
// Values are acquired through leases: an acquired value remains valid until
// its lease is released, even when other callers are filling the cache.
package sieve

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrClosed = errors.New("sieve: closed")

// Loader constructs a value for a missing key. If it returns an error, it
// must release any resources allocated before the error.
type Loader[K comparable, V any] func(context.Context, K) (V, error)

// Disposer releases a value when evicted or when the cache is shut down.
// It may be nil if values require no cleanup.
type Disposer[V any] func(V) error

// Cache is a bounded SIEVE cache. New entries go to the front; hits only
// mark an entry as visited. Acquired entries cannot be evicted.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	entries  map[K]*entry[K, V]
	queue    list.List
	hand     *list.Element
	capacity int
	load     Loader[K, V]
	dispose  Disposer[V]
	changed  chan struct{}
	closing  bool
	closeErr error
}

type entry[K comparable, V any] struct {
	key     K
	value   V
	refs    int
	visited bool
	node    *list.Element
}

// Lease pins Value until Release is called. Do not use Value after Release.
// A Lease must not be copied after first use.
type Lease[K comparable, V any] struct {
	Value V
	cache *Cache[K, V]
	entry *entry[K, V]
	once  sync.Once
	err   error
}

// New creates a cache with room for capacity values.
// The loader and disposer run while the cache lock is held. They must not
// call back into this cache; prefer fast, context-aware loaders.
func New[K comparable, V any](capacity int, load Loader[K, V], dispose Disposer[V]) *Cache[K, V] {
	if capacity <= 0 {
		panic("sieve: capacity must be positive")
	}
	if load == nil {
		panic("sieve: loader must not be nil")
	}
	return &Cache[K, V]{
		entries:  make(map[K]*entry[K, V]),
		capacity: capacity,
		load:     load,
		dispose:  dispose,
		changed:  make(chan struct{}),
	}
}

func (c *Cache[K, V]) signalLocked() {
	close(c.changed)
	c.changed = make(chan struct{})
}

// victimLocked finds an unpinned, unvisited entry, giving a second chance
// to visited entries. Two sweeps suffice even if all candidates were visited.
func (c *Cache[K, V]) victimLocked() *entry[K, V] {
	n := c.queue.Len()
	for scanned := 0; scanned < 2*n; scanned++ {
		if c.hand == nil {
			c.hand = c.queue.Back()
		}
		node := c.hand
		c.hand = node.Prev()
		e := node.Value.(*entry[K, V])
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

func (c *Cache[K, V]) disposeLocked(value V) error {
	if c.dispose != nil {
		return c.dispose(value)
	}
	return nil
}

// Acquire returns a pinned value. On a miss it loads the value once under
// the cache mutex. If all slots are pinned, Acquire waits for a release,
// shutdown, or context cancellation.
func (c *Cache[K, V]) Acquire(ctx context.Context, key K) (*Lease[K, V], error) {
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
		e := &entry[K, V]{key: key, value: value, refs: 1}
		e.node = c.queue.PushFront(e)
		c.entries[key] = e
		c.mu.Unlock()
		return &Lease[K, V]{Value: value, cache: c, entry: e}, nil
	}
}

// Release unpins the value and is safe to call more than once.
// An active value is disposed on its last release after shutdown.
func (l *Lease[K, V]) Release() error {
	l.once.Do(func() {
		c := l.cache
		c.mu.Lock()
		defer c.mu.Unlock()
		e := l.entry
		e.refs--
		if e.refs != 0 {
			return
		}
		if c.closing {
			delete(c.entries, e.key)
			c.queue.Remove(e.node)
			l.err = c.disposeLocked(e.value)
			c.closeErr = errors.Join(c.closeErr, l.err)
		}
		c.signalLocked()
	})
	return l.err
}

// Len returns the number of cached values, including pinned values.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Shutdown rejects new acquisitions and disposes idle entries immediately.
// Pinned entries are disposed on their final release.
func (c *Cache[K, V]) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdownLocked()
	return c.closeErr
}

func (c *Cache[K, V]) shutdownLocked() {
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
func (c *Cache[K, V]) Close(ctx context.Context) error {
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
