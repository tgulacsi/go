// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

// pools package contains some pools.
package pools

import (
	"io"
	"math/rand"
)

// IdlePool is a pool of io.Closers.
// Each element will be Closed on eviction.
//
// The backing store is a simple []io.Closer, which is treated as random store,
// to achive uniform reuse.
type IdlePool struct {
	elems []io.Closer
}

// NewIdlePool returns an IdlePool.
func NewIdlePool(size int) IdlePool {
	return IdlePool{make([]io.Closer, size)}
}

// Get returns a closer or nil, if no pool found.
func (p IdlePool) Get() io.Closer {
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
func (p IdlePool) Put(c io.Closer) {
	n := len(p.elems)
	i0 := rand.Intn(n)
	for i := 0; i < n; i++ {
		j := (i0 + i) % n
		if p.elems[j] == nil {
			p.elems[j] = c
			return
		}
	}
	p.elems[i0].Close()
	p.elems[i0] = c
}

// Close all elements.
func (p IdlePool) Close() error {
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
