---
paths:
  - "cli/internal/tui/**"
---

# TUI Testing Patterns

## 1. After Any Visual Change

```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/   # review EVERY change
```

Golden files are ground truth. Never update them to match broken code.

## 2. Test at Multiple Sizes

Every visual component must be tested at 60x20, 80x30, and 120x40.

## 3. Golden File Naming

`{component}-{variant}-{width}x{height}.golden`

## 4. Test Helpers (testhelpers_test.go)

- `testApp(t)` — empty catalog, 80x30
- `testAppSize(t, w, h)` — custom dimensions
- `keyRune(r)` — simulate a rune keypress
- `keyPress(k)` — simulate a special key (Enter, Esc, etc.)
- `pressN(m, key, n)` — press a key N times
- `assertContains(t, view, substr)` — assert substring in view output
- `assertNotContains(t, view, substr)` — assert substring NOT in view
- `requireGolden(t, name, snapshot)` — compare against golden file
- `snapshotApp(t, app)` — capture app view for golden comparison

## 5. Deterministic Output (testmain_test.go)

Already configured — do not modify without understanding:
- `lipgloss.SetColorProfile(termenv.Ascii)` — no color codes in goldens
- `lipgloss.SetHasDarkBackground(true)` — consistent AdaptiveColor
- Warmup render in `init()` — prevents AdaptiveColor race condition

The warmup render fix was added to resolve a flaky `TestGoldenFullApp_Modal`
test. `lipgloss.AdaptiveColor` triggers `HasDarkBackground()` which mutates
renderer state. The warmup in test `init()` ensures this happens before any
parallel test goroutines start.
