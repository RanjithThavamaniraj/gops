// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package agent provides hooks programs can register to retrieve
// diagnostics data by using gops.
package agent

import (
	"fmt"
	"net"
	"os"
	gosignal "os/signal"
	"runtime"
	"runtime/pprof"

	"github.com/google/gops/signal"
)

// Start stars the gops agent on a host process. Once agent started,
// users can use the advanced gops features.
//
// Note: The agent exposes an endpoint via a UNIX socket that can be used by
// any program on the system. Review your security requirements before
// starting the agent.
func Start() error {
	// TODO(jbd): Expose these endpoints on HTTP. Then, we can enable
	// the agent on Windows systems.
	sock := fmt.Sprintf("/tmp/gops%d.sock", os.Getpid())
	l, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	gosignal.Notify(c, os.Interrupt)
	go func() {
		// cleanup the socket on shutdown.
		<-c
		os.Remove(sock)
		os.Exit(1)
	}()

	go func() {
		buf := make([]byte, 1)
		for {
			fd, err := l.Accept()
			if err != nil {
				fmt.Fprintf(os.Stderr, "gops: %v", err)
				continue
			}
			if _, err := fd.Read(buf); err != nil {
				fmt.Fprintf(os.Stderr, "gops: %v", err)
				continue
			}
			if err := handle(fd, buf); err != nil {
				fmt.Fprintf(os.Stderr, "gops: %v", err)
				continue
			}
			fd.Close()
		}
	}()
	return err
}

func handle(conn net.Conn, msg []byte) error {
	switch msg[0] {
	case signal.StackTrace:
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		_, err := conn.Write(buf[:n])
		return err
	case signal.GC:
		runtime.GC()
		_, err := conn.Write([]byte("ok"))
		return err
	case signal.MemStats:
		var s runtime.MemStats
		runtime.ReadMemStats(&s)
		fmt.Fprintf(conn, "alloc: %v bytes\n", s.Alloc)
		fmt.Fprintf(conn, "total-alloc: %v bytes\n", s.TotalAlloc)
		fmt.Fprintf(conn, "sys: %v bytes\n", s.Sys)
		fmt.Fprintf(conn, "lookups: %v\n", s.Lookups)
		fmt.Fprintf(conn, "mallocs: %v\n", s.Mallocs)
		fmt.Fprintf(conn, "frees: %v\n", s.Frees)
		fmt.Fprintf(conn, "heap-alloc: %v bytes\n", s.HeapAlloc)
		fmt.Fprintf(conn, "heap-sys: %v bytes\n", s.HeapSys)
		fmt.Fprintf(conn, "heap-idle: %v bytes\n", s.HeapIdle)
		fmt.Fprintf(conn, "heap-in-use: %v bytes\n", s.HeapInuse)
		fmt.Fprintf(conn, "heap-released: %v bytes\n", s.HeapReleased)
		fmt.Fprintf(conn, "heap-objects: %v\n", s.HeapObjects)
		fmt.Fprintf(conn, "stack-in-use: %v bytes\n", s.StackInuse)
		fmt.Fprintf(conn, "stack-sys: %v bytes\n", s.StackSys)
		fmt.Fprintf(conn, "next-gc: when heap-alloc >= %v bytes\n", s.NextGC)
		fmt.Fprintf(conn, "last-gc: %v ns ago\n", s.LastGC)
		fmt.Fprintf(conn, "gc-pause: %v ns\n", s.PauseTotalNs)
		fmt.Fprintf(conn, "num-gc: %v\n", s.NumGC)
		fmt.Fprintf(conn, "enable-gc: %v\n", s.EnableGC)
		fmt.Fprintf(conn, "debug-gc: %v\n", s.DebugGC)
	case signal.Version:
		fmt.Fprintf(conn, "%v\n", runtime.Version())
	case signal.HeapProfile:
		pprof.Lookup("heap").WriteTo(conn, 1)
	case signal.CPUProfile:
		panic("not implemented")
	}
	return nil
}
