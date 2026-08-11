# gta — guesstimated time of arrival

Wraps any command with a progress bar that learns from historical run
times on your machine. No configuration needed — first run observes,
second run estimates. Works for both finite commands (`npm ci`, `go test`)
and long-running processes (`nx serve`, dev servers).

## Install

```sh
go install github.com/bjohnson-va/gta@latest
```

## Usage

```sh
gta -- npm ci
gta -- npx nx serve business-center-client
gta --label galaxy-serve -- npm serve business-center-client
gta --estimate 30s -- npm ci   # seed the first run
```

### How it works

**Finite commands** (builds, tests, installs):

1. First run: spinner with elapsed time
2. Second run onward: progress bar with ETA based on
   a weighted rolling average of past runs
3. Timing is recorded when the command exits

**Long-running processes** (dev servers, watch mode):

1. gta intercepts stdout/stderr while passing it through
2. When the process has produced substantial output (≥256 bytes)
   and then goes quiet for 5 seconds, gta declares it "ready"
3. Timing is recorded at that point; the process keeps running
4. Future runs show a progress bar based on learned startup time

### Progress bar

**TTY (interactive terminal):**

```
  [████████████████░░░░░░░░░░░░░░] 53% ~42s remaining
```

When elapsed exceeds the estimate:

```
  [██████████████████████████████] still working... +12s over estimate
```

When readiness is detected for a long-running process:

```
  gta: ready in 1m28s (process still running)
```

**Non-TTY (agent/CI):**

Prints one status line every 15 seconds to avoid flooding
transcripts:

```
  53% ~42s remaining
```

### Agent-friendly

When stdout is not a terminal (e.g., captured by an AI agent),
`gta` automatically:

- Switches to plain-text status lines instead of animated bars
- Reduces tick frequency to 15s
- Prints a preamble asking the agent to read output in full

### Options

| Flag | Description |
|------|-------------|
| `--label <name>` | Override the timing key |
| `--estimate <dur>` | Seed estimate for first run (e.g., `30s`, `2m`, `90`) |
| `--quiet-timeout <dur>` | How long output must be silent before declaring "ready" (default `5s`) |
| `-h`, `--help` | Show help |

### Timing storage

Timing history is stored in `~/.config/gta/` as JSON files.
The weighted average gives new samples 3/5 weight and history
2/5 weight, converging to recent reality within ~5 runs.

### Command normalization

Leading `VAR=VALUE` environment variable assignments are
stripped from the timing key and set on the child process,
so these share the same history:

```sh
gta -- PID=123 npm serve app
gta -- PID=456 npm serve app
```

## License

MIT
