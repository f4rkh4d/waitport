package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Result holds the outcome of waiting on a single target.
type Result struct {
	Target   string
	Ready    bool
	Duration time.Duration
	Err      error
}

// WaitOne blocks until the target host:port accepts a TCP connection,
// retrying on interval, or until ctx is cancelled.
func WaitOne(ctx context.Context, target string, interval time.Duration) Result {
	start := time.Now()
	var lastErr error
	dialer := net.Dialer{Timeout: interval}

	for {
		dialCtx, cancel := context.WithTimeout(ctx, interval)
		conn, err := dialer.DialContext(dialCtx, "tcp", target)
		cancel()
		if err == nil {
			_ = conn.Close()
			return Result{Target: target, Ready: true, Duration: time.Since(start)}
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return Result{
				Target:   target,
				Ready:    false,
				Duration: time.Since(start),
				Err:      lastErr,
			}
		case <-time.After(interval):
		}
	}
}

// WaitAll polls all targets concurrently and writes progress lines to out
// unless quiet is true. Returns true if every target became reachable
// before the context expired.
func WaitAll(ctx context.Context, targets []string, interval time.Duration, out io.Writer, quiet bool) (bool, []Result) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]Result, len(targets))

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target string) {
			defer wg.Done()
			r := WaitOne(ctx, target, interval)
			mu.Lock()
			results[idx] = r
			if !quiet {
				if r.Ready {
					fmt.Fprintf(out, "  \u2713 %s ready in %s\n", r.Target, roundDur(r.Duration))
				} else {
					fmt.Fprintf(out, "  \u2717 %s failed after %s: %v\n", r.Target, roundDur(r.Duration), r.Err)
				}
			}
			mu.Unlock()
		}(i, t)
	}

	wg.Wait()

	ok := true
	for _, r := range results {
		if !r.Ready {
			ok = false
			break
		}
	}
	return ok, results
}

func roundDur(d time.Duration) time.Duration {
	if d < time.Second {
		return d.Round(time.Millisecond)
	}
	return d.Round(100 * time.Millisecond)
}
