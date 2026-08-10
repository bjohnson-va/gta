# gta — guesstimated time of arrival

Wraps any command with a progress bar that learns from historical run
times on your machine. No configuration needed — first run observes,
second run estimates.

## Install

```sh
go install github.com/bjohnson-va/gta@latest
```

## Usage

```sh
gta -- npm ci
gta --label galaxy-serve -- npm serve business-center-client
gta --estimate 30s -- npm ci   # seed the first run
```

### How it works

1. Run a command through `gta`
2. First run: shows elapsed time with a spinner (no bar)
3. Second run onward: shows a progress bar with ETA based
   on a weighted rolling average of past runs

The timing key is `(working directory, command)` by default.
Use `--label` when you need a custom key.

### Progress bar

**TTY (interactive terminal):**

```
  [████████████████░░░░░░░░░░░░░░] 53% ~42s remaining
```

When elapsed exceeds the estimate:

```
  [██████████████████████████████] still working... +12s over estimate
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
| `-h`, `--help` | Show help |

### Timing storage

Timing history is stored in `~/.config/gta/` as JSON files.
The weighted average gives new samples 3/5 weight and history
2/5 weight, converging to recent reality within ~5 runs.

### Command normalization

Leading `VAR=VALUE` environment variable assignments are
stripped from the timing key, so these share the same history:

```sh
gta -- PID=123 npm serve app
gta -- PID=456 npm serve app
```

## License

MIT
