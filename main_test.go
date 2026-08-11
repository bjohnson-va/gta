package main

import (
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLabel string
		wantEstMs int64
		wantCmd   []string
	}{
		{
			name:    "bare command after double dash",
			args:    []string{"--", "npm", "ci"},
			wantCmd: []string{"npm", "ci"},
		},
		{
			name:    "bare command without double dash",
			args:    []string{"npm", "ci"},
			wantCmd: []string{"npm", "ci"},
		},
		{
			name:      "label with space-separated value",
			args:      []string{"--label", "my-build", "--", "make"},
			wantLabel: "my-build",
			wantCmd:   []string{"make"},
		},
		{
			name:      "label with equals sign",
			args:      []string{"--label=my-build", "--", "make"},
			wantLabel: "my-build",
			wantCmd:   []string{"make"},
		},
		{
			name:      "estimate parses duration",
			args:      []string{"--estimate", "30s", "--", "npm", "ci"},
			wantEstMs: 30000,
			wantCmd:   []string{"npm", "ci"},
		},
		{
			name:      "estimate parses bare seconds",
			args:      []string{"--estimate=90", "--", "npm", "ci"},
			wantEstMs: 90000,
			wantCmd:   []string{"npm", "ci"},
		},
		{
			name: "no args yields nil command",
			args: []string{},
		},
		{
			name: "label and estimate together",
			args: []string{
				"--label", "x",
				"--estimate", "2m",
				"--", "sleep", "1",
			},
			wantLabel: "x",
			wantEstMs: 120000,
			wantCmd:   []string{"sleep", "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseArgs(tt.args)
			if opts.label != tt.wantLabel {
				t.Errorf(
					"label = %q, want %q",
					opts.label, tt.wantLabel,
				)
			}
			if opts.estimateMs != tt.wantEstMs {
				t.Errorf(
					"estimateMs = %d, want %d",
					opts.estimateMs, tt.wantEstMs,
				)
			}
			if !slicesEqual(
				opts.cmdArgs, tt.wantCmd,
			) {
				t.Errorf(
					"cmd = %v, want %v",
					opts.cmdArgs, tt.wantCmd,
				)
			}
		})
	}
}

func TestNormalizeCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no env vars passes through",
			args: []string{"npm", "ci"},
			want: "npm ci",
		},
		{
			name: "strips leading env vars",
			args: []string{
				"PID=123", "NODE_ENV=prod",
				"npm", "serve",
			},
			want: "npm serve",
		},
		{
			name: "path-like values are not env vars",
			args: []string{
				"./run.sh", "--flag",
			},
			want: "./run.sh --flag",
		},
		{
			name: "digit-leading name is not env var",
			args: []string{"1FOO=bar", "cmd"},
			want: "1FOO=bar cmd",
		},
		{
			name: "all env vars yields empty string",
			args: []string{"FOO=bar"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCommand(tt.args)
			if got != tt.want {
				t.Errorf(
					"normalizeCommand(%v) = %q, want %q",
					tt.args, got, tt.want,
				)
			}
		})
	}
}

func TestLooksLikeEnvVar(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"simple", "FOO=bar", true},
		{"underscore", "MY_VAR=1", true},
		{"no equals", "FOO", false},
		{"leading digit", "1FOO=bar", false},
		{"empty name", "=bar", false},
		{"path", "./foo=bar", false},
		{"hyphen in name", "MY-VAR=1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeEnvVar(tt.s)
			if got != tt.want {
				t.Errorf(
					"looksLikeEnvVar(%q) = %v, want %v",
					tt.s, got, tt.want,
				)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int64
	}{
		{"go duration seconds", "30s", 30000},
		{"go duration minutes", "2m", 120000},
		{"bare number as seconds", "90", 90000},
		{"fractional seconds", "1.5", 1500},
		{"invalid returns zero", "xyz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDuration(tt.s)
			if got != tt.want {
				t.Errorf(
					"parseDuration(%q) = %d, want %d",
					tt.s, got, tt.want,
				)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
