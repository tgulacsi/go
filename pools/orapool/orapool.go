// Copyright 2015 Tamás Gulácsi. All rights reserved.
// Use of this source code is governed by The MIT License
// found in the accompanying LICENSE file.

// Package orapool implements a simple Srv/Ses pool for
// gopkg.in/rana/ora.v3, till that includes this.
package orapool

import (
	"strings"
	"sync"
	"time"

	"github.com/tgulacsi/go/orahlp"
	"github.com/tgulacsi/go/pools"
	"gopkg.in/rana/ora.v3"
)

const (
	DefaultPoolSize      = 4
	DefaultEvictDuration = time.Minute
)

// NewSrvPool returns a connection pool, which evicts the idle connections in every minute.
// The pool holds at most size idle Srv/Ses.
// If size is zero, DefaultPoolSize will be used.
// If env is nil, a new Env is created with ora.OpenEnv(nil).
func NewSrvPool(env *ora.Env, dsn string, size int) *SrvPool {
	if env == nil {
		var err error
		if env, err = ora.OpenEnv(nil); err != nil {
			panic(err)
		}
	}
	srvCfg := ora.NewSrvCfg()
	sesCfg := ora.NewSesCfg()
	if strings.IndexByte(dsn, '@') == -1 {
		srvCfg.Dblink = dsn
	} else {
		sesCfg.Username, sesCfg.Password, srvCfg.Dblink = orahlp.SplitDSN(dsn)
	}
	p := &SrvPool{
		env:         env,
		srvCfg:      srvCfg,
		sesCfg:      sesCfg,
		evict:       time.Minute,
		sesPoolSize: size,
		srv:         pools.NewIdlePool(size),
	}
	p.SetEvictDuration(DefaultEvictDuration)
	return p
}

type SrvPool struct {
	env         *ora.Env
	srvCfg      *ora.SrvCfg
	sesCfg      *ora.SesCfg
	sesPoolSize int

	mu       sync.Mutex // protects following fields
	evict    time.Duration
	tickerCh chan *time.Ticker
	srv      *pools.IdlePool
}

// Get returns a new SesPool from the pool, or creates a new if no idle is in the pool.
func (p *SrvPool) Get() (*SesPool, error) {
	x := p.srv.Get()
	if x == nil {
		srv, err := p.env.OpenSrv(p.srvCfg)
		if err != nil {
			return nil, err
		}
		return &SesPool{
			sesCfg: p.sesCfg,
			evict:  p.evict,
			Srv:    srv,
			ses:    pools.NewIdlePool(p.sesPoolSize),
		}, nil
	}
	return x.(*SesPool), nil
}

// Put the unneeded srv back into the pool.
func (p *SrvPool) Put(sesPool *SesPool) {
	if sesPool != nil && sesPool.IsOpen() {
		p.srv.Put(sesPool)
	}
}

// Set the eviction duration to the given.
// Also starts eviction if not yet started.
func (p *SrvPool) SetEvictDuration(dur time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tickerCh == nil { // first initialize
		p.tickerCh = initEvictor(func() {
			p.mu.Lock()
			dur := p.evict
			p.mu.Unlock()
			p.srv.Evict(dur)
		})
	}
	p.evict = dur
	p.tickerCh <- time.NewTicker(dur)
}

type SesPool struct {
	*ora.Srv
	sesCfg *ora.SesCfg

	mu       sync.Mutex
	evict    time.Duration
	tickerCh chan *time.Ticker
	ses      *pools.IdlePool
}

func (p *SesPool) Close() error {
	var firstErr error
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ses != nil {
		for {
			if x := p.ses.Get(); x != nil {
				if err := x.Close(); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			break
		}
	}
	if p.Srv != nil {
		if err := p.Srv.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Get a session from the pool.
func (p *SesPool) Get() (*ora.Ses, error) {
	var ses *ora.Ses
	for {
		x := p.ses.Get()
		if x == nil { // the pool is empty
			break
		}
		ses = x.(*ora.Ses)
		if err := ses.Ping(); err == nil {
			return ses, nil
		}
	}
	return p.Srv.OpenSes(p.sesCfg)
}

// Put the session back to the session pool.
// Also puts back the srv into the pool if has no used sessions.
func (p *SesPool) Put(ses *ora.Ses) {
	if ses == nil || !ses.IsOpen() {
		return
	}
	p.ses.Put(ses)
}

// Set the eviction duration to the given.
// Also starts eviction if not yet started.
func (p *SesPool) SetEvictDuration(dur time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tickerCh == nil { // first initialize
		p.tickerCh = initEvictor(func() {
			p.mu.Lock()
			p.mu.Lock()
			dur := p.evict
			p.mu.Unlock()
			p.ses.Evict(dur)
		})
	}
	p.evict = dur
	p.tickerCh <- time.NewTicker(dur)
}
func initEvictor(fn func()) chan *time.Ticker {
	tickerCh := make(chan *time.Ticker)
	go func(tickerCh <-chan *time.Ticker) {
		ticker := <-tickerCh
		for {
			select {
			case <-ticker.C:
				fn()
			case nxt := <-tickerCh:
				ticker.Stop()
				ticker = nxt
			}
		}
	}(tickerCh)
	return tickerCh
}
