package main

import (
	"os"
	"testing"
)

func TestTimingStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	s.saveEstimate("test-key", 5000)
	got := s.loadEstimate("test-key", 0)
	if got != 5000 {
		t.Errorf(
			"first save: got %d, want 5000", got,
		)
	}
}

func TestTimingStore_WeightedAverageConverges(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	s.saveEstimate("k", 10000)
	s.saveEstimate("k", 10000)
	s.saveEstimate("k", 10000)
	s.saveEstimate("k", 10000)
	s.saveEstimate("k", 10000)

	// After 5 identical samples the average should
	// have converged to the sample value.
	got := s.loadEstimate("k", 0)
	if got != 10000 {
		t.Errorf(
			"converged avg: got %d, want 10000", got,
		)
	}
}

func TestTimingStore_AverageDriftsTowardNewSamples(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	s.saveEstimate("k", 10000)
	s.saveEstimate("k", 20000)

	// avg = (10000*2 + 20000*3) / 5 = 16000
	got := s.loadEstimate("k", 0)
	if got != 16000 {
		t.Errorf(
			"drifted avg: got %d, want 16000", got,
		)
	}
}

func TestTimingStore_MissingKeyReturnsSeed(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	got := s.loadEstimate("nonexistent", 5000)
	if got != 5000 {
		t.Errorf("seed fallback: got %d, want 5000", got)
	}
}

func TestTimingStore_MissingKeyNoSeedReturnsZero(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	got := s.loadEstimate("nonexistent", 0)
	if got != 0 {
		t.Errorf(
			"no-seed fallback: got %d, want 0", got,
		)
	}
}

func TestTimingStore_CorruptFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	// Write a valid entry, then corrupt the file.
	s.saveEstimate("k", 5000)
	path := s.path("k")
	_ = os.WriteFile(path, []byte("not json"), 0644)

	got := s.loadEstimate("k", 99)
	if got != 99 {
		t.Errorf(
			"corrupt file: got %d, want seed 99", got,
		)
	}
}

func TestTimingStore_ClampsSubMillisecondToOne(t *testing.T) {
	dir := t.TempDir()
	s := &timingStore{dir: dir}

	s.saveEstimate("k", 0)
	got := s.loadEstimate("k", 0)
	if got < 1 {
		t.Errorf(
			"sub-ms clamp: got %d, want >= 1", got,
		)
	}
}
