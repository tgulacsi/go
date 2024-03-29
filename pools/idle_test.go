// Copyright 2015, 2022 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by an Apache 2.0
// license that can be found in the LICENSE file.

package pools_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/tgulacsi/go/pools"
)

func TestIdlePool(t *testing.T) {
	p := pools.NewIdlePool(2)
	defer p.Close()
	r := bytes.NewReader(nil)
	c := io.NopCloser(r)
	p.Put(c)
	d := p.Get()
	if d != c {
		t.Errorf("want %v, got %v", c, d)
	}

	p.Put(c)
	p.Put(d)
	p.Get()
	e := p.Get()
	if e == nil || e != c && e != d {
		t.Errorf("want %v or %v, got %v", c, d, e)
	}

	p.Put(c)
	p.Put(d)
	p.Put(d)
	e = p.Get()
	if e != d {
		t.Errorf("want %v, got %v", d, e)
	}
	p.Put(e)

	p.Evict(1 * time.Minute)
	e = p.Get()
	if e != d {
		t.Errorf("nil eviction, wanted %v, got %v", d, e)
	}

	p.Evict(0) // empty pool
	e = p.Get()
	if e != nil {
		t.Errorf("wanted nil, got %v", e)
	}
}
