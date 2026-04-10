# TUI Golden File Testing Infrastructure

*Design date: 2026-02-25*

## Problem

The current TUI development workflow requires manual visual inspection for every change. Tests verify state transitions and text content but cannot detect layout regressions, styling changes, alignment shifts, or broken rendering. This creates a bottleneck: every TUI change requires the developer to run syllago and walk through flows visually.

## Goal

Automated visual regression testing for the syllago TUI. After a change, `make test` tells you if any screen's visual output changed, and `git diff testdata/` shows you exactly what changed — eliminating the manual inspection loop.

## Approach: Hybrid Golden Files + Targeted Assertions

Three-layer testing pyramid:

### Layer 1: Direct Model Tests (Existing)
- Call `Update()` with messages, assert on model state
- Already ~5,800 lines in 19 test files
- Covers: navigation, state transitions, edge cases
- **No changes needed** — these stay as-is

### Layer 2: teatest Integration Tests (Existing)
- Full program simulation with `teatest.NewTestModel`
- 7 integration tests covering multi-step flows
- Covers: category→items→detail, search, settings, import, resize, quit
- **No changes needed** — these stay as-is

### Layer 3: Golden File Visual Regression (NEW)
- Snapshot `View()` output for key screens
- Compare against `.golden` reference files
- `go test -update` refreshes golden files when UI intentionally changes
- `git diff testdata/` IS the visual review

## Golden File Strategy

### Full-App Snapshots (~8 files)
Capture the complete rendered screen (sidebar + content panel) for key states:

1. **Category welcome** — First thing users see after launch
2. **Items list (Skills)** — Items with descriptions in content panel
3. **Detail overview tab** — Sidebar + detail with README content
4. **Detail files tab** — File tree viewer
5. **Detail install tab** — Provider checkboxes
6. **Search results** — Filtered items after search query
7. **Modal rendering** — Confirmation modal overlay
8. **Settings screen** — Settings with toggles

### Component-Isolated Snapshots (~6 files)
Capture individual sub-model `View()` output in isolation:

1. **Sidebar only** — Category list with selection indicator
2. **Items list only** — Just the content panel items
3. **Detail tabs only** — Tab bar rendering
4. **Modal only** — Modal component in isolation
5. **Help overlay** — Keyboard shortcut help screen
6. **File browser** — File tree component

### Total: ~14 golden files

## Infrastructure Requirements

### 1. Golden file package setup
Use `github.com/charmbracelet/x/exp/golden` for standalone golden file comparison. This works with direct model tests (no teatest needed).

### 2. `-update` flag registration
Register a global `-update` flag so `go test -update ./cli/internal/tui/...` refreshes all golden files:

```go
// golden_test.go
var update = flag.Bool("update", false, "update golden files")
```

### 3. `.gitattributes` for golden files
Prevent Git from corrupting golden files or showing noisy diffs:

```
cli/internal/tui/testdata/*.golden binary
```

Note: Using `binary` means `git diff` won't inline the diff by default. An alternative is `diff=golden` with a custom diff driver, or just not marking as binary and accepting raw diffs.

### 4. Deterministic rendering
Already handled — `NO_COLOR=1` is set in `testhelpers_test.go` init(). Also need to ensure:
- Fixed terminal dimensions (80x30 via `testApp()`)
- No time-dependent content in views
- `zone.NewGlobal()` already called

### 5. ANSI stripping for golden files
Use `github.com/charmbracelet/x/ansi` Strip() to remove escape sequences. Golden files should contain plain text for readable diffs. `NO_COLOR=1` already handles most of this, but belt-and-suspenders with explicit stripping.

### 6. Test helpers

```go
// requireGolden compares view output against a golden file.
// Pass -update to refresh: go test -update ./cli/internal/tui/...
func requireGolden(t *testing.T, name string, actual string) {
    t.Helper()
    goldenPath := filepath.Join("testdata", name+".golden")

    if *update {
        os.MkdirAll("testdata", 0o755)
        os.WriteFile(goldenPath, []byte(actual), 0o644)
        return
    }

    expected, err := os.ReadFile(goldenPath)
    if err != nil {
        t.Fatalf("golden file %s not found (run with -update to create): %v", goldenPath, err)
    }

    if string(expected) != actual {
        t.Errorf("golden file mismatch: %s\n\nRun with -update to refresh.\n\nDiff:\n%s",
            goldenPath, diffStrings(string(expected), actual))
    }
}
```

## Test File Organization

```
cli/internal/tui/
  golden_test.go              # Golden file infrastructure (flag, helpers, diff)
  golden_fullapp_test.go      # Full-app snapshot tests
  golden_components_test.go   # Component-isolated snapshot tests
  testdata/
    fullapp-category-welcome.golden
    fullapp-items-skills.golden
    fullapp-detail-overview.golden
    fullapp-detail-files.golden
    fullapp-detail-install.golden
    fullapp-search-results.golden
    fullapp-modal.golden
    fullapp-settings.golden
    component-sidebar.golden
    component-items.golden
    component-detail-tabs.golden
    component-modal.golden
    component-help.golden
    component-filebrowser.golden
```

## Workflow After Implementation

### Making a TUI change:
1. Edit the TUI code
2. `make test` — golden file tests fail if any screen changed
3. `go test -update ./cli/internal/tui/...` — refresh golden files
4. `git diff cli/internal/tui/testdata/` — review visual changes
5. If the diff looks right → commit
6. If the diff shows a regression → fix the code

### Golden File Update Protocol (for AI-assisted development)

When Maive makes a TUI change during a development session, golden files MUST be updated as part of the same change — not deferred. The protocol:

1. **After any TUI code change**, run `go test -update ./cli/internal/tui/...` to regenerate affected golden files
2. **Review the diff** — `git diff cli/internal/tui/testdata/` shows what visually changed
3. **Stage golden files alongside code changes** — golden file updates are part of the same commit, not a separate "fix tests" commit
4. **If golden diff reveals an unintended regression**, fix the code before proceeding

**Why this matters:** If golden file updates get deferred, they pile up and lose their value as a review mechanism. The diff should always reflect the change you just made, not accumulated drift from multiple changes. Treat golden files like generated code — they stay in sync with source, always.

**For large refactors** (e.g., sidebar redesign, modal system overhaul):
- Run `-update` after the refactor is complete, not after each micro-step
- Review the full golden diff as a "before/after" of the visual change
- Consider this the "visual review" that replaces manual inspection
- If the diff is too large to review meaningfully, break the refactor into smaller commits

### Benefits:
- **No more manual visual inspection** for regression detection
- **Git diff IS the review** — see exactly what changed on each screen
- **CI catches regressions** — broken layout fails the build
- **Fast** — golden file tests run in milliseconds (no goroutines, no teatest)
- **Low maintenance** — ~14 files, only update when UI intentionally changes
- **Golden files always in sync** — updated as part of the change, not after

## Decisions Made

- **Hybrid approach:** Golden files for key screens + targeted assertions for logic
- **Both full-app and component snapshots:** Full-app catches integration issues, components catch isolated regressions with smaller blast radius
- **ANSI stripped output:** Golden files contain plain text for readable git diffs
- **~14 golden files total:** Enough to catch regressions, not so many they become noisy
- **No VHS/visual regression at this stage:** Can add later for screenshot-quality testing; golden files are sufficient for v1.0 velocity
