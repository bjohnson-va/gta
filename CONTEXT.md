# gta Domain Glossary

## Command
The user's program being wrapped. Passed after `--`
or as trailing args. gta executes it, relays its
output, and measures its duration.

## Timing Key
Unique identifier for a command's timing history.
Auto-generated from `(working directory, normalized
command)` or overridden with `--label`. Two runs with
different inline env vars but the same executable and
args share a key.

## Normalized Command
The command string with leading `VAR=VALUE` env var
assignments stripped. `PID=123 npm serve app` normalizes
to `npm serve app`.

## Estimate
The predicted duration for a command, in milliseconds.
Resolved in priority order: stored average from timing
history > user-provided seed (`--estimate`) > zero
(indeterminate mode).

## Sample
One recorded run of a command. Each sample updates the
stored average via a weighted rolling average (3/5 new,
2/5 history), converging to recent reality within ~5
runs.

## Timing Record
Persisted state for a timing key: last run duration,
sample count, and weighted average. Stored as a JSON
file in `~/.config/gta/`, keyed by a SHA-256 hash of
the timing key.

## Quiescence
The event where a still-running process has produced
substantial output (≥256 bytes) and then goes silent
for the quiet timeout (default 5s). gta interprets
this as "ready" — the command has finished its
startup work even though the process hasn't exited.
Designed for dev servers and watch-mode tools.

## Indeterminate Mode
When no estimate exists (first run, no `--estimate`),
the progress display shows a spinner with elapsed time
instead of a percentage bar.

## Agent Mode
Automatic behavior when stdout is not a TTY. Progress
ticks at 15s intervals (vs 1s) to avoid flooding agent
transcripts, and a preamble asks the agent to read
output in full.
