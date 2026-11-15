// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0

// Package sieve provides a bounded, generic SIEVE cache for leased resources.
package sieve

import (
	"container/list"
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

	// Loader creates the value for a missing key. On failure, Loader must clean up
	// any resources it allocated before returning the error.
	Loader[K comparable, V any] func(K) (V, error)

	// Disposer releases an evicted or closed value. It can be nil when values do
	// not need explicit cleanup.
	Disposer[V any] func(V) error

	// Cache is a bounded SIEVE cache. Entries are inserted at the front of the
	// queue, and hits only set the visited bit. A value with an active lease is
	// never evicted. The zero value of Cache is not usable; call New.
	//
	// Cache misses never wait for an occupied slot: if no entry can be evicted,
	// Acquire returns ErrFull immediately.
	Cache[K comparable, V any] struct {
		mu       sync.Mutex
		entries  map[K]*entry[K, V]
		queue    list.List
		hand     *list.Element
		capacity int
		load     Loader[K, V]
		dispose  Disposer[V]
		closed   bool
		closeErr error
	}

	entry[K comparable, V any] struct {
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
		entry *entry[K, V]
		once  sync.Once
		err   error
	}

	leaseReleaser[K comparable, V any] interface {
		releaseLease(e *entry[K, V]) error
	}
)

// New creates a cache holding at most capacity values.
//
// load and dispose are called while the cache mutex is held, preventing
// duplicate loads. They must not call back into this cache. Although Acquire
// never waits for a free slot, normal mutex contention and slow callbacks can
// still delay calls.
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
	}
}

// victimLocked finds the next unpinned entry whose visited bit is clear,
// giving visited entries a second chance by resetting that bit. Two full
// scans are sufficient unless all entries are pinned.
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
	if c.dispose == nil {
		return nil
	}
	return c.dispose(value)
}

// Acquire returns a lease on the given key. On a miss, an unpinned entry is
// evicted if necessary. If the cache is full and all entries are pinned, it
// returns ErrFull immediately instead of waiting for Release.
//
// The loader runs under the cache lock; it is called once per cache miss.
func (c *Cache[K, V]) Acquire(key K) (*Lease[K, V], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrClosed
	}
	if e := c.entries[key]; e != nil {
		e.refs++
		e.visited = true
		return &Lease[K, V]{Value: e.value, cache: c, entry: e}, nil
	}

	if len(c.entries) == c.capacity {
		victim := c.victimLocked()
		if victim == nil {
			return nil, ErrFull
		}
		delete(c.entries, victim.key)
		c.queue.Remove(victim.node)
		if err := c.disposeLocked(victim.value); err != nil {
			return nil, fmt.Errorf("sieve: evict: %w", err)
		}
	}

	value, err := c.load(key)
	if err != nil {
		return nil, err
	}
	e := &entry[K, V]{key: key, value: value, refs: 1}
	e.node = c.queue.PushFront(e)
	c.entries[key] = e
	return &Lease[K, V]{Value: value, cache: c, entry: e}, nil
}

// Release drops a pin. After Close, the last Release also disposes the value.
// It returns any disposal error associated with this final release.
func (l *Lease[K, V]) Release() error {
	l.once.Do(func() {
		l.err = l.cache.releaseLease(l.entry)
	})
	return l.err
}

func (c *Cache[K, V]) releaseLease(e *entry[K, V]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	e.refs--
	if e.refs != 0 || !c.closed {
		return nil
	}
	delete(c.entries, e.key)
	c.queue.Remove(e.node)
	err := c.disposeLocked(e.value)
	c.closeErr = errors.Join(c.closeErr, err)
	return err
}

// Len returns the number of entries including pinned ones.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Close closes the cache without waiting for active leases. Idle entries are
// disposed immediately; pinned entries are disposed on their final Release.
// Subsequent calls return accumulated disposal errors, including errors from
// releases that happened after the first Close.
func (c *Cache[K, V]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return c.closeErr
	}

	c.closed = true
	c.hand = nil
	for key, e := range c.entries {
		if e.refs != 0 {
			continue
		}
		delete(c.entries, key)
		c.queue.Remove(e.node)
		c.closeErr = errors.Join(c.closeErr, c.disposeLocked(e.value))
	}
	return c.closeErr
}
