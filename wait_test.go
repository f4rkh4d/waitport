package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// startListener opens a TCP listener on a random port and returns the
// "host:port" string plus a cleanup func.
func startListener(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// accept and immediately close anything that connects
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		wg.Wait()
	}
}

// freePort returns a host:port that is not listening.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func TestWaitOneImmediatelyReady(t *testing.T) {
	addr, stop := startListener(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	r := WaitOne(ctx, addr, 50*time.Millisecond)
	if !r.Ready {
		t.Fatalf("expected ready, got err=%v", r.Err)
	}
	if r.Target != addr {
		t.Fatalf("target mismatch: %s vs %s", r.Target, addr)
	}
}

func TestWaitOneTimeout(t *testing.T) {
	addr := freePort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	r := WaitOne(ctx, addr, 50*time.Millisecond)
	elapsed := time.Since(start)

	if r.Ready {
		t.Fatalf("expected not ready")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("waited too long: %s", elapsed)
	}
}

func TestWaitOneBecomesReady(t *testing.T) {
	// find a port, then start listener after a delay
	addr := freePort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		time.Sleep(250 * time.Millisecond)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		}()
		// keep it open until test ends
		time.AfterFunc(2*time.Second, func() { _ = ln.Close() })
	}()

	r := WaitOne(ctx, addr, 75*time.Millisecond)
	if !r.Ready {
		t.Fatalf("expected ready after delay, err=%v", r.Err)
	}
	if r.Duration < 200*time.Millisecond {
		t.Fatalf("ready too fast? %s", r.Duration)
	}
}

func TestWaitAllMultipleReady(t *testing.T) {
	a1, s1 := startListener(t)
	defer s1()
	a2, s2 := startListener(t)
	defer s2()
	a3, s3 := startListener(t)
	defer s3()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var buf bytes.Buffer
	ok, results := WaitAll(ctx, []string{a1, a2, a3}, 50*time.Millisecond, &buf, false)
	if !ok {
		t.Fatalf("expected all ready, results=%+v", results)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	out := buf.String()
	for _, a := range []string{a1, a2, a3} {
		if !strings.Contains(out, a) {
			t.Errorf("output missing %s: %s", a, out)
		}
	}
}

func TestWaitAllOneFails(t *testing.T) {
	a1, s1 := startListener(t)
	defer s1()
	bad := freePort(t)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	ok, results := WaitAll(ctx, []string{a1, bad}, 50*time.Millisecond, &buf, true)
	if ok {
		t.Fatalf("expected failure because %s is closed", bad)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// quiet=true: nothing should have been written
	if buf.Len() != 0 {
		t.Fatalf("quiet mode wrote output: %q", buf.String())
	}
}

func TestWaitAllConcurrency(t *testing.T) {
	// start N listeners; confirm WaitAll finishes fast (concurrent, not serial)
	n := 8
	addrs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		a, stop := startListener(t)
		defer stop()
		addrs = append(addrs, a)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	var buf bytes.Buffer
	ok, _ := WaitAll(ctx, addrs, 50*time.Millisecond, &buf, true)
	elapsed := time.Since(start)

	if !ok {
		t.Fatalf("expected all ready")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("too slow for concurrent dials: %s", elapsed)
	}
}

func TestHasColon(t *testing.T) {
	cases := map[string]bool{
		"localhost:5432": true,
		"127.0.0.1:80":   true,
		"redis:6379":     true,
		"nocolon":        false,
		":8080":          false,
		"host:":          false,
		"":               false,
	}
	for in, want := range cases {
		if got := hasColon(in); got != want {
			t.Errorf("hasColon(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResultErrorPopulated(t *testing.T) {
	addr := freePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	r := WaitOne(ctx, addr, 50*time.Millisecond)
	if r.Ready {
		t.Fatal("expected failure")
	}
	if r.Err == nil {
		t.Fatal("expected non-nil error on failure")
	}
	// sanity: error message should reference refused / timeout / connection
	msg := fmt.Sprintf("%v", r.Err)
	if msg == "" {
		t.Fatal("empty error message")
	}
}
