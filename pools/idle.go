// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// Package pools contains some pools.
package pools

import (
	"io"
	"math/rand"
	"sync"
	"time"
)

// IdlePool is a pool of io.Closers.
// Each element will be Closed on eviction.
//
// The backing store is a simple []io.Closer, which is treated as random store,
// to achive uniform reuse.
type IdlePool struct {
	elems []io.Closer
	times []time.Time

	sync.Mutex
}

// Evict evicts idle items idle for more than the given duration.
func (p *IdlePool) Evict(dur time.Duration) {
	p.Lock()
	defer p.Unlock()
	deadline := time.Now().Add(-dur)
	for i, t := range p.times {
		e := p.elems[i]
		if e == nil || t.After(deadline) {
			continue
		}
		e.Close()
		p.elems[i] = nil
	}
}

// NewIdlePool returns an IdlePool.
func NewIdlePool(size int) *IdlePool {
	return &IdlePool{
		elems: make([]io.Closer, size),
		times: make([]time.Time, size),
	}
}

// Get returns a closer or nil, if no pool found.
func (p *IdlePool) Get() io.Closer {
	p.Lock()
	defer p.Unlock()
	for i, c := range p.elems {
		if c == nil {
			continue
		}
		p.elems[i] = nil
		return c
	}
	return nil
}

// Put a new element into the store. The slot is chosen randomly.
// If no empty slot is found, one (random) is Close()-d and this new
// element is put there.
// This way elements reused uniformly.
func (p *IdlePool) Put(c io.Closer) {
	p.Lock()
	defer p.Unlock()
	now := time.Now()
	n := len(p.elems)
	i0 := rand.Intn(n)
	for i := 0; i < n; i++ {
		j := (i0 + i) % n
		if p.elems[j] == nil {
			p.elems[j] = c
			p.times[j] = now
			return
		}
	}
	p.elems[i0].Close()
	p.elems[i0] = c
	p.times[i0] = now
}

// Close all elements.
func (p *IdlePool) Close() error {
	p.Lock()
	defer p.Unlock()
	var err error
	for i, c := range p.elems {
		p.elems[i] = nil
		if c == nil {
			continue
		}
		if closeErr := c.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
