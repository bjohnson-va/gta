package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestQuiescence_FiresAfterOutputBurstThenSilence(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("integration test")
	}

	binary := buildGTA(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Quiescence fires after 2s of silence, then gta
	// blocks on child exit — sleep must be short enough
	// that the child exits within the test timeout.
	// Output must exceed minBytesBeforeQuiet (4096).
	cmd := exec.Command(binary,
		"--quiet-timeout", "2s",
		"--", "bash", "-c",
		`for i in $(seq 1 300); do echo "line $i padding text to reach byte threshold"; done; sleep 4`,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		t.Fatal("gta did not detect quiescence within 15s")
	}

	elapsed := time.Since(start)
	output := stderr.String()

	if !strings.Contains(output, "ready in") {
		t.Errorf(
			"expected 'ready in' message, got:\n%s",
			output,
		)
	}
	if elapsed > 10*time.Second {
		t.Errorf(
			"quiescence took %v, expected under 10s",
			elapsed,
		)
	}
}

func TestQuiescence_DoesNotFireBelowByteThreshold(
	t *testing.T,
) {
	if testing.Short() {
		t.Skip("integration test")
	}

	binary := buildGTA(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Emit < 4096 bytes then go silent. Quiescence
	// should NOT fire — gta should keep waiting.
	cmd := exec.Command(binary,
		"--quiet-timeout", "1s",
		"--", "bash", "-c",
		`echo "tiny"; sleep 5`,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("gta hung unexpectedly")
	}

	output := stderr.String()
	if strings.Contains(output, "ready in") {
		t.Errorf(
			"quiescence fired below byte threshold:\n%s",
			output,
		)
	}
	if !strings.Contains(output, "done in") {
		t.Errorf(
			"expected normal 'done in' exit, got:\n%s",
			output,
		)
	}
}

func TestFiniteCommand_RecordsTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	binary := buildGTA(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cmd := exec.Command(binary,
		"--", "bash", "-c", "sleep 1",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "done in") {
		t.Errorf(
			"expected 'done in' message, got:\n%s",
			output,
		)
	}

	entries, _ := os.ReadDir(
		dir + "/gta",
	)
	if len(entries) == 0 {
		t.Error("no timing file created")
	}
}

func TestFailingCommand_PreservesExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	binary := buildGTA(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cmd := exec.Command(binary,
		"--", "bash", "-c", "exit 42",
	)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf(
			"exit code = %d, want 42",
			exitErr.ExitCode(),
		)
	}
}

func buildGTA(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/gta"
	cmd := exec.Command(
		"go", "build", "-o", binary, ".",
	)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}
