# Go-Dev Wrapper Reference

Complete reference for `go-dev` (Mac/Linux) and `go-dev.ps1` (Windows) wrappers (80-85% token reduction).

---

## Prohibition

> **NEVER run raw `go test`, `go build`, or `go vet` commands.**
> Always use `go-dev` (Mac/Linux) or `go-dev.ps1` (Windows).

## Locating the Wrapper

- Mac/Linux: `~/.claude/bin/go-dev` or `which go-dev`
- Windows: `~/.claude/bin/go-dev.ps1` or `Get-Command go-dev.ps1`

## Commands

```bash
go-dev build ./...              # Build with errors only
go-dev test ./...               # Tests with pass/fail summary
go-dev test-v ./...             # Tests with more failure context
go-dev test-short ./...         # Skip slow/integration tests
go-dev test-tags integration ./... # Include tagged tests
go-dev vet ./...                # Vet with limited output
go-dev fmt ./...                # Format, show changed files
go-dev race ./...               # Race detection, issues only
go-dev cover ./...              # Coverage summary
go-dev bench ./...              # All benchmarks, results only
go-dev bench-run BenchmarkX ./... # Specific benchmark, 5s benchtime
go-dev escape ./...             # Escape analysis, heap escapes only
go-dev check ./...              # Build + vet + test in one
go-dev mod                      # Tidy and verify modules
go-dev lint ./...               # staticcheck or vet fallback
```

On Windows, replace `go-dev` with `go-dev.ps1`.

## Chaining

```bash
# Mac/Linux
~/.claude/bin/go-dev test ./... && ~/.claude/bin/go-dev build ./...

# Windows PowerShell
~/.claude/bin/go-dev.ps1 test ./...; ~/.claude/bin/go-dev.ps1 build ./...
```

## When to Use Each

| Task | Command |
|------|---------|
| Quick verification | `go-dev check ./...` |
| After code changes | `go-dev build ./...` |
| Before committing | `go-dev test ./...` |
| Debug test failures | `go-dev test-v ./pkg/...` |
| Concurrency work | `go-dev race ./...` |
| Allocation analysis | `go-dev escape ./...` |
| Escape + inlining | `GO_DEV_VERBOSE=1 go-dev escape ./...` |

## Environment Variables

- `GO_DEV_VERBOSE=1` -- more output lines
- `GO_DEV_RAW=1` -- unfiltered output (use sparingly)

## Fallback (Raw Commands)

Only use raw commands when ALL true:
1. Wrapper not at `~/.claude/bin/go-dev` (or `go-dev.ps1`)
2. Wrapper not in PATH (`which go-dev` / `Get-Command go-dev.ps1` fails)
3. User explicitly approves

Then: `go test ./... 2>&1 | head -50` (Mac/Linux) or `go test ./... 2>&1 | Select-Object -First 50` (Windows).
