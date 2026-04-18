package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

const usage = `waitport — block until tcp ports are reachable

usage:
  waitport [flags] host:port [host:port ...]

flags:
  --timeout DURATION   total time to wait before giving up (default 60s)
  --interval DURATION  poll interval per target (default 250ms)
  --quiet              suppress progress output (errors still print)
  -h, --help           show this help

exit codes:
  0  all targets reachable
  1  timeout or argument error

examples:
  waitport localhost:5432 --timeout 30s
  waitport db:5432 redis:6379 --interval 500ms
`

func main() {
	fs := flag.NewFlagSet("waitport", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		timeout  time.Duration
		interval time.Duration
		quiet    bool
		help     bool
	)
	fs.DurationVar(&timeout, "timeout", 60*time.Second, "total timeout")
	fs.DurationVar(&interval, "interval", 250*time.Millisecond, "poll interval")
	fs.BoolVar(&quiet, "quiet", false, "suppress progress output")
	fs.BoolVar(&help, "help", false, "show help")
	fs.BoolVar(&help, "h", false, "show help (short)")

	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	if err := fs.Parse(normalizeArgs(os.Args[1:])); err != nil {
		os.Exit(1)
	}

	if help {
		fmt.Print(usage)
		return
	}

	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "error: need at least one host:port target")
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	// light validation: must contain a colon
	for _, t := range targets {
		if !hasColon(t) {
			fmt.Fprintf(os.Stderr, "error: %q is not a valid host:port\n", t)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !quiet {
		fmt.Printf("waiting for %d endpoint(s) (timeout %s)...\n", len(targets), timeout)
	}

	ok, _ := WaitAll(ctx, targets, interval, os.Stdout, quiet)
	if !ok {
		if !quiet {
			fmt.Fprintln(os.Stderr, "timed out.")
		} else {
			fmt.Fprintln(os.Stderr, "waitport: timeout")
		}
		os.Exit(1)
	}
	if !quiet {
		fmt.Println("all ready.")
	}
}

// normalizeArgs converts long-form `--flag` into `-flag` and reorders
// so that all flags come before positional args. this lets users write
// `waitport host:port --timeout 5s` the way they expect, since stdlib
// flag stops at the first non-flag arg.
func normalizeArgs(args []string) []string {
	knownBool := map[string]bool{"quiet": true, "help": true, "h": true}

	var flags []string
	var positional []string

	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		// normalise double-dash
		stripped := a
		isLong := len(a) > 2 && a[0] == '-' && a[1] == '-'
		isShort := len(a) > 1 && a[0] == '-' && !isLong
		if isLong {
			stripped = a[1:]
		}
		if isLong || isShort {
			name := stripped[1:]
			hasEq := false
			for j := 0; j < len(name); j++ {
				if name[j] == '=' {
					name = name[:j]
					hasEq = true
					break
				}
			}
			flags = append(flags, stripped)
			// if flag is not boolean and no '=' present, consume next arg as value
			if !hasEq && !knownBool[name] && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i += 2
				continue
			}
			i++
			continue
		}
		positional = append(positional, a)
		i++
	}
	return append(flags, positional...)
}

func hasColon(s string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i > 0 && i < len(s)-1
		}
	}
	return false
}
