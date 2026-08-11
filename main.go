// gta — guesstimated time of arrival
//
// Wraps any command with a progress bar that learns from
// historical run times on this machine.
//
//	gta -- npm ci
//	gta --label my-build -- make all
//	gta --estimate 30s -- npm ci   # seed first run
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"
)

// Default quiescence timeout: if the process is still
// running and no output has been seen for this long,
// declare it "ready" and record the timing.
const defaultQuietTimeout = 5 * time.Second

func main() {
	args := os.Args[1:]
	opts := parseArgs(args)

	if len(opts.cmdArgs) == 0 {
		printUsage()
		os.Exit(1)
	}

	key := buildKey(opts.label, opts.cmdArgs)
	store := newTimingStore()
	estimateMs := store.loadEstimate(
		key, opts.estimateMs,
	)

	cmd := exec.Command(
		opts.cmdArgs[0], opts.cmdArgs[1:]...,
	)
	cmd.Stdin = os.Stdin
	// Put child in its own process group so we can
	// forward signals cleanly.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Pipe stdout/stderr through gta to track output
	// timing and volume for quiescence detection.
	var lastOutput atomic.Int64
	lastOutput.Store(time.Now().UnixMilli())
	var totalBytes atomic.Int64

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"gta: stdout pipe: %v\n", err)
		os.Exit(1)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"gta: stderr pipe: %v\n", err)
		os.Exit(1)
	}

	printAgentHint()

	start := time.Now()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr,
			"gta: failed to start command: %v\n", err)
		os.Exit(1)
	}

	// Relay child output while tracking timestamps.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		relay(
			stdoutPipe, os.Stdout,
			&lastOutput, &totalBytes,
		)
	}()
	go func() {
		defer wg.Done()
		relay(
			stderrPipe, os.Stderr,
			&lastOutput, &totalBytes,
		)
	}()

	// Forward signals to the child process group.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			_ = syscall.Kill(-cmd.Process.Pid,
				sig.(syscall.Signal))
		}
	}()

	// Wait for process exit in the background.
	done := make(chan error, 1)
	go func() {
		wg.Wait()
		done <- cmd.Wait()
	}()

	ticker := newProgressTicker(estimateMs)
	ticker.start(start)

	quietTimeout := opts.quietTimeout
	if quietTimeout == 0 {
		quietTimeout = defaultQuietTimeout
	}

	// Quiescence fires when enough output has been
	// produced (indicating real work happened) and
	// then output goes quiet. 256 bytes is well past
	// a single status line ("Building..." ~20 bytes)
	// but catches even minimal completion output
	// (a single error + "Watch mode enabled").
	const minBytesBeforeQuiet int64 = 256

	quietCheck := time.NewTicker(500 * time.Millisecond)
	defer quietCheck.Stop()

	var cmdErr error
	quiesced := false

	for {
		select {
		case cmdErr = <-done:
			ticker.stop()
			goto finished

		case <-quietCheck.C:
			if totalBytes.Load() < minBytesBeforeQuiet {
				continue
			}
			last := lastOutput.Load()
			sinceLast := time.Since(
				time.UnixMilli(last),
			)
			if sinceLast >= quietTimeout {
				ticker.stop()
				quiesced = true
				goto finished
			}
		}
	}

finished:
	elapsed := time.Since(start)
	elapsedMs := elapsed.Milliseconds()

	clearProgressBar()

	if quiesced {
		fmt.Fprintf(os.Stderr,
			"gta: ready in %s (process still running)\n",
			formatDurationMs(elapsedMs))
		store.saveEstimate(key, elapsedMs)
		// Wait for the process to exit (or signal).
		cmdErr = <-done
		if cmdErr != nil {
			if exitErr, ok :=
				cmdErr.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr,
			"gta: command failed after %s\n",
			formatDurationMs(elapsedMs))
		if exitErr, ok :=
			cmdErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr,
		"gta: done in %s\n",
		formatDurationMs(elapsedMs))
	store.saveEstimate(key, elapsedMs)
}

// relay copies from r to w, updating lastOutput on
// each read. This lets gta detect when output goes
// quiet.
func relay(
	r io.Reader,
	w io.Writer,
	lastOutput *atomic.Int64,
	totalBytes *atomic.Int64,
) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			lastOutput.Store(
				time.Now().UnixMilli(),
			)
			totalBytes.Add(int64(n))
			_, _ = w.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

type parsedOpts struct {
	label        string
	estimateMs   int64
	quietTimeout time.Duration
	cmdArgs      []string
}

// parseArgs extracts --label, --estimate,
// --quiet-timeout, and the command after --.
func parseArgs(args []string) parsedOpts {
	opts := parsedOpts{}

	i := 0
	for i < len(args) {
		switch {
		case args[i] == "--":
			opts.cmdArgs = args[i+1:]
			return opts

		case args[i] == "--label" &&
			i+1 < len(args):
			opts.label = args[i+1]
			i += 2

		case strings.HasPrefix(args[i], "--label="):
			opts.label = strings.TrimPrefix(
				args[i], "--label=")
			i++

		case args[i] == "--estimate" &&
			i+1 < len(args):
			opts.estimateMs = parseDuration(args[i+1])
			i += 2

		case strings.HasPrefix(
			args[i], "--estimate="):
			opts.estimateMs = parseDuration(
				strings.TrimPrefix(
					args[i], "--estimate="))
			i++

		case args[i] == "--quiet-timeout" &&
			i+1 < len(args):
			d, err := time.ParseDuration(args[i+1])
			if err == nil {
				opts.quietTimeout = d
			}
			i += 2

		case strings.HasPrefix(
			args[i], "--quiet-timeout="):
			d, err := time.ParseDuration(
				strings.TrimPrefix(
					args[i], "--quiet-timeout="))
			if err == nil {
				opts.quietTimeout = d
			}
			i++

		case args[i] == "--help" || args[i] == "-h":
			printUsage()
			os.Exit(0)

		default:
			opts.cmdArgs = args[i:]
			return opts
		}
	}
	return opts
}

// parseDuration parses "30s", "2m", "90" (seconds).
func parseDuration(s string) int64 {
	d, err := time.ParseDuration(s)
	if err == nil {
		return d.Milliseconds()
	}
	// Try bare number as seconds.
	var secs float64
	if _, err := fmt.Sscanf(
		s, "%f", &secs,
	); err == nil {
		return int64(secs * 1000)
	}
	return 0
}

// buildKey creates a timing key from the label or from
// (cwd, normalized command).
func buildKey(
	label string, cmdArgs []string,
) string {
	if label != "" {
		return label
	}
	cwd, _ := os.Getwd()
	normalized := normalizeCommand(cmdArgs)
	return cwd + "\x00" + normalized
}

// normalizeCommand strips leading VAR=VALUE env
// assignments and joins the remaining args.
func normalizeCommand(args []string) string {
	i := 0
	for i < len(args) {
		if looksLikeEnvVar(args[i]) {
			i++
			continue
		}
		break
	}
	return strings.Join(args[i:], " ")
}

// looksLikeEnvVar returns true for strings like
// "FOO=bar" where the left side is a valid env var name.
func looksLikeEnvVar(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	name := s[:eq]
	for i, r := range name {
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
		if !unicode.IsLetter(r) &&
			!unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func printUsage() {
	fmt.Fprintf(os.Stderr,
		`gta — guesstimated time of arrival

Usage:
  gta [options] -- <command> [args...]
  gta [options] <command> [args...]

Options:
  --label <name>        Override the timing key
  --estimate <dur>      Seed estimate for first run
                        (e.g. 30s, 2m, 90)
  --quiet-timeout <dur> Quiescence timeout for
                        long-running processes
                        (default 5s)
  -h, --help            Show this help

Examples:
  gta -- npm ci
  gta --label my-build -- make all
  gta --estimate 30s -- npm ci
`)
}
