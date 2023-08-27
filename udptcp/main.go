// Package main of udptcp is an UDP-TCP or TCP-UDP proxy.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	flagListen := flag.String("listen", "udp:0.0.0.0:1194", "listen on this address")
	flagForward := flag.String("forward", "tcp:1.2.3.4:1194", "forward to this address")
	flag.Parse()

	handleConnection := connForwarder(splitType(*flagForward))

	listenType, listenAddr := splitType(*flagListen)
	if listenType == "tcp" {
		log.Println("Listening on " + *flagListen)
		ln, err := net.Listen(listenType, listenAddr)
		if err != nil {
			log.Fatal(fmt.Errorf("%s: %w", *flagListen, err))
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			ctx := context.Background()
			go handleConnection(ctx, conn)
		}
	}
	lAddr, err := net.ResolveUDPAddr(listenType, listenAddr)
	if err != nil {
		log.Fatal(fmt.Errorf("%s: %w", *flagListen, err))
	}
	log.Println("Listening on "+listenType+":", lAddr)
	for {
		conn, err := net.ListenUDP(listenType, lAddr)
		if err != nil {
			log.Printf("%s:%v: %+v", listenType, lAddr, err)
		}
		log.Println(conn, err)
		if conn != nil {
			ctx := context.Background()
			if err := handleConnection(ctx, conn); err != nil {
				log.Println("handle:", err)
			}
			conn.Close()
			continue
		}
		time.Sleep(1 * time.Second)
	}
}

func connForwarder(typ, addr string) func(ctx context.Context, conn net.Conn) error {
	var conns sync.Map
	return func(ctx context.Context, down net.Conn) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := down.RemoteAddr()
		log.Println("received connection from", key)
		var up net.Conn
		if I, ok := conns.Load(key); ok {
			up = I.(net.Conn)
		}
		if up == nil {
			var err error
			if up, err = net.Dial(typ, addr); err != nil {
				return fmt.Errorf("%s:%s: %w", typ, addr, err)
			}
			conns.Store(key, up)
		}
		defer conns.Delete(key)
		grp, _ := errgroup.WithContext(ctx)
		grp.Go(func() error { _, err := io.Copy(up, down); return err })
		grp.Go(func() error { _, err := io.Copy(down, up); return err })
		return grp.Wait()
	}
}

func splitType(addr string) (typ, address string) {
	if i := strings.IndexByte(addr, ':'); i >= 0 {
		return addr[:i], addr[i+1:]
	}
	return "", addr
}
