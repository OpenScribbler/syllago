# Design: Comprehensive TUI Integration Tests

## Problem

The Nesco TUI has 6 screens, ~165 distinct user interactions, and currently only 3 basic navigation tests (plus 2 file browser tests). After implementing the settings screen, content transparency, and directory-per-item restructuring, two bugs slipped through (hooks showing 0 items, missing descriptions). Comprehensive tests will catch these classes of issues going forward.

## Design Decisions

### Hybrid Testing Strategy

**Decision:** Direct model tests (Update() + state assertions) for exhaustive state coverage, plus teatest integration tests for rendered output verification.

**Why:** Direct tests are fast and deterministic—they call Update() and check state fields. Teatest catches rendering regressions that state-only tests miss (wrong text, broken layout, missing UI elements).

**Trade-offs considered:**
- Pure direct tests: Fast but miss rendering bugs
- Pure teatest: Catches everything but slow, fragile golden files
- Hybrid: Best of both worlds, more code to maintain

### String Assertions Over Golden Files

**Decision:** Use `strings.Contains()` checks instead of golden file snapshots for most tests. Only add golden files for 3-5 key screens in the teatest integration tests.

**Why:** Golden files break on any UI tweak (color change, padding, wording). String assertions are resilient and produce readable failure messages. Golden files are reserved for high-value screens where exact layout matters.

### Test Fixtures: Real Files on Disk

**Decision:** Use `t.TempDir()` with real files for test fixtures rather than mocking the catalog.

**Why:** The detail screen's file viewer reads actual files. The import workflow detects content types from disk. Real files ensure these paths work. The existing tests already use this pattern.

### Side Effect Isolation

**Decision:** Test state transitions rather than actual side effects (filesystem writes, clipboard, git operations).

**Why:** Tests should verify the TUI's state machine, not the installer/git/clipboard implementations. Those have their own tests. The TUI tests verify that pressing 'i' sets the right state and returns the right command.

## Scope

### In Scope
- All 6 screens: Category, Items, Detail, Import, Update, Settings
- Global search overlay
- Environment variable setup wizard (detail sub-flow)
- ~165 test cases covering all user interactions
- Teatest integration tests for rendered output verification

### Out of Scope
- File browser tests (already exist in filebrowser_test.go)
- Installer/uninstaller behavior (tested separately)
- Provider detection logic (tested separately)
- Git operations (tested separately)

## Architecture

### File Organization
One test file per screen/feature, plus shared helpers:
- `testhelpers_test.go` — shared fixtures and assertion helpers
- `category_test.go` — main menu screen
- `items_test.go` — item list screen
- `detail_test.go` — detail view (tabs, overview, files, install)
- `detail_env_test.go` — env var setup wizard
- `settings_test.go` — settings screen
- `search_test.go` — global search
- `import_test.go` — import workflow
- `update_test.go` — update workflow
- `integration_test.go` — teatest full-program tests

### Dependencies
- `github.com/charmbracelet/x/exp/teatest` — for integration tests
- Standard `testing` package — for direct model tests
