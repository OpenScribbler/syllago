# TUI Testing Setup Reference

Copy-pasteable test infrastructure for each build phase.

## testmain_test.go — Required Foundation

```go
package tui

import (
    "os"
    "testing"

    zone "github.com/lrstanley/bubblezone"
    "github.com/muesli/termenv"
    "github.com/charmbracelet/lipgloss"
)

func TestMain(m *testing.M) {
    // Deterministic output — REQUIRED for golden file stability
    lipgloss.SetColorProfile(termenv.Ascii)
    lipgloss.SetHasDarkBackground(true)
    os.Setenv("NO_COLOR", "1")
    os.Setenv("TERM", "dumb")

    // bubblezone global manager
    zone.NewGlobal()

    // Warmup render to stabilize AdaptiveColor state
    _ = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000", Dark: "#fff"}).Render("warmup")

    os.Exit(m.Run())
}
```

## testhelpers_test.go — Reusable Across All Phases

```go
package tui

import (
    "flag"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/x/ansi"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// --- Key helpers ---

func keyRune(r rune) tea.KeyMsg {
    return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func keyPress(k tea.KeyType) tea.KeyMsg {
    return tea.KeyMsg{Type: k}
}

var (
    keyUp    = keyPress(tea.KeyUp)
    keyDown  = keyPress(tea.KeyDown)
    keyLeft  = keyPress(tea.KeyLeft)
    keyRight = keyPress(tea.KeyRight)
    keyEnter = keyPress(tea.KeyEnter)
    keyEsc   = keyPress(tea.KeyEsc)
    keyTab   = keyPress(tea.KeyTab)
)

func pressN(m tea.Model, key tea.Msg, n int) tea.Model {
    for i := 0; i < n; i++ {
        m, _ = m.Update(key)
    }
    return m
}

// --- App construction ---

func testApp(t *testing.T) App {
    return testAppSize(t, 80, 30)
}

func testAppSize(t *testing.T, w, h int) App {
    t.Helper()
    cat := testCatalog(t)
    provs := testProviders()
    app := NewApp(cat, provs, testConfig())
    m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
    return m.(App)
}

// --- Golden file helpers ---

var tempDirRe = regexp.MustCompile(`/tmp/Test[A-Za-z0-9_]+/\d+`)

func normalizeSnapshot(s string) string {
    s = ansi.Strip(s)
    s = tempDirRe.ReplaceAllString(s, "<TESTDIR>")
    lines := strings.Split(s, "\n")
    for i, line := range lines {
        lines[i] = strings.TrimRight(line, " ")
    }
    return strings.Join(lines, "\n")
}

func snapshotApp(t *testing.T, app App) string {
    t.Helper()
    return normalizeSnapshot(app.View())
}

func requireGolden(t *testing.T, name string, got string) {
    t.Helper()
    path := filepath.Join("testdata", name+".golden")
    if *updateGolden {
        os.MkdirAll("testdata", 0o755)
        os.WriteFile(path, []byte(got), 0o644)
        return
    }
    want, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("golden file %s not found (run with -update-golden to create)", path)
    }
    if string(want) != got {
        t.Errorf("golden mismatch for %s:\n%s", name, diffStrings(string(want), got))
    }
}

func diffStrings(want, got string) string {
    wantLines := strings.Split(want, "\n")
    gotLines := strings.Split(got, "\n")
    var sb strings.Builder
    max := len(wantLines)
    if len(gotLines) > max {
        max = len(gotLines)
    }
    for i := 0; i < max; i++ {
        wl, gl := "", ""
        if i < len(wantLines) {
            wl = wantLines[i]
        }
        if i < len(gotLines) {
            gl = gotLines[i]
        }
        if wl != gl {
            sb.WriteString("--- want line " + string(rune('0'+i)) + ":\n")
            sb.WriteString("  " + wl + "\n")
            sb.WriteString("+++ got  line " + string(rune('0'+i)) + ":\n")
            sb.WriteString("  " + gl + "\n")
        }
    }
    if len(wantLines) != len(gotLines) {
        sb.WriteString("line count: want " + strings.Repeat(".", len(wantLines)) + " got " + strings.Repeat(".", len(gotLines)) + "\n")
    }
    return sb.String()
}

// --- Assertion helpers ---

func assertContains(t *testing.T, view, substr string) {
    t.Helper()
    stripped := ansi.Strip(view)
    if !strings.Contains(stripped, substr) {
        t.Errorf("view does not contain %q\n\nView:\n%s", substr, stripped)
    }
}

func assertNotContains(t *testing.T, view, substr string) {
    t.Helper()
    stripped := ansi.Strip(view)
    if strings.Contains(stripped, substr) {
        t.Errorf("view unexpectedly contains %q", substr)
    }
}
```

## Per-Phase Test Checklist

### Phase 1 (Shell + Styles)
- [ ] Golden: empty shell at 60x20, 80x30, 120x40
- [ ] Unit: WindowSizeMsg updates dimensions
- [ ] Unit: "too small" warning below 60x20
- [ ] Unit: helpbar renders version string

### Phase 2 (Topbar + Dropdown)
- [ ] Unit: dropdown open/close (1/2/3 keys, Enter, Esc)
- [ ] Unit: dropdown j/k navigation, wrapping
- [ ] Unit: dropdown Enter selects, fires ActiveMsg
- [ ] Unit: mutual exclusion (Content/Collection)
- [ ] Unit: click-to-open, click-item-to-select
- [ ] Golden: topbar at each size
- [ ] Golden: dropdown open state

### Phase 3 (Explorer + Items + Preview)
- [ ] Unit: item cursor movement (j/k, Home/End)
- [ ] Unit: focus switching (h/l between items and content)
- [ ] Unit: scroll indicators (N more above/below)
- [ ] Golden: explorer at each size with 5 items
- [ ] Golden: explorer with 85+ items (overflow)
- [ ] Golden: explorer with 0 items (empty state)

### Phase 4 (Metadata + Actions)
- [ ] Unit: metadata updates on item selection change
- [ ] Unit: action key dispatch (i/u/c/p)
- [ ] Unit: click action buttons
- [ ] Golden: metadata bar at each size

### Phase 5 (Split Content Zone)
- [ ] Unit: pane switching within content zone
- [ ] Unit: file tree navigation
- [ ] Unit: Hooks compat tab toggle
- [ ] Golden: split zone for Skills and Hooks

### Phase 6 (Gallery Grid)
- [ ] Unit: card grid navigation (arrows)
- [ ] Unit: contents sidebar updates on card selection
- [ ] Golden: gallery at each size for Library, Registries, Loadouts

### Phase 7 (Modals + Toasts + Search)
- [ ] Unit: modal lifecycle (open, button focus, confirm, cancel)
- [ ] Unit: wizard step transitions + validateStep()
- [ ] Unit: toast dismiss behavior (success vs error)
- [ ] Unit: search filter, match count, Esc cancel
- [ ] Integration (teatest): async install workflow
- [ ] Integration (teatest): toast auto-dismiss timing
- [ ] Golden: modal overlay at each size
