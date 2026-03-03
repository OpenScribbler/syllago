---
paths:
  - "cli/internal/tui/*_test.go"
---

# TUI Test Conventions

## Golden File Tests

Visual output tests use golden files in `testdata/`:
- Component tests: `component-*.golden` (individual pieces)
- Full app tests: `fullapp-*.golden` (complete layouts at specific sizes)
- Size variants: `-60x20`, `-120x40`, `-160x50` (responsive testing)

**After ANY visual change, regenerate goldens:**
```bash
go test ./cli/internal/tui/ -update-golden
```
Then review the diff to confirm changes are intentional.

**Path normalization:** Tests replace temp directory paths with `<TESTDIR>` and strip trailing whitespace so golden files are deterministic across machines.

## Test Helpers

Use `testhelpers_test.go` utilities:
- `requireGolden(t, name, got)` — compare output to golden file
- Test fixtures create mock catalog items with known paths

## Test Structure

- Table-driven tests with `t.Run()` subtests
- Test both keyboard and mouse interactions for interactive components
- Modal tests should verify: open → navigate → confirm/cancel → state after close
- Always test Esc dismissal and click-away behavior for modals
