package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// timingStore persists per-command duration history so
// subsequent runs can show a progress bar. Storage lives
// in ~/.config/gta/ with one JSON file hashed from the
// timing key — avoids filesystem-unsafe characters in
// filenames while keeping the store inspectable.
type timingStore struct {
	dir string
}

type timing struct {
	Key     string `json:"key"`
	LastMs  int64  `json:"lastMs"`
	Samples int    `json:"samples"`
	AvgMs   int64  `json:"avgMs"`
}

func newTimingStore() *timingStore {
	dir := configDir()
	_ = os.MkdirAll(dir, 0755)
	return &timingStore{dir: dir}
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gta")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gta")
}

// loadEstimate returns the stored average for key.
// Falls back to seedMs if provided and positive, then
// to 0 (which triggers indeterminate mode).
func (s *timingStore) loadEstimate(
	key string, seedMs int64,
) int64 {
	t := s.load(key)
	if t != nil && t.AvgMs > 0 {
		return t.AvgMs
	}
	if seedMs > 0 {
		return seedMs
	}
	return 0
}

// saveEstimate records a new sample using a weighted
// rolling average: 3/5 new, 2/5 history. Converges to
// recent reality within ~5 samples.
func (s *timingStore) saveEstimate(
	key string, elapsedMs int64,
) {
	if elapsedMs < 1 {
		elapsedMs = 1
	}
	t := s.load(key)
	if t == nil {
		t = &timing{Key: key}
	}
	t.LastMs = elapsedMs
	t.Samples++
	if t.AvgMs == 0 {
		t.AvgMs = elapsedMs
	} else {
		t.AvgMs = (t.AvgMs*2 + elapsedMs*3) / 5
	}
	if t.AvgMs < 1 {
		t.AvgMs = 1
	}
	s.save(t)
}

func (s *timingStore) path(key string) string {
	h := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(h[:8]) + ".json"
	return filepath.Join(s.dir, name)
}

func (s *timingStore) load(key string) *timing {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return nil
	}
	var t timing
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

func (s *timingStore) save(t *timing) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path(t.Key), data, 0644)
}
