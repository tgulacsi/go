// Copyright (c) 2019, 2020 Tamás Gulácsi.
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

package httpunix

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// ListenAndServe is the same as http.ListenAndServe, except it can listen on unix domain sockets.
func ListenAndServe(ctx context.Context, addr string, hndl http.Handler) error {
	return ListenAndServeSrv(ctx, addr, http.Server{
		Handler: hndl,

		// ReadTimeout is the maximum duration for reading the entire
		// request, including the body.
		//
		// Because ReadTimeout does not let Handlers make per-request
		// decisions on each request body's acceptable deadline or
		// upload rate, most users will prefer to use
		// ReadHeaderTimeout. It is valid to use them both.
		ReadTimeout: time.Minute,

		// ReadHeaderTimeout is the amount of time allowed to read
		// request headers. The connection's read deadline is reset
		// after reading the headers and the Handler can decide what
		// is considered too slow for the body. If ReadHeaderTimeout
		// is zero, the value of ReadTimeout is used. If both are
		// zero, there is no timeout.
		ReadHeaderTimeout: 15 * time.Second,

		// WriteTimeout is the maximum duration before timing out
		// writes of the response. It is reset whenever a new
		// request's header is read. Like ReadTimeout, it does not
		// let Handlers make decisions on a per-request basis.
		WriteTimeout: time.Hour,

		// IdleTimeout is the maximum amount of time to wait for the
		// next request when keep-alives are enabled. If IdleTimeout
		// is zero, the value of ReadTimeout is used. If both are
		// zero, there is no timeout.
		IdleTimeout: 5 * time.Minute,
	})
}

// ListenAndServeSrv is the same as http.ListenAndServe, except it can listen on unix domain sockets.
func ListenAndServeSrv(ctx context.Context, addr string, srv http.Server) error {
	addr = strings.TrimPrefix(addr, "http+")
	var ln net.Listener
	if !strings.HasPrefix(addr, "unix:") {
		srv.Addr = addr
	} else {
		srv.Addr = addr
		addrU := addr
		addr = strings.TrimPrefix(addr[4:], "://")
		addr = strings.TrimPrefix(addr, ":")
		os.Remove(addr)
		var err error
		if ln, err = net.Listen("unix", addr); err != nil {
			return fmt.Errorf("%s: %w", addrU, err)
		}
		defer ln.Close()
	}
	go func() {
		<-ctx.Done()
		if ln != nil {
			ln.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		srv.Close()
	}()
	if ln != nil {
		return srv.Serve(ln)
	}
	return srv.ListenAndServe()
}
