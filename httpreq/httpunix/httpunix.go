// Copyright (c) 2013-2015 Tommi Virtanen.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package httpunix provides a HTTP transport (net/http.RoundTripper)
// that uses Unix domain sockets instead of HTTP.
//
// This is useful for non-browser connections within the same host, as
// it allows using the file system for credentials of both client
// and server, and guaranteeing unique names.
//
// The URLs look like this:
//
//     http+unix://LOCATION/PATH_ETC
//
// where LOCATION is translated to a file system path with
// Transport.RegisterLocation, and PATH_ETC follow normal http: scheme
// conventions.
package httpunix

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// Scheme is the URL scheme used for HTTP over UNIX domain sockets.
const Scheme = "http+unix"

// Transport is a http.RoundTripper that connects to Unix domain
// sockets.
type Transport struct {
	DialTimeout           time.Duration
	RequestTimeout        time.Duration
	ResponseHeaderTimeout time.Duration

	mu sync.RWMutex
	// map a URL "hostname" to a UNIX domain socket path
	loc map[string]string
}

// RegisterLocation registers an URL location and maps it to the given
// file system path.
//
// Calling RegisterLocation twice for the same location is a
// programmer error, and causes a panic.
func (t *Transport) RegisterLocation(loc string, path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.loc == nil {
		t.loc = make(map[string]string)
	}
	if _, exists := t.loc[loc]; exists {
		panic("location " + loc + " already registered")
	}
	t.loc[loc] = path
}

var _ http.RoundTripper = (*Transport)(nil)

// RoundTrip executes a single HTTP transaction. See
// net/http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil {
		return nil, errors.New("http+unix: nil Request.URL")
	}
	if req.URL.Scheme != Scheme {
		return nil, errors.New("unsupported protocol scheme: " + req.URL.Scheme)
	}
	if req.URL.Host == "" {
		return nil, errors.New("http+unix: no Host in request URL")
	}
	t.mu.RLock()
	path, ok := t.loc[req.URL.Host]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.New("unknown location: " + req.Host)
	}

	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	deadline, _ := req.Context().Deadline()
	dur := t.DialTimeout
	if !deadline.IsZero() {
		if d2 := time.Until(deadline); d2 < dur {
			dur = d2
		}
	}
	c, err := net.DialTimeout("unix", path, dur)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(c)
	dl := deadline
	if t.RequestTimeout > 0 {
		if d2 := time.Now().Add(t.RequestTimeout); dl.IsZero() || d2.Before(dl) {
			dl = d2
		}
	}
	if !dl.IsZero() {
		c.SetWriteDeadline(dl)
	}
	if err := req.Write(c); err != nil {
		return nil, err
	}
	dl = deadline
	if t.ResponseHeaderTimeout > 0 {
		if d2 := time.Now().Add(t.ResponseHeaderTimeout); dl.IsZero() || d2.Before(dl) {
			dl = d2
		}
	}
	if !dl.IsZero() {
		c.SetReadDeadline(dl)
	}
	return http.ReadResponse(r, req)
}
