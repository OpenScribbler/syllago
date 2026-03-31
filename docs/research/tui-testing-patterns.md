# TUI Testing Patterns — How Go TUI Apps Test Their Code

Research into how real Go TUI applications (lazygit, k9s, superfile, pug, gh-dash) test their terminal UIs, plus the Bubble Tea testing ecosystem. Conducted 2026-03-24.

---

## The Testing Pyramid for TUI Apps

Three layers, from fastest to most realistic:

| Layer | What It Tests | Speed | Tool |
|-------|--------------|-------|------|
| **Unit tests** | State transitions, keyboard routing, command emission | ~ms | Direct `Update()` calls |
| **Golden file tests** | Visual layout regression, responsive breakpoints | ~ms | `View()` + file comparison |
| **Integration tests** | Full event loop, async operations, program lifecycle | ~seconds | `teatest` or custom harness |

**Key insight from lazygit and k9s:** Neither project tests full visual rendering in unit tests. Lazygit tests *behavior* (did the right lines appear in the right views?). K9s tests *data transformation* (did the renderer produce the right strings?). Full pixel/screen comparison is avoided because it's brittle and rarely catches bugs that behavioral tests miss.

---

## Layer 1: Unit Tests — Direct `Update()` Calls

The idiomatic, preferred approach. Since `Update()` is a pure function `(Model, Msg) -> (Model, Cmd)`, you call it like any other function and assert on the returned model state.

### Pattern: State Transition Testing

```go
func TestKeyboard_EscClosesDropdown(t *testing.T) {
    app := testApp(t)

    // Open dropdown
    m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
    a := m.(App)
    assert(t, a.topBar.menuOpen, "dropdown should be open")

    // Close with Esc
    m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
    a = m.(App)
    assert(t, !a.topBar.menuOpen, "Esc should close the dropdown")
}
```

### Pattern: Command Emission Testing

```go
func TestEnter_EmitsSelectCommand(t *testing.T) {
    app := testApp(t)
    m, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
    if cmd == nil {
        t.Fatal("Enter should produce a command")
    }
    msg := cmd() // execute the Cmd synchronously to get the tea.Msg
    sel, ok := msg.(topBarSelectMsg)
    if !ok {
        t.Fatalf("expected topBarSelectMsg, got %T", msg)
    }
}
```

### Pattern: Mouse Event Testing

```go
func TestMouseClick_SelectsItem(t *testing.T) {
    app := testApp(t)
    // Construct mouse event at known coordinates
    m, _ := app.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 15, Y: 8})
    a := m.(App)
    assert(t, a.items.cursor == 2, "click at Y=8 should select item 2")
}
```

For bubblezone-marked regions, you need to know the rendered coordinates. In unit tests, either:
- Calculate expected positions from known layout dimensions
- Use `zone.Get("itemName").InBounds(msg)` in the component, test the component's state after the click

### Pattern: Key Construction Helpers

```go
// Common key helpers for test readability
func keyRune(r rune) tea.KeyMsg {
    return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
func keyPress(k tea.KeyType) tea.KeyMsg {
    return tea.KeyMsg{Type: k}
}
func pressN(app tea.Model, key tea.Msg, n int) tea.Model {
    for i := 0; i < n; i++ {
        app, _ = app.Update(key)
    }
    return app
}
```

### What to Test at This Layer

- Keyboard routing: does key X in state Y produce state Z?
- Focus management: does Tab cycle focus correctly?
- Navigation: does Enter drill in, Esc go back?
- Dropdown open/close/select lifecycle
- Cursor movement and bounds (wrapping, clamping)
- State preservation across refreshes (the `rebuildItems()` pattern)
- Command emission for async operations

### What NOT to Test at This Layer

- Exact visual output (use golden files)
- Async operation completion (use teatest)
- Layout at different terminal sizes (use golden files at multiple sizes)

---

## Layer 2: Golden File Tests — Visual Regression

Compare `model.View()` output against stored reference files. This catches layout regressions, truncation bugs, responsive breakpoint issues, and style changes.

### The Canonical Implementation

```go
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func requireGolden(t *testing.T, name string, got string) {
    t.Helper()
    path := filepath.Join("testdata", name+".golden")

    // Normalize before comparison
    got = normalizeSnapshot(got)

    if *updateGolden {
        os.WriteFile(path, []byte(got), 0o644)
        return
    }

    want, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("golden file %s not found (run with -update-golden)", path)
    }
    if string(want) != got {
        t.Errorf("golden mismatch for %s:\n%s", name, diffStrings(string(want), got))
    }
}
```

### Critical: Deterministic Output Setup

This is the most important part. Without this, golden files diverge between local dev and CI:

```go
func init() {
    // Force ASCII color profile — no ANSI escape codes in output
    lipgloss.SetColorProfile(termenv.Ascii)

    // Prevent AdaptiveColor from mutating renderer state
    lipgloss.SetHasDarkBackground(true)

    // Disable bubblezone interference with string comparison
    zone.NewGlobal()

    // Suppress environment-dependent behavior
    os.Setenv("NO_COLOR", "1")
    os.Setenv("TERM", "dumb")
}
```

**Why each line matters:**
- `SetColorProfile(Ascii)` — prevents color escape codes that differ between terminals
- `SetHasDarkBackground(true)` — prevents `AdaptiveColor` from calling `HasDarkBackground()` which mutates global state and is non-deterministic on WSL
- `zone.NewGlobal()` — required before any `zone.Mark()` calls in `View()`
- `NO_COLOR`/`TERM` — prevents libraries from detecting terminal capabilities

### Normalization Before Comparison

```go
func normalizeSnapshot(s string) string {
    // Replace temp directory paths with stable placeholder
    s = tempDirRe.ReplaceAllString(s, "<TESTDIR>")

    // Trim trailing whitespace per line (padding produces invisible differences)
    lines := strings.Split(s, "\n")
    for i, line := range lines {
        lines[i] = strings.TrimRight(line, " ")
    }
    return strings.Join(lines, "\n")
}
```

### ANSI Codes: Strip or Preserve?

| Approach | Pros | Cons |
|----------|------|------|
| **Strip ANSI** (v1 approach) | Human-readable diffs, stable across terminals | Can't catch style regressions (wrong color) |
| **Preserve ANSI** (v2 approach) | Catches color/style bugs | Diffs are unreadable, fragile across environments |

**Recommendation:** Strip ANSI for golden files. Use separate style-specific unit tests to verify that the right styles are applied to the right elements. This gives the best of both: readable golden diffs for layout, targeted tests for styles.

```go
import "github.com/charmbracelet/x/ansi"

func snapshotApp(t *testing.T, app tea.Model) string {
    return normalizeSnapshot(ansi.Strip(app.(App).View()))
}
```

### Golden Test Matrix

Test each screen at multiple terminal sizes to catch responsive issues:

```go
func TestGolden_Explorer_120x40(t *testing.T) {
    app := testAppSize(t, 120, 40)
    navigateTo(app, "skills")
    requireGolden(t, "explorer-skills-120x40", snapshotApp(t, app))
}

func TestGolden_Explorer_80x30(t *testing.T) {
    app := testAppSize(t, 80, 30)
    navigateTo(app, "skills")
    requireGolden(t, "explorer-skills-80x30", snapshotApp(t, app))
}

func TestGolden_Explorer_60x20(t *testing.T) {
    app := testAppSize(t, 60, 20)
    navigateTo(app, "skills")
    requireGolden(t, "explorer-skills-60x20", snapshotApp(t, app))
}
```

### Update Workflow

```bash
# Regenerate all golden files after intentional visual changes
cd cli && go test ./internal/tui/ -update-golden

# Review every change before committing
git diff testdata/

# Run tests to verify goldens match
cd cli && go test ./internal/tui/
```

### What to Test at This Layer

- Layout structure at each breakpoint (60x20, 80x30, 120x40)
- Empty states (no items, no registries)
- Overflow states (85+ items, long names, long descriptions)
- Each major screen (explorer, gallery, modal open, toast visible)
- Dropdown open state
- Search active state

---

## Layer 3: Integration Tests — `teatest`

For testing behavior that depends on the actual event loop: async operations, tick-based updates, program lifecycle.

### `teatest` API

```go
import "github.com/charmbracelet/x/exp/teatest"

func TestInstallWorkflow(t *testing.T) {
    app := NewApp(catalog, providers, config)

    tm := teatest.NewTestModel(t, app,
        teatest.WithInitialTermSize(120, 40),
    )

    // Navigate to an item
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Wait for detail view to render
    teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
        return bytes.Contains(bts, []byte("alpha-skill"))
    }, teatest.WithDuration(3*time.Second))

    // Trigger install
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

    // Wait for install modal
    teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
        return bytes.Contains(bts, []byte("Install"))
    })

    // Confirm
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

    // Wait for success toast
    teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
        return bytes.Contains(bts, []byte("Done:"))
    })

    // Quit and check final state
    tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
    fm := tm.FinalModel(t)
    m := fm.(App)
    // Assert install was recorded in model state
}
```

### When to Use `teatest` vs Direct `Update()`

| Scenario | Use |
|----------|-----|
| Key X opens menu Y | Direct `Update()` |
| Dropdown navigation and selection | Direct `Update()` |
| Focus cycling | Direct `Update()` |
| Install workflow (async operation) | `teatest` |
| Registry sync (HTTP + git) | `teatest` with mocked backend |
| Toast auto-dismiss (timer-based) | `teatest` with `WaitFor` |
| Program exits cleanly on Ctrl+C | `teatest.WaitFinished()` |

### Limitation

`teatest` runs the full program in a goroutine. You can't inspect intermediate model state between keystrokes — you can only observe output. For most TUI testing, direct `Update()` is better because you get full model access.

---

## How the Big Projects Test

### Lazygit: Code-Defined E2E Tests (Generation 3)

After failed attempts with manual testing (Gen 1) and recorded session replay (Gen 2), lazygit settled on **code-defined integration tests** that drive a real lazygit process:

```go
var Commit = NewIntegrationTest(NewIntegrationTestArgs{
    Description: "Staging files and committing",
    SetupRepo: func(shell *Shell) {
        shell.CreateFile("myfile", "content")
    },
    Run: func(t *TestDriver, keys config.KeybindingConfig) {
        t.Views().Files().
            IsFocused().
            Lines(
                Equals("▼ /").IsSelected(),
                Equals(" ?? myfile"),
            ).
            Press(keys.Files.CommitChanges)

        t.ExpectPopup().CommitMessagePanel().
            Type("my commit").Confirm()

        t.Views().Commits().Focus().Lines(
            Contains("my commit").IsSelected(),
        )
    },
})
```

**Key architecture:**
- Uses `tcell.NewSimulationScreen()` — in-memory screen, no real terminal
- `LAZYGIT_HEADLESS=true` env var triggers simulation mode
- Injects `tcell.EventKey` events into the simulation screen
- Reads back `view.BufferLines()` for assertions (behavioral, not pixel)
- "Lazygit is no longer busy" signal for synchronization (replaces polling)
- Sandbox mode: `SetupRepo` runs, then hands control to human for manual exploration
- 120+ tests, runs in parallel in CI with 2 retries

**What lazygit does NOT do:** Golden file / pixel snapshot comparison. All assertions are behavioral (lines contain expected text, selection state is correct).

### K9s: Layer-Isolated Testing

K9s tests each architectural layer independently:

1. **Render tests** (`internal/render/`): Pure data transformation. Load YAML fixture, call `renderer.Render()`, assert `row.Fields` matches expected strings. No terminal involved.

2. **View component tests** (`internal/view/`): Test initialization and configuration. `assert.Equal(t, "Pods", po.Name())`. No screen rendering.

3. **Test escape hatches** on real structs: `Prompt.SendKey(ev)` marked "testing only!", `Flash.SetTestMode()`. These bypass the event loop for direct state injection.

4. **Integration tests** (`*_int_test.go`): Require real Kubernetes API, separated for selective CI runs.

**What k9s does NOT do:** Full TUI rendering tests, event loop simulation, or golden file comparisons.

### Superfile: Python E2E Suite

Superfile uses a **Python-based integration test suite** (`testsuite/main.py`) that runs functional scenarios. Go unit tests focus on file operation logic, not visual rendering.

### Pug, gh-dash: Minimal TUI Testing

Pug tests utility functions (time formatting, sanitization) and table model state (selection, toggle). The actual TUI components (`pane_manager.go`, `model.go`) have almost no tests. gh-dash has no visible TUI test infrastructure.

**This is the norm.** Most Bubble Tea apps rely heavily on manual testing for the visual layer. The projects with good test coverage (lazygit, k9s) invested significant engineering in custom test infrastructure.

---

## Syllago's Existing Test Foundation

### What v1 Had (Worth Preserving)

The `tui_v1/` package had a comprehensive test setup:

- **`testhelpers_test.go`**: `init()` with `NO_COLOR=1`, `TERM=dumb`, `termenv.Ascii` profile, `SetHasDarkBackground(true)`, `zone.NewGlobal()` — all determinism fixes
- **`testCatalog(t)`**: Full catalog with real filesystem fixtures via `t.TempDir()`
- **`testCatalogLarge(t)`**: 85+ items for overflow/scroll testing
- **`testCatalogEmpty(t)`**: Empty state testing
- **`testAppSize(t, w, h)`**: Builds app AND manually propagates sub-model dimensions
- **`golden_test.go`**: `-update-golden` flag, `requireGolden()`, `normalizeSnapshot()`, `diffStrings()`
- **15 golden tests** covering every screen at multiple sizes
- **Overflow golden tests** with large datasets

### What v2 Currently Has (Gaps)

- `testmain_test.go`: Only `zone.NewGlobal()` — missing `SetColorProfile`, `NO_COLOR`, `TERM`
- `app_test.go`: Good behavioral tests but thin catalog (2 items, 1 provider)
- `capture_test.go`: Dev debug tool, not a real test
- Golden files exist in `testdata/` but have no test that reads them
- Golden files store raw ANSI (fragile) — v1 stripped ANSI (stable)
- No large/empty catalog variants
- No overflow/boundary testing

---

## Recommended Testing Strategy for Syllago TUI v3

### Per-Phase Testing Requirements

Each build phase should include its own tests before moving to the next phase:

**Phase 1 (Shell + Styles):**
- `testmain_test.go` with full determinism setup (v1 pattern)
- `testhelpers_test.go` with `testApp()`, `testAppSize()`, key helpers
- Golden tests: empty shell at 60x20, 80x30, 120x40
- Unit tests: `WindowSizeMsg` handling, "too small" warning

**Phase 2 (Topbar + Dropdown):**
- Unit tests: dropdown open/close, keyboard navigation (j/k/Enter/Esc), mutual exclusion, wrapping
- Mouse tests: click to open, click item to select, click-away to close
- Golden tests: topbar at each size, dropdown open state
- Component test: dropdown in isolation (not just via app)

**Phase 3 (Explorer + Items + Preview):**
- Unit tests: item selection, cursor movement, scroll, focus switching (h/l)
- Golden tests: explorer at each size, with 5 items, with 85+ items, with 0 items
- `testCatalogLarge(t)` and `testCatalogEmpty(t)` catalogs

**Phase 4 (Metadata + Actions):**
- Unit tests: action key dispatch (i/u/c/p), metadata updates on item change
- Golden tests: metadata bar at each size, with/without item selected
- Mouse tests: click action buttons

**Phase 5 (Split Content Zone):**
- Unit tests: pane switching, file tree navigation, preview scroll
- Golden tests: split zone at each size for Skills and Hooks

**Phase 6 (Gallery Grid):**
- Unit tests: card grid navigation, contents sidebar updates
- Golden tests: gallery at each size for Library, Registries, Loadouts

**Phase 7 (Modals + Toasts + Search):**
- Unit tests: modal lifecycle, button focus, wizard step transitions, toast dismiss
- `teatest` integration tests: async install workflow, toast auto-dismiss
- Golden tests: modal overlay at each size
- Wizard invariant tests (existing `validateStep()` pattern)

### Test File Structure

```
cli/internal/tui/
  testmain_test.go      — init() determinism, zone setup
  testhelpers_test.go   — testApp, testCatalog, key helpers, golden helpers
  app_test.go           — shell-level unit tests
  topbar_test.go        — dropdown unit + golden tests
  dropdown_test.go      — dropdown component in isolation
  explorer_test.go      — explorer layout unit + golden tests
  items_test.go         — items list unit tests
  preview_test.go       — preview pane unit tests
  metadata_test.go      — metadata bar unit + golden tests
  gallery_test.go       — gallery grid unit + golden tests
  modal_test.go         — modal unit + golden tests
  toast_test.go         — toast unit tests
  search_test.go        — search unit tests
  integration_test.go   — teatest-based E2E tests (Phase 7+)
  testdata/
    shell-120x40.golden
    shell-80x30.golden
    shell-60x20.golden
    topbar-dropdown-open.golden
    explorer-skills-120x40.golden
    explorer-skills-80x30.golden
    explorer-empty-80x30.golden
    explorer-overflow-80x30.golden
    gallery-loadouts-120x40.golden
    modal-confirm-80x30.golden
    ...
```

### Golden File Naming Convention

```
{component}-{variant}-{width}x{height}.golden

Examples:
  shell-empty-80x30.golden
  topbar-content-open-120x40.golden
  explorer-skills-120x40.golden
  explorer-skills-overflow-80x30.golden
  gallery-loadouts-80x30.golden
  modal-confirm-80x30.golden
```

---

## Sources

### Articles
- [Lessons Learned Revamping Lazygit's Integration Tests](https://jesseduffield.com/IntegrationTests/) — Jesse Duffield
- [More Lazygit Integration Testing](https://jesseduffield.com/More-Lazygit-Integration-Testing/) — Jesse Duffield
- [Writing Bubble Tea Tests](https://charm.land/blog/teatest/) — Charm blog
- [Writing Bubble Tea Tests](https://carlosbecker.com/posts/teatest/) — Carlos Becker
- [Testing Bubble Tea Interfaces](https://patternmatched.substack.com/p/testing-bubble-tea-interfaces) — Pattern Matched
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — leg100
- [Testing Terminal User Interface Apps](https://blog.waleedkhan.name/testing-tui-apps/) — Waleed Khan

### Libraries
- [teatest](https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest) — Charm's official test harness
- [catwalk](https://github.com/knz/catwalk) — Data-driven TUI testing
- [tcell SimulationScreen](https://pkg.go.dev/github.com/gdamore/tcell) — In-memory terminal

### Discussions
- [Improvements to teatest & golden (charmbracelet/x #533)](https://github.com/charmbracelet/x/discussions/533)
- [CLI/TUI Testing Package (bubbletea #1528)](https://github.com/charmbracelet/bubbletea/discussions/1528)
- [lazygit pkg/integration README](https://github.com/jesseduffield/lazygit/blob/master/pkg/integration/README.md)
