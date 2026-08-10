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
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"
)

func main() {
	args := os.Args[1:]
	label, estimate, cmdArgs := parseArgs(args)

	if len(cmdArgs) == 0 {
		printUsage()
		os.Exit(1)
	}

	key := buildKey(label, cmdArgs)
	store := newTimingStore()
	estimateMs := store.loadEstimate(key, estimate)

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Put child in its own process group so we can
	// forward signals cleanly.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	printAgentHint()

	start := time.Now()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr,
			"gta: failed to start command: %v\n", err)
		os.Exit(1)
	}

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

	// Progress rendering loop.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := newProgressTicker(estimateMs)
	ticker.start(start)

	var cmdErr error
	select {
	case cmdErr = <-done:
		ticker.stop()
	}

	elapsed := time.Since(start)
	elapsedMs := elapsed.Milliseconds()

	clearProgressBar()

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr,
			"gta: command failed after %s\n",
			formatDurationMs(elapsedMs))
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr,
		"gta: done in %s\n", formatDurationMs(elapsedMs))
	store.saveEstimate(key, elapsedMs)
}

// parseArgs extracts --label, --estimate, and the command
// after --. Returns (label, estimateMs, cmdArgs).
func parseArgs(
	args []string,
) (string, int64, []string) {
	label := ""
	var estimateMs int64

	i := 0
	for i < len(args) {
		switch {
		case args[i] == "--":
			return label, estimateMs, args[i+1:]

		case args[i] == "--label" && i+1 < len(args):
			label = args[i+1]
			i += 2

		case strings.HasPrefix(args[i], "--label="):
			label = strings.TrimPrefix(
				args[i], "--label=")
			i++

		case args[i] == "--estimate" &&
			i+1 < len(args):
			estimateMs = parseDuration(args[i+1])
			i += 2

		case strings.HasPrefix(
			args[i], "--estimate="):
			estimateMs = parseDuration(
				strings.TrimPrefix(
					args[i], "--estimate="))
			i++

		case args[i] == "--help" || args[i] == "-h":
			printUsage()
			os.Exit(0)

		default:
			// Everything from here on is the command.
			return label, estimateMs, args[i:]
		}
	}
	return label, estimateMs, nil
}

// parseDuration parses "30s", "2m", "90" (seconds).
func parseDuration(s string) int64 {
	d, err := time.ParseDuration(s)
	if err == nil {
		return d.Milliseconds()
	}
	// Try bare number as seconds.
	var secs float64
	if _, err := fmt.Sscanf(s, "%f", &secs); err == nil {
		return int64(secs * 1000)
	}
	return 0
}

// buildKey creates a timing key from the label or from
// (cwd, normalized command).
func buildKey(label string, cmdArgs []string) string {
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
	fmt.Fprintf(os.Stderr, `gta — guesstimated time of arrival

Usage:
  gta [options] -- <command> [args...]
  gta [options] <command> [args...]

Options:
  --label <name>      Override the timing key
  --estimate <dur>    Seed estimate for first run
                      (e.g. 30s, 2m, 90)
  -h, --help          Show this help

Examples:
  gta -- npm ci
  gta --label my-build -- make all
  gta --estimate 30s -- npm ci
`)
}
