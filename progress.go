package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

var isTTY = term.IsTerminal(int(os.Stderr.Fd()))

// progressTicker drives the progress bar rendering loop.
// TTY gets 1s ticks for smooth animation; non-TTY gets
// 15s ticks to avoid flooding agent transcripts.
type progressTicker struct {
	estimateMs int64
	ticker     *time.Ticker
	quit       chan struct{}
}

func newProgressTicker(
	estimateMs int64,
) *progressTicker {
	return &progressTicker{
		estimateMs: estimateMs,
		quit:       make(chan struct{}),
	}
}

func (p *progressTicker) start(started time.Time) {
	interval := time.Second
	if !isTTY {
		interval = 15 * time.Second
	}
	p.ticker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-p.ticker.C:
				elapsed := time.Since(started)
				drawProgressBar(
					elapsed.Milliseconds(),
					p.estimateMs,
				)
			case <-p.quit:
				return
			}
		}
	}()
}

func (p *progressTicker) stop() {
	p.ticker.Stop()
	close(p.quit)
}

func drawProgressBar(
	elapsedMs, estimateMs int64, overLabel ...string,
) {
	label := "still working"
	if len(overLabel) > 0 && overLabel[0] != "" {
		label = overLabel[0]
	}
	if estimateMs <= 0 {
		// No estimate — show elapsed time only.
		drawIndeterminate(elapsedMs)
		return
	}
	pct := float64(elapsedMs) / float64(estimateMs)

	if !isTTY {
		drawProgressNonTTY(
			pct, label, elapsedMs, estimateMs,
		)
		return
	}
	drawProgressTTY(
		pct, label, elapsedMs, estimateMs,
	)
}

func drawProgressTTY(
	pct float64,
	label string,
	elapsedMs, estimateMs int64,
) {
	barWidth := 30
	if pct > 1.0 {
		overBy := elapsedMs - estimateMs
		bar := strings.Repeat("█", barWidth)
		fmt.Fprintf(os.Stderr,
			"\r  [%s] %s... +%ss over estimate  ",
			bar, label, fmtSec(overBy))
		return
	}
	if pct > 0.99 {
		pct = 0.99
	}

	filled := int(
		math.Round(pct * float64(barWidth)),
	)
	if filled > barWidth {
		filled = barWidth
	}

	remaining := estimateMs - elapsedMs
	if remaining < 0 {
		remaining = 0
	}

	bar := strings.Repeat("█", filled) +
		strings.Repeat("░", barWidth-filled)
	fmt.Fprintf(os.Stderr,
		"\r  [%s] %d%% ~%ss remaining  ",
		bar, int(pct*100), fmtSec(remaining))
}

func drawProgressNonTTY(
	pct float64,
	label string,
	elapsedMs, estimateMs int64,
) {
	if pct > 1.0 {
		fmt.Fprintf(os.Stderr,
			"  %s... %ss elapsed "+
				"(+%ss over estimate)\n",
			label,
			fmtSec(elapsedMs),
			fmtSec(elapsedMs-estimateMs))
		return
	}
	fmt.Fprintf(os.Stderr,
		"  %d%% ~%ss remaining\n",
		int(pct*100),
		fmtSec(estimateMs-elapsedMs))
}

// drawIndeterminate renders a spinner with elapsed time
// when no historical estimate exists yet.
func drawIndeterminate(elapsedMs int64) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := (elapsedMs / 100) % int64(len(frames))
	if isTTY {
		fmt.Fprintf(os.Stderr,
			"\r  %s %s elapsed  ",
			frames[idx],
			formatDurationMs(elapsedMs))
	} else {
		fmt.Fprintf(os.Stderr,
			"  %s elapsed\n",
			formatDurationMs(elapsedMs))
	}
}

func clearProgressBar() {
	if isTTY {
		fmt.Fprintf(os.Stderr,
			"\r%s\r", strings.Repeat(" ", 70))
	}
}

func formatDurationMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	s := ms / 1000
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s = s % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}

func fmtSec(ms int64) string {
	s := ms / 1000
	if s < 0 {
		s = 0
	}
	return fmt.Sprintf("%d", s)
}

func printAgentHint() {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	fmt.Fprintln(os.Stderr,
		"[gta] Output is optimized for AI agent "+
			"consumption. Do not pipe, redirect, "+
			"or truncate — read it in full.")
}
