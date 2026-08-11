# gta — guesstimated time of arrival

## Domain
Read CONTEXT.md for the glossary. Use those terms
exactly — "quiescence" not "idle detection", "timing
key" not "cache key", "sample" not "measurement".

## Architecture
Single-binary CLI. No library mode. Three files:
- `main.go` — arg parsing, process lifecycle, 
  quiescence loop
- `progress.go` — TTY/non-TTY rendering, agent hint
- `timing.go` — JSON persistence, weighted average

## Conventions
- 80-char line width
- `go fmt` before committing
- Table-driven tests
- No external dependencies beyond `golang.org/x/term`

## Testing
Run `go test ./...` before committing.

Integration tests that need real subprocesses should
use `bash -c` with sleep/echo to simulate long-running
commands. Keep integration tests under 10s.

## Quiescence design
The output volume heuristic (256 bytes) is the ONLY
guard against false triggers. Do not add time-based
guards — they were tried and failed because mid-build
silent gaps vary unpredictably. See git history for
the iterations that led here.
