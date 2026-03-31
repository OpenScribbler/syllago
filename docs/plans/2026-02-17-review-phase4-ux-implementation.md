# Phase 4: TUI UX Improvements - Implementation Plan

**Date:** 2026-02-17
**Phase:** 4 of 6
**Items:** 4.1 - 4.20 (20 tasks)
**Focus:** BubbleTea TUI usability and polish

This plan follows strict TDD rhythm: Write failing test, verify failure, implement, verify pass, commit.

---

## Dependency Analysis

Several items have dependencies or natural groupings:

1. **4.3 (key conflict)** must precede **4.5 (help overlay)** -- the help overlay should show corrected bindings
2. **4.6 (Shift+Tab)** is independent, trivial, do first as warmup
3. **4.2 (live search)** and **4.4 (search position)** are tightly coupled -- both modify search behavior
4. **4.1 (spinners)** is foundational -- adds the spinner dependency used nowhere else yet
5. **4.10 (state preservation)** depends on understanding the detail model lifecycle from 4.1
6. **4.15 (scroll counts)** and **4.16 (detail breadcrumb position)** are related UI polish
7. **4.17 (next/prev navigation)** depends on **4.16** (passing cursor/count to detail)

## Execution Order (7 Batches)

| Batch | Items | Theme | Commit(s) |
|-------|-------|-------|-----------|
| A | 4.6, 4.3 | Key binding fixes | 2 commits |
| B | 4.2, 4.4 | Live search + position fix | 1 commit |
| C | 4.5 | Help overlay | 1 commit |
| D | 4.1 | Spinner/loading indicators | 1 commit |
| E | 4.7, 4.9, 4.11, 4.12, 4.13 | Small UX polish batch | 1-2 commits |
| F | 4.8, 4.10, 4.14 | State/guidance improvements | 1-2 commits |
| G | 4.15, 4.16, 4.17, 4.18, 4.19, 4.20 | Enhancement batch | 2-3 commits |

---

## Batch A: Key Binding Fixes

### 4.6: Add Shift+Tab for reverse tab cycling

**Severity:** MEDIUM
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/keys.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_test.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/testhelpers_test.go`

**What changes:**

1. Add `ShiftTab` binding to `keyMap` in `keys.go`:
```go
ShiftTab key.Binding
```
And in `keys`:
```go
ShiftTab: key.NewBinding(
    key.WithKeys("shift+tab"),
    key.WithHelp("shift+tab", "prev tab"),
),
```

2. Add `keyShiftTab` helper in `testhelpers_test.go`:
```go
keyShiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
```

3. In `detail.go`, in the tab switching block (line ~322), add a `shift+tab` case:
```go
case "shift+tab":
    m.activeTab = (m.activeTab + 2) % 3
    m.scrollOffset = 0
    return m, nil
```

**Why `(m.activeTab + 2) % 3`:** Going backward by 1 in modular arithmetic of 3 is the same as going forward by 2. `(0+2)%3 = 2`, `(1+2)%3 = 0`, `(2+2)%3 = 1` -- correct reverse cycle.

**Test strategy:**

Add to `detail_test.go`:
```go
func TestDetailShiftTabReverseCycle(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)

    // Start at Overview (0)
    if app.detail.activeTab != tabOverview {
        t.Fatalf("expected tabOverview, got %d", app.detail.activeTab)
    }

    // Shift+Tab: Overview -> Install (wraps backward)
    m, _ := app.Update(keyShiftTab)
    app = m.(App)
    if app.detail.activeTab != tabInstall {
        t.Fatalf("expected tabInstall after shift+tab from Overview, got %d", app.detail.activeTab)
    }

    // Shift+Tab: Install -> Files
    m, _ = app.Update(keyShiftTab)
    app = m.(App)
    if app.detail.activeTab != tabFiles {
        t.Fatalf("expected tabFiles after shift+tab from Install, got %d", app.detail.activeTab)
    }

    // Shift+Tab: Files -> Overview
    m, _ = app.Update(keyShiftTab)
    app = m.(App)
    if app.detail.activeTab != tabOverview {
        t.Fatalf("expected tabOverview after shift+tab from Files, got %d", app.detail.activeTab)
    }
}

func TestDetailShiftTabBlockedDuringAction(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)
    app.detail.confirmAction = actionUninstall

    m, _ := app.Update(keyShiftTab)
    app = m.(App)
    if app.detail.activeTab != tabOverview {
        t.Fatal("shift+tab should be blocked during active action")
    }
}
```

**Commit message:** `feat(tui): add Shift+Tab for reverse tab cycling in detail view (4.6)`

---

### 4.3: Resolve `c` key binding conflict (copy vs confirm)

**Severity:** HIGH
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/filebrowser.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/filebrowser_test.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_test.go`

**What changes:**

The file browser uses `msg.String() == "c"` for "confirm selection" (line 191 of filebrowser.go). The detail view uses `key.Matches(msg, keys.Copy)` for "copy to clipboard." When a user is in the file browser (during import), pressing `c` triggers confirm. When in detail, `c` copies. The conflict is that both use the same key for different semantic actions.

**Fix:** Change file browser confirm from `c` to `d` (for "done"). Also migrate raw `msg.String()` checks to `key.Matches` for consistency.

1. In `filebrowser.go` line 191, change:
```go
// Before:
case msg.String() == "c":
// After:
case msg.String() == "d":
```

2. In `filebrowser.go` View() line 300, update the help text:
```go
// Before:
"up/down navigate . enter open dir . space select . a select all . c confirm . esc parent dir"
// After:
"up/down navigate . enter open dir . space select . a select all . d done . esc parent dir"
```

3. Also fix the `msg.String() == "a"` on line 183 to use `key.Matches` for consistency. Add file-browser-specific bindings:

```go
// In filebrowser.go, add local bindings:
var fbKeys = struct {
    SelectAll key.Binding
    Done      key.Binding
}{
    SelectAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
    Done:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
}
```

Then replace `msg.String() == "a"` with `key.Matches(msg, fbKeys.SelectAll)` and `msg.String() == "c"` with `key.Matches(msg, fbKeys.Done)`.

**Test strategy:**

Add to `filebrowser_test.go`:
```go
func TestFileBrowserDKeyConfirms(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

    fb := newFileBrowser(tmp, catalog.Skills)
    fb.width = 80
    fb.height = 30

    // Select file.txt (Down to it, Space to select)
    fb, _ = fb.Update(keyDown)
    fb, _ = fb.Update(keySpace)

    if fb.SelectionCount() != 1 {
        t.Fatalf("expected 1 selected, got %d", fb.SelectionCount())
    }

    // 'd' confirms
    fb, cmd := fb.Update(keyRune('d'))
    if cmd == nil {
        t.Fatal("expected fileBrowserDoneMsg command from 'd'")
    }
}

func TestFileBrowserCKeyDoesNotConfirm(t *testing.T) {
    tmp := t.TempDir()
    os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0644)

    fb := newFileBrowser(tmp, catalog.Skills)
    fb.width = 80
    fb.height = 30

    fb, _ = fb.Update(keyDown)
    fb, _ = fb.Update(keySpace)

    // 'c' should NOT confirm anymore
    fb, cmd := fb.Update(keyRune('c'))
    if cmd != nil {
        t.Fatal("'c' should NOT trigger confirm in file browser")
    }
}
```

Also verify `c` still works in detail view (existing `TestDetailPromptCopy` covers this).

**Commit message:** `fix(tui): resolve c key conflict by changing file browser confirm to d (4.3)`

---

## Batch B: Live Search + Position Fix

### 4.2: Implement live-filtering search (filter as you type)

**Severity:** HIGH
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/search.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/search_test.go`

**What changes:**

Currently, search requires pressing Enter to apply the filter. The goal is to filter the list on every keystroke while search is active.

**Approach:** When search is active and we're on `screenItems`, update `a.items` on every keystroke (not just on Enter). On `screenCategory`, we need to decide: do we show a preview count, or do we navigate to a temporary filtered view? The simplest approach: live-filtering only works on the items screen. On category, search still transitions on Enter (but shows a match count in the search bar).

Key changes to `app.go` in the search-active block:

1. After `a.search, cmd = a.search.Update(msg)`, add live filtering logic:
```go
// Live-filter items while typing
if a.screen == screenItems {
    query := a.search.query()
    ct := a.items.contentType
    var source []catalog.ContentItem
    if ct == catalog.SearchResults {
        source = a.catalog.Items
    } else if ct == catalog.MyTools {
        for _, item := range a.catalog.Items {
            if item.Local {
                source = append(source, item)
            }
        }
    } else {
        source = a.catalog.ByType(ct)
    }
    filtered := filterItems(source, query)
    items := newItemsModel(ct, filtered, a.providers, a.catalog.RepoRoot)
    items.width = a.width
    items.height = a.height
    a.items = items
}
```

2. When search is active on category screen and user types, show a match count in the search view. Modify `searchModel` to accept an optional match count:
```go
type searchModel struct {
    input      textinput.Model
    active     bool
    matchCount int  // -1 = not showing count
}
```

In `search.View()`:
```go
func (m searchModel) View() string {
    if !m.active {
        return ""
    }
    v := m.input.View()
    if m.matchCount >= 0 {
        v += " " + helpStyle.Render(fmt.Sprintf("(%d matches)", m.matchCount))
    }
    return v
}
```

Set match count when on category screen during live typing:
```go
if a.screen == screenCategory {
    query := a.search.query()
    if query != "" {
        a.search.matchCount = len(filterItems(a.catalog.Items, query))
    } else {
        a.search.matchCount = -1
    }
}
```

3. Enter still applies the filter and deactivates search (no change needed to Enter logic). Esc still cancels and resets.

**Test strategy:**

Add to `search_test.go`:
```go
func TestSearchLiveFilterItems(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyEnter) // -> items (Skills)
    app = m.(App)
    origCount := len(app.items.items)

    // Activate search
    m, _ = app.Update(keyRune('/'))
    app = m.(App)

    // Type "alpha" -- should filter live
    for _, r := range "alpha" {
        m, _ = app.Update(keyRune(r))
        app = m.(App)
    }

    // Items should be filtered without pressing Enter
    if len(app.items.items) >= origCount && origCount > 1 {
        t.Fatalf("expected fewer items during live search, got %d (was %d)", len(app.items.items), origCount)
    }
}

func TestSearchLiveFilterCategoryShowsCount(t *testing.T) {
    app := testApp(t)
    assertScreen(t, app, screenCategory)

    m, _ := app.Update(keyRune('/'))
    app = m.(App)

    for _, r := range "skill" {
        m, _ = app.Update(keyRune(r))
        app = m.(App)
    }

    // Match count should be visible
    view := app.View()
    assertContains(t, view, "matches)")
}
```

### 4.4: Move search overlay to a fixed visible position

**Severity:** HIGH
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`

**What changes:**

Currently, search is appended at the bottom of content (line 411 in app.go: `content += "\n" + a.search.View()`). This can be off-screen if content is long.

**Fix:** Replace the help bar with the search input when search is active. The help bar is the last line of each screen's `View()` output. Instead of appending search, we should replace the help bar.

Approach: In `App.View()`, when search is active, trim the last line (help bar) from content and replace it with the search input:

```go
// Overlay search if active (replaces help bar)
if a.search.active {
    lines := strings.Split(content, "\n")
    // Remove trailing empty lines and the help bar
    for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
        lines = lines[:len(lines)-1]
    }
    if len(lines) > 0 {
        lines = lines[:len(lines)-1] // remove help bar
    }
    content = strings.Join(lines, "\n")
    content += "\n" + a.search.View()
}
```

**Test strategy:**

The existing `TestSearchHasLabel` test verifies search is visible. Add:
```go
func TestSearchReplacesHelpBar(t *testing.T) {
    app := testApp(t)

    // Before search: help bar is visible
    view := app.View()
    assertContains(t, view, "/ search")

    // Activate search
    m, _ := app.Update(keyRune('/'))
    app = m.(App)

    view = app.View()
    assertContains(t, view, "Search:")
    // Help bar should be replaced, not both showing
    assertNotContains(t, view, "/ search")
}
```

**Commit message:** `feat(tui): implement live-filtering search and fix search overlay position (4.2, 4.4)`

---

## Batch C: Help Overlay

### 4.5: Add `?` help overlay with all keyboard shortcuts

**Severity:** MEDIUM
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/help_overlay.go` (new file)
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/keys.go`

**What changes:**

1. Add `Help` binding to `keyMap` in `keys.go`:
```go
Help: key.NewBinding(
    key.WithKeys("?"),
    key.WithHelp("?", "help"),
),
```

2. Create `help_overlay.go` with a `helpOverlayModel`:
```go
package tui

import "strings"

type helpOverlayModel struct {
    active bool
    screen screen
}

func (m helpOverlayModel) View(s screen, width, height int) string {
    if !m.active {
        return ""
    }

    var lines []string
    lines = append(lines, titleStyle.Render("Keyboard Shortcuts"))
    lines = append(lines, "")

    // Global shortcuts
    lines = append(lines, labelStyle.Render("Global:"))
    lines = append(lines, "  "+helpStyle.Render("?         help (this overlay)"))
    lines = append(lines, "  "+helpStyle.Render("ctrl+c    quit"))
    lines = append(lines, "  "+helpStyle.Render("/         search"))
    lines = append(lines, "  "+helpStyle.Render("esc       back / cancel"))
    lines = append(lines, "")

    // Context-sensitive shortcuts
    switch s {
    case screenCategory:
        lines = append(lines, labelStyle.Render("Category Screen:"))
        lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
        lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
        lines = append(lines, "  "+helpStyle.Render("enter     select"))
        lines = append(lines, "  "+helpStyle.Render("q         quit"))

    case screenItems:
        lines = append(lines, labelStyle.Render("Items Screen:"))
        lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
        lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
        lines = append(lines, "  "+helpStyle.Render("enter     view details"))
        lines = append(lines, "  "+helpStyle.Render("home/g    jump to top"))
        lines = append(lines, "  "+helpStyle.Render("end/G     jump to bottom"))

    case screenDetail:
        lines = append(lines, labelStyle.Render("Detail Screen:"))
        lines = append(lines, "  "+helpStyle.Render("tab       next tab"))
        lines = append(lines, "  "+helpStyle.Render("shift+tab previous tab"))
        lines = append(lines, "  "+helpStyle.Render("1/2/3     jump to tab"))
        lines = append(lines, "  "+helpStyle.Render("up/down   scroll / navigate"))
        lines = append(lines, "  "+helpStyle.Render("i         install"))
        lines = append(lines, "  "+helpStyle.Render("u         uninstall"))
        lines = append(lines, "  "+helpStyle.Render("c         copy prompt"))
        lines = append(lines, "  "+helpStyle.Render("s         save prompt"))
        lines = append(lines, "  "+helpStyle.Render("e         env var setup (MCP)"))
        lines = append(lines, "  "+helpStyle.Render("p         promote (local)"))

    // Import/Settings/Update omitted for brevity -- add as appropriate
    }

    lines = append(lines, "")
    lines = append(lines, helpStyle.Render("Press ? or esc to close"))

    return strings.Join(lines, "\n")
}
```

3. Add `helpOverlay helpOverlayModel` field to `App` struct.

4. In `App.Update`, handle `?` toggle before screen-specific handling:
```go
// Help overlay toggle (skip when search active or text input active)
if key.Matches(msg, keys.Help) && !a.search.active {
    if a.helpOverlay.active {
        a.helpOverlay.active = false
        return a, nil
    }
    // Don't activate during text input
    if a.screen == screenDetail && a.detail.HasTextInput() {
        break
    }
    a.helpOverlay.active = true
    a.helpOverlay.screen = a.screen
    return a, nil
}

// If help overlay is active, esc closes it
if a.helpOverlay.active {
    if msg.Type == tea.KeyEsc {
        a.helpOverlay.active = false
        return a, nil
    }
    return a, nil // swallow all other keys while overlay is shown
}
```

5. In `App.View()`, when overlay is active, replace content:
```go
if a.helpOverlay.active {
    content = a.helpOverlay.View(a.screen, a.width, a.height)
}
```

**Test strategy:**

```go
func TestHelpOverlayToggle(t *testing.T) {
    app := testApp(t)

    // ? activates overlay
    m, _ := app.Update(keyRune('?'))
    app = m.(App)
    if !app.helpOverlay.active {
        t.Fatal("expected help overlay active after ?")
    }

    view := app.View()
    assertContains(t, view, "Keyboard Shortcuts")

    // ? again deactivates
    m, _ = app.Update(keyRune('?'))
    app = m.(App)
    if app.helpOverlay.active {
        t.Fatal("expected help overlay inactive after second ?")
    }
}

func TestHelpOverlayEscCloses(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyRune('?'))
    app = m.(App)

    m, _ = app.Update(keyEsc)
    app = m.(App)
    if app.helpOverlay.active {
        t.Fatal("expected help overlay closed after esc")
    }
}

func TestHelpOverlaySwallowsKeys(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyRune('?'))
    app = m.(App)

    // Down key should not change category cursor
    origCursor := app.category.cursor
    m, _ = app.Update(keyDown)
    app = m.(App)
    if app.category.cursor != origCursor {
        t.Fatal("keys should be swallowed while help overlay is active")
    }
}

func TestHelpOverlayContextSensitive(t *testing.T) {
    // Category context
    app := testApp(t)
    m, _ := app.Update(keyRune('?'))
    app = m.(App)
    view := app.View()
    assertContains(t, view, "Category Screen")

    // Close and navigate to detail
    m, _ = app.Update(keyEsc)
    app = m.(App)
    m, _ = app.Update(keyEnter) // items
    app = m.(App)
    m, _ = app.Update(keyEnter) // detail
    app = m.(App)

    m, _ = app.Update(keyRune('?'))
    app = m.(App)
    view = app.View()
    assertContains(t, view, "Detail Screen")
}
```

**Commit message:** `feat(tui): add ? help overlay with context-sensitive keyboard shortcuts (4.5)`

---

## Batch D: Spinner/Loading Indicators

### 4.1: Add spinner/loading indicators for blocking operations

**Severity:** HIGH
**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/update.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/import.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/update_test.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/import_test.go`

**What changes:**

Three blocking operations currently show no progress:
1. **Update screen**: `stepUpdatePull` shows static "Updating syllago..." text, and `fetchPreview()` shows nothing while loading
2. **Import screen**: `startClone()` shows static "Cloning repository..." message
3. **Detail model**: `newDetailModel()` runs `glamour.Render()` and `installer.ParseMCPConfig()` synchronously in the constructor

**Approach for update and import (spinner):**

Add `spinner.Model` to `updateModel` and `importModel`. The spinner needs its own `Init()` -> `tick` message cycle, but since these are sub-models, we'll start the spinner tick via a `tea.Cmd`.

For `updateModel`:
```go
import "github.com/charmbracelet/bubbles/spinner"

type updateModel struct {
    // ... existing fields ...
    spinner   spinner.Model
    loading   bool  // true while an async operation is in progress
}

func newUpdateModel(...) updateModel {
    sp := spinner.New()
    sp.Spinner = spinner.Dot
    sp.Style = lipgloss.NewStyle().Foreground(primaryColor)
    return updateModel{
        // ... existing ...
        spinner: sp,
    }
}
```

In `Update`, handle `spinner.TickMsg`:
```go
case spinner.TickMsg:
    if m.loading {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
```

When starting async operations (`fetchPreview`, `startPull`), set `m.loading = true` and return `tea.Batch(asyncCmd, m.spinner.Tick)`. When the result arrives, set `m.loading = false`.

In `View`, show spinner:
```go
case stepUpdatePull:
    s += "\n" + m.spinner.View() + " " + helpStyle.Render("Updating syllago...") + "\n"
```

The same pattern for `importModel` during `startClone`:
```go
// In updateGitURL, when starting clone:
m.message = ""
m.loading = true
return m, tea.Batch(m.startClone(url), m.spinner.Tick)
```

**Note:** The spinner's `TickMsg` must be routed through `App.Update`. Add a case in `app.go`:
```go
case spinner.TickMsg:
    switch a.screen {
    case screenUpdate:
        var cmd tea.Cmd
        a.updater, cmd = a.updater.Update(msg)
        return a, cmd
    case screenImport:
        var cmd tea.Cmd
        a.importer, cmd = a.importer.Update(msg)
        return a, cmd
    }
```

**Approach for detail model (async init):**

The `newDetailModel` constructor does synchronous work (glamour render, MCP parse, file reads, installer.CheckStatus). Move heavy work into a `tea.Cmd` that returns a `detailReadyMsg`:

```go
type detailReadyMsg struct {
    renderedBody   string
    renderedReadme string
    llmPrompt      string
    mcpConfig      *installer.MCPConfig
    providerChecks []bool
}
```

Change `newDetailModel` to do minimal setup, then return a command:
```go
func newDetailModel(item catalog.ContentItem, providers []provider.Provider, repoRoot string) (detailModel, tea.Cmd) {
    // ... minimal field setup ...
    m.loading = true
    return m, m.loadContent()
}

func (m detailModel) loadContent() tea.Cmd {
    item := m.item
    providers := m.providers
    repoRoot := m.repoRoot
    return func() tea.Msg {
        // Do all the heavy work here
        var msg detailReadyMsg
        if item.Type == catalog.MCP {
            msg.mcpConfig, _ = installer.ParseMCPConfig(item.Path)
        }
        // ... glamour render, etc ...
        return msg
    }
}
```

Handle `detailReadyMsg` in `detail.Update` to populate fields and set `loading = false`.

In app.go, update the detail entry to capture the command:
```go
// Before:
a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
// After:
var cmd tea.Cmd
a.detail, cmd = newDetailModel(item, a.providers, a.catalog.RepoRoot)
a.screen = screenDetail
return a, cmd
```

Route `detailReadyMsg` through app:
```go
case detailReadyMsg:
    if a.screen == screenDetail {
        var cmd tea.Cmd
        a.detail, cmd = a.detail.Update(msg)
        return a, cmd
    }
```

**Gotcha:** The detail model's `loadContent` closure runs on a goroutine. It must not mutate model state -- it returns a message. All the data it needs is captured as value copies in the closure.

**Test strategy:**

```go
func TestUpdateModelSpinnerDuringPull(t *testing.T) {
    m := newUpdateModel("/tmp", "1.0.0", "2.0.0", 5)
    m.width = 80
    m.height = 30
    m.step = stepUpdatePull
    m.loading = true

    view := m.View()
    // Should contain spinner or loading indicator text
    assertContains(t, view, "Updating syllago")
}

func TestUpdateModelLoadingFlagSet(t *testing.T) {
    m := newUpdateModel("/tmp", "1.0.0", "2.0.0", 5)
    m.step = stepUpdateMenu
    m.cursor = 0

    // Simulate "See what's new" -> should set loading
    // (We can't easily test the full async flow in unit tests,
    // but we can verify the model state transitions)

    // After fetchPreview returns:
    m2, _ := m.Update(updatePreviewMsg{log: "commit 1", stat: "1 file"})
    if m2.step != stepUpdatePreview {
        t.Fatal("expected stepUpdatePreview after preview msg")
    }
}

func TestDetailModelLoading(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyEnter) // -> items
    app = m.(App)

    m, cmd := app.Update(keyEnter) // -> detail (should return loadContent cmd)
    app = m.(App)

    // The detail should be in loading state initially
    if !app.detail.loading {
        t.Fatal("expected detail to be loading after creation")
    }

    // Cmd should not be nil (loadContent returns a command)
    if cmd == nil {
        t.Fatal("expected a command from detail creation")
    }
}
```

**Commit message:** `feat(tui): add spinner/loading indicators for update, import, and detail (4.1)`

---

## Batch E: Small UX Polish

### 4.7: Warn on unsaved settings when pressing Esc

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/settings.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/settings_test.go`

**What changes:**

Track whether settings have been modified since load. The simplest approach: auto-save on Esc. Config is local and cheap to write. A confirmation dialog adds cognitive load for no real benefit. The user already made deliberate changes; pressing Esc means "I'm done," not "throw everything away."

Add to `settingsModel`:
```go
type settingsModel struct {
    // ... existing fields ...
    dirty        bool  // true if any setting changed since load/save
}
```

Set `dirty = true` whenever a setting is toggled (in `activateRow` for auto-update, in `applySubPicker` for providers/detectors).

In `app.go`, screenSettings back handler:
```go
if a.settings.dirty {
    a.settings.save()
}
a.screen = screenCategory
```

**Test strategy:**
```go
func TestSettingsAutoSaveOnEsc(t *testing.T) {
    app := testApp(t)
    // Navigate to settings
    nTypes := len(catalog.AllContentTypes())
    app = pressN(app, keyDown, nTypes+3)
    m, _ := app.Update(keyEnter)
    app = m.(App)
    assertScreen(t, app, screenSettings)

    // Toggle auto-update (makes it dirty)
    m, _ = app.Update(keyEnter) // toggle auto-update
    app = m.(App)

    if !app.settings.dirty {
        t.Fatal("expected dirty=true after toggle")
    }

    // Esc should auto-save and go back
    m, _ = app.Update(keyEsc)
    app = m.(App)
    assertScreen(t, app, screenCategory)
}
```

### 4.9: Make `q` quit from any screen (or navigate back)

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`

**What changes:**

Currently `q` only quits from category screen. Better approach: `q` navigates back one level (like Esc), and quits from category.

For non-category screens where search is not active and no text input is active, `q` acts as back (same as Esc):

```go
// In app.go, before screen-specific handling:
if key.Matches(msg, keys.Quit) && !a.search.active {
    if a.screen == screenCategory {
        return a, tea.Quit
    }
    // On other screens, q navigates back (like esc)
    // Skip if text input is active
    if (a.screen == screenDetail && a.detail.HasTextInput()) ||
       (a.screen == screenImport && a.importer.hasTextInput()) ||
       (a.screen == screenSettings && a.settings.editMode != editNone) {
        break
    }
    // Synthesize an esc key
    return a.Update(tea.KeyMsg{Type: tea.KeyEsc})
}
```

Need to add `hasTextInput()` to importModel:
```go
func (m importModel) hasTextInput() bool {
    return m.step == stepGitURL || m.step == stepPath || m.step == stepName
}
```

**Test strategy:**
```go
func TestQFromItemsNavigatesBack(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyEnter) // -> items
    app = m.(App)
    assertScreen(t, app, screenItems)

    m, _ = app.Update(keyRune('q'))
    app = m.(App)
    assertScreen(t, app, screenCategory)
}

func TestQFromDetailNavigatesBack(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)
    m, _ := app.Update(keyRune('q'))
    app = m.(App)
    assertScreen(t, app, screenItems)
}
```

### 4.11: Add Home/End keys for jump-to-top/bottom

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/keys.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/category.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items_test.go`

**What changes:**

Add Home/End bindings to `keys.go`:
```go
Home: key.NewBinding(
    key.WithKeys("home", "g"),
    key.WithHelp("home/g", "top"),
),
End: key.NewBinding(
    key.WithKeys("end", "G"),
    key.WithHelp("end/G", "bottom"),
),
```

In `items.go` Update:
```go
case key.Matches(msg, keys.Home):
    m.cursor = 0
case key.Matches(msg, keys.End):
    if len(m.items) > 0 {
        m.cursor = len(m.items) - 1
    }
```

In `category.go` Update:
```go
case key.Matches(msg, keys.Home):
    m.cursor = 0
case key.Matches(msg, keys.End):
    m.cursor = len(m.types) + 3 // last row (settings)
```

**Gotcha with `g` and `G`:** Since `g` is lowercase and `G` is shift+g, we need to be careful that `g` doesn't conflict with other bindings. Currently no `g` binding exists, so this is safe. However, `G` (capital G) in bubbletea is a `KeyRunes` message with rune `G`. The `key.WithKeys("G")` will match shift+g correctly.

**Test strategy:**
```go
func TestItemsHomeEnd(t *testing.T) {
    items := make([]catalog.ContentItem, 20)
    for i := range items {
        items[i] = catalog.ContentItem{Name: fmt.Sprintf("item-%02d", i), Type: catalog.Skills}
    }
    m := newItemsModel(catalog.Skills, items, nil, "/tmp")
    m.width = 80
    m.height = 30

    // End jumps to last
    m, _ = m.Update(keyRune('G'))
    if m.cursor != 19 {
        t.Fatalf("expected cursor 19 after End, got %d", m.cursor)
    }

    // Home jumps to first
    m, _ = m.Update(keyRune('g'))
    if m.cursor != 0 {
        t.Fatalf("expected cursor 0 after Home, got %d", m.cursor)
    }
}
```

### 4.12: Consistent message auto-clear behavior across screens

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go`

**What changes:**

Category clears messages on any keypress (line 38 in category.go). Import clears on any keypress (line 178 in import.go). Detail view does NOT clear messages on keypress.

Add at the top of `detailModel.Update`, in the `tea.KeyMsg` case, before any other handling:
```go
case tea.KeyMsg:
    // Clear transient message on any keypress (consistent with other screens)
    if m.message != "" && msg.Type != tea.KeyEsc && !m.HasTextInput() {
        m.message = ""
        m.messageIsErr = false
    }
```

**Gotcha:** This must come BEFORE the text input routing (savePath, env setup), because those inputs set messages on completion. The clear happens first, then the action sets a new message. This is the correct order. The message clear should NOT happen during text input states (actionSavePath, actionEnvValue, etc.) because it would clear error messages while the user is still typing.

**Test strategy:**
```go
func TestDetailMessageClearsOnKeypress(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)
    app.detail.message = "test message"
    app.detail.messageIsErr = false

    // Any non-esc key should clear
    m, _ := app.Update(keyDown)
    app = m.(App)
    if app.detail.message != "" {
        t.Fatal("expected message to be cleared on keypress")
    }
}
```

### 4.13: Style table header differently from help text

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/styles.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items.go`

**What changes:**

Add a new style in `styles.go`:
```go
tableHeaderStyle = lipgloss.NewStyle().
    Foreground(mutedColor).
    Bold(true)
```

In `items.go` line 222, change:
```go
// Before:
s += helpStyle.Render(hdr) + "\n"
// After:
s += tableHeaderStyle.Render(hdr) + "\n"
```

**Test strategy:**

Visual test -- verify the style exists and has bold:
```go
func TestTableHeaderStyleIsBold(t *testing.T) {
    if !tableHeaderStyle.GetBold() {
        t.Fatal("tableHeaderStyle should be bold")
    }
}
```

**Commit message:** `feat(tui): small UX polish - q back, Home/End, message clear, table header, auto-save (4.7, 4.9, 4.11, 4.12, 4.13)`

---

## Batch F: State/Guidance Improvements

### 4.8: Add breadcrumbs/step indicators to import, update, settings screens

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/import.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/update.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/settings.go`

**What changes:**

Add a breadcrumb at the top of each screen's View().

For import, add step indicator:
```go
func (m importModel) stepLabel() string {
    switch m.step {
    case stepSource:
        return "Step 1 of 4: Source"
    case stepType:
        return "Step 2 of 4: Content Type"
    case stepProvider:
        return "Step 2b of 4: Provider"
    case stepBrowseStart, stepBrowse, stepPath:
        return "Step 3 of 4: Browse"
    case stepValidate:
        return "Step 3b of 4: Review"
    case stepGitURL:
        return "Step 2 of 3: Repository URL"
    case stepGitPick:
        return "Step 3 of 3: Select Item"
    case stepName:
        return "Step 2 of 3: Name"
    case stepConfirm:
        return "Confirm"
    }
    return ""
}
```

In `import.View()`, add after the title:
```go
s := helpStyle.Render("syllago > Import >") + " " + titleStyle.Render("Import Content") + "\n"
if label := m.stepLabel(); label != "" {
    s += helpStyle.Render(label) + "\n"
}
```

For update:
```go
s := helpStyle.Render("syllago >") + " " + titleStyle.Render("Update syllago") + "\n"
```

For settings:
```go
// Already has breadcrumb: "syllago > Settings"
// No step indicator needed for settings (single screen with sub-pickers)
```

**Test strategy:**
```go
func TestImportShowsStepIndicator(t *testing.T) {
    app := testApp(t)
    nTypes := len(catalog.AllContentTypes())
    app = pressN(app, keyDown, nTypes+1) // Import
    m, _ := app.Update(keyEnter)
    app = m.(App)

    view := app.View()
    assertContains(t, view, "Step 1")
}
```

### 4.10: Preserve detail model state when re-entering same item

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`

**What changes:**

Currently, navigating back from detail and re-entering the same item rebuilds `detailModel` from scratch. Cache the last detail model and reuse if the item path matches.

Add to `App`:
```go
type App struct {
    // ... existing fields ...
    cachedDetail     *detailModel
    cachedDetailPath string // path of the item in cachedDetail
}
```

In the items -> detail transition:
```go
if key.Matches(msg, keys.Enter) && len(a.items.items) > 0 {
    item := a.items.selectedItem()
    if a.cachedDetailPath == item.Path && a.cachedDetail != nil {
        a.detail = *a.cachedDetail
    } else {
        a.detail, cmd = newDetailModel(item, a.providers, a.catalog.RepoRoot)
    }
    a.detail.width = a.width
    a.detail.height = a.height
    a.screen = screenDetail
    return a, cmd
}
```

When navigating back from detail, cache it:
```go
if key.Matches(msg, keys.Back) {
    // ... existing back logic ...
    cached := a.detail
    a.cachedDetail = &cached
    a.cachedDetailPath = a.detail.item.Path
    a.screen = screenItems
    return a, nil
}
```

Invalidate cache when catalog changes (after import, promote, update):
- In `importDoneMsg`, `promoteDoneMsg`, `updatePullMsg` handlers, set `a.cachedDetail = nil`.

**Test strategy:**
```go
func TestDetailStatePreservedOnReenter(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)

    // Switch to Files tab
    m, _ := app.Update(keyRune('2'))
    app = m.(App)
    if app.detail.activeTab != tabFiles {
        t.Fatal("expected tabFiles")
    }

    // Navigate back
    m, _ = app.Update(keyEsc)
    app = m.(App)
    assertScreen(t, app, screenItems)

    // Re-enter same item
    m, _ = app.Update(keyEnter)
    app = m.(App)
    assertScreen(t, app, screenDetail)

    // Tab should be preserved
    if app.detail.activeTab != tabFiles {
        t.Fatalf("expected tabFiles preserved, got %d", app.detail.activeTab)
    }
}

func TestDetailStateClearedOnDifferentItem(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)

    m, _ := app.Update(keyRune('2')) // Files tab
    app = m.(App)

    // Back
    m, _ = app.Update(keyEsc)
    app = m.(App)

    // Move to different item
    m, _ = app.Update(keyDown)
    app = m.(App)

    // Enter different item
    m, _ = app.Update(keyEnter)
    app = m.(App)

    // Should NOT preserve previous tab
    if app.detail.activeTab != tabOverview {
        t.Fatalf("expected tabOverview for new item, got %d", app.detail.activeTab)
    }
}
```

### 4.14: Add empty-state guidance for My Tools

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items.go`

**What changes:**

When the items screen shows "No items found" for My Tools, add a hint:

```go
if len(m.items) == 0 {
    s += helpStyle.Render("  No items found") + "\n"
    if m.contentType == catalog.MyTools {
        s += "\n" + helpStyle.Render("  Use Import to add content, or run 'syllago add' from the command line.") + "\n"
    }
    s += "\n" + helpStyle.Render("esc back")
    return s
}
```

**Test strategy:**
```go
func TestMyToolsEmptyGuidance(t *testing.T) {
    m := newItemsModel(catalog.MyTools, nil, nil, "/tmp")
    m.width = 80
    m.height = 30

    view := m.View()
    assertContains(t, view, "Import")
    assertContains(t, view, "syllago add")
}
```

**Commit message:** `feat(tui): add breadcrumbs, state preservation, and empty-state guidance (4.8, 4.10, 4.14)`

---

## Batch G: Enhancement Polish

### 4.15: Show item count and position in scroll indicators

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/items.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`

**What changes:**

In `items.go`, replace generic scroll indicators:
```go
// Before:
s += helpStyle.Render("  (more items above)") + "\n"
// After:
s += helpStyle.Render(fmt.Sprintf("  (%d more above)", offset)) + "\n"

// Before:
s += helpStyle.Render("  (more items below)") + "\n"
// After:
s += helpStyle.Render(fmt.Sprintf("  (%d more below)", len(m.items)-end)) + "\n"
```

Same pattern in `detail_render.go` for file viewer scroll (lines 188-199).

**Test strategy:**
```go
func TestItemsScrollCountAbove(t *testing.T) {
    items := make([]catalog.ContentItem, 50)
    for i := range items {
        items[i] = catalog.ContentItem{Name: fmt.Sprintf("item-%02d", i), Type: catalog.Skills}
    }
    m := newItemsModel(catalog.Skills, items, nil, "/tmp")
    m.width = 80
    m.height = 10 // Small height forces scrolling

    // Navigate to bottom
    for i := 0; i < 40; i++ {
        m, _ = m.Update(keyDown)
    }

    view := m.View()
    assertContains(t, view, "more above")
    // Should show count, not just "more items above"
    assertNotContains(t, view, "(more items above)")
}
```

### 4.16: Show item position in detail view breadcrumb

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go` (add fields)
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go` (render position)
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go` (pass cursor/count)

**What changes:**

Add to `detailModel`:
```go
type detailModel struct {
    // ... existing ...
    listPosition int // 0-based position in the items list
    listTotal    int // total items in the list
}
```

In `app.go`, when entering detail, set these:
```go
a.detail.listPosition = a.items.cursor
a.detail.listTotal = len(a.items.items)
```

In `detail_render.go` `renderContent()`, update breadcrumb:
```go
// Before:
s := helpStyle.Render("syllago > "+m.item.Type.Label()+" >") + " " + titleStyle.Render(name)
// After:
position := ""
if m.listTotal > 0 {
    position = fmt.Sprintf(" (%d of %d)", m.listPosition+1, m.listTotal)
}
s := helpStyle.Render("syllago > "+m.item.Type.Label()+" >") + " " + titleStyle.Render(name) + helpStyle.Render(position)
```

**Test strategy:**
```go
func TestDetailShowsPosition(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)
    view := app.View()
    assertContains(t, view, "1 of")
}
```

### 4.17: Add next/previous item navigation from detail view

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go`
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go` (help bar)

**What changes:**

Add `ctrl+n` / `ctrl+p` navigation at the app level (not detail level, since it needs access to the items list).

In `app.go`, in `screenDetail` key handling, before delegating to detail:
```go
case screenDetail:
    // Next/previous item navigation
    if msg.String() == "ctrl+n" && !a.detail.HasTextInput() && a.detail.confirmAction == actionNone {
        if a.items.cursor < len(a.items.items)-1 {
            a.items.cursor++
            item := a.items.selectedItem()
            a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
            a.detail.width = a.width
            a.detail.height = a.height
            a.detail.listPosition = a.items.cursor
            a.detail.listTotal = len(a.items.items)
            return a, cmd // cmd from newDetailModel if async
        }
        return a, nil
    }
    if msg.String() == "ctrl+p" && !a.detail.HasTextInput() && a.detail.confirmAction == actionNone {
        if a.items.cursor > 0 {
            a.items.cursor--
            item := a.items.selectedItem()
            a.detail = newDetailModel(item, a.providers, a.catalog.RepoRoot)
            a.detail.width = a.width
            a.detail.height = a.height
            a.detail.listPosition = a.items.cursor
            a.detail.listTotal = len(a.items.items)
            return a, cmd
        }
        return a, nil
    }
```

Update help bar in `detail_render.go` to include `ctrl+n/p`:
```go
if m.listTotal > 1 {
    helpParts = append(helpParts, "ctrl+n/p next/prev")
}
```

**Test strategy:**
```go
func TestDetailNextPrevItem(t *testing.T) {
    app := testApp(t)
    m, _ := app.Update(keyEnter) // -> items
    app = m.(App)

    if len(app.items.items) < 2 {
        t.Skip("need at least 2 items")
    }

    m, _ = app.Update(keyEnter) // -> detail of first item
    app = m.(App)
    firstName := app.detail.item.Name

    // ctrl+n goes to next
    ctrlN := tea.KeyMsg{Type: tea.KeyCtrlN}
    m, _ = app.Update(ctrlN)
    app = m.(App)

    if app.detail.item.Name == firstName {
        t.Fatal("expected different item after ctrl+n")
    }
    assertScreen(t, app, screenDetail)

    // ctrl+p goes back
    ctrlP := tea.KeyMsg{Type: tea.KeyCtrlP}
    m, _ = app.Update(ctrlP)
    app = m.(App)

    if app.detail.item.Name != firstName {
        t.Fatalf("expected %s after ctrl+p, got %s", firstName, app.detail.item.Name)
    }
}
```

### 4.18: Preview install destination paths before confirming

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`

**What changes:**

In the method picker (`actionChooseMethod`), show the target paths for each checked provider:

```go
if m.confirmAction == actionChooseMethod || m.confirmAction == actionSaveMethod {
    // ... existing method picker code ...

    // Show destination preview
    detected := m.detectedProviders()
    s += "\n" + helpStyle.Render("Destination paths:") + "\n"
    for i, checked := range m.providerChecks {
        if !checked || i >= len(detected) {
            continue
        }
        p := detected[i]
        destDir := p.InstallDir("", m.item.Type) // homeDir not needed for preview
        dest := filepath.Join(destDir, m.item.Name)
        s += "  " + helpStyle.Render(p.Name+": ") + valueStyle.Render(dest) + "\n"
    }
}
```

**Gotcha:** The `InstallDir` function takes a homeDir parameter. We'll need to resolve it. Check the provider interface: `InstallDir` is `func(homeDir string, ct catalog.ContentType) string`. In production, homeDir would be `os.UserHomeDir()`. For preview, we can call it with the actual home dir or just show the path pattern.

**Test strategy:**
```go
func TestDetailMethodPickerShowsPaths(t *testing.T) {
    app := navigateToDetail(t, catalog.Skills)
    app.detail.activeTab = tabInstall
    app.detail.confirmAction = actionChooseMethod
    app.detail.providerChecks[0] = true

    view := app.View()
    assertContains(t, view, "Destination")
}
```

### 4.19: Audit help bar shortcuts match functional state

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail_render.go`

**What changes:**

This is an audit task. Walk through `renderHelp()` and verify each shortcut listed actually does something in the current state. The main issue to check:

1. "c copy" shows on Overview tab for prompts -- does `c` work? Yes, `keys.Copy` is handled.
2. "s save" shows on Overview tab for prompts -- but save only works on Install tab. **Bug:** The help bar shows "c copy, s save" on Overview tab, but `s` is gated to `tabInstall`. Fix: remove "s save" from Overview tab help, or make `s` work from Overview too.

Fix in `renderHelp()`:
```go
case tabOverview:
    helpParts = append(helpParts, "up/down scroll")
    if m.item.Type == catalog.Prompts && m.item.Body != "" && m.confirmAction == actionNone {
        helpParts = append(helpParts, "c copy")
        // Note: "s save" only works on Install tab, don't show here
    }
```

3. "shift+tab" should now appear (after 4.6)
4. "?" should appear (after 4.5)
5. "ctrl+n/p" should appear (after 4.17)

These will be added as part of their respective items.

**Test strategy:**
```go
func TestHelpBarNoSaveOnOverviewTab(t *testing.T) {
    app := navigateToDetail(t, catalog.Prompts)
    // Should be on Overview tab by default
    view := app.detail.renderHelp()
    assertNotContains(t, view, "s save")
    assertContains(t, view, "c copy")
}
```

### 4.20: Warn about large directories in file browser

**Files to modify:**
- `/home/hhewett/.local/src/syllago/cli/internal/tui/filebrowser.go`

**What changes:**

Add a skip list for known non-content directories. In `loadDir`, skip entries matching common bloat directories:

```go
var skipDirs = map[string]bool{
    "node_modules": true,
    ".git":         true,
    "__pycache__":  true,
    ".venv":        true,
    "venv":         true,
    "target":       true, // Rust
    "vendor":       true, // Go
    ".tox":         true,
    ".mypy_cache":  true,
    ".pytest_cache": true,
    "dist":         true,
    "build":        true,
}
```

In the entry loop of `loadDir`:
```go
for _, e := range osEntries {
    if e.Name() == "." || e.Name() == ".." {
        continue
    }
    if e.IsDir() && skipDirs[e.Name()] {
        continue // skip known non-content directories
    }
    // ... rest of loop
}
```

Also cap the total visible entries. If a directory has more than 500 entries, show a warning and only load the first 500:
```go
const maxEntries = 500
if len(osEntries) > maxEntries {
    fb.errMsg = fmt.Sprintf("Directory has %d entries (showing first %d). Navigate into subdirectories for better performance.", len(osEntries), maxEntries)
    osEntries = osEntries[:maxEntries]
}
```

**Test strategy:**
```go
func TestFileBrowserSkipsNodeModules(t *testing.T) {
    tmp := t.TempDir()
    os.MkdirAll(filepath.Join(tmp, "node_modules"), 0755)
    os.MkdirAll(filepath.Join(tmp, "src"), 0755)

    fb := newFileBrowser(tmp, catalog.Skills)

    for _, entry := range fb.entries {
        if entry.name == "node_modules" {
            t.Fatal("node_modules should be skipped in file browser")
        }
    }
    // src should still appear
    found := false
    for _, entry := range fb.entries {
        if entry.name == "src" {
            found = true
        }
    }
    if !found {
        t.Fatal("expected 'src' to still appear")
    }
}
```

**Commit message(s):**
- `feat(tui): add scroll counts, item position, and next/prev navigation (4.15, 4.16, 4.17)`
- `feat(tui): add install path preview, help bar audit, large dir warning (4.18, 4.19, 4.20)`

---

## Summary of All Commits (in execution order)

1. `feat(tui): add Shift+Tab for reverse tab cycling in detail view (4.6)`
2. `fix(tui): resolve c key conflict by changing file browser confirm to d (4.3)`
3. `feat(tui): implement live-filtering search and fix search overlay position (4.2, 4.4)`
4. `feat(tui): add ? help overlay with context-sensitive keyboard shortcuts (4.5)`
5. `feat(tui): add spinner/loading indicators for update, import, and detail (4.1)`
6. `feat(tui): small UX polish - q back, Home/End, message clear, table header, auto-save (4.7, 4.9, 4.11, 4.12, 4.13)`
7. `feat(tui): add breadcrumbs, state preservation, and empty-state guidance (4.8, 4.10, 4.14)`
8. `feat(tui): add scroll counts, item position, and next/prev navigation (4.15, 4.16, 4.17)`
9. `feat(tui): add install path preview, help bar audit, large dir warning (4.18, 4.19, 4.20)`

---

## Key Architectural Decisions

**Why `d` instead of Enter for file browser confirm (4.3):** Enter already has meaning (open directory). Overloading it with "confirm when selections exist" is confusing -- you'd enter a directory when you meant to confirm. A dedicated key is clearer.

**Why live-filter only on items screen (4.2):** On the category screen, there's nothing to visually filter -- it's a fixed list of content types. Live-filtering makes sense where you have a variable-length list of items. The match count preview on category gives feedback without disrupting the layout.

**Why auto-save settings (4.7):** Config is local and cheap to write. A confirmation dialog adds cognitive load for no real benefit. The user already made deliberate changes; pressing Esc means "I'm done," not "throw everything away."

**Why cache only the last detail model (4.10):** Caching multiple models adds memory pressure and complexity for marginal benefit. Users typically go back-and-forth with the same item. Invalidation is simple: clear on catalog changes.

**Why spinner.Dot style (4.1):** The Dot spinner is the most minimal and terminal-friendly. It works in all terminal emulators and doesn't distract from content.

### Critical Files for Implementation
- `/home/hhewett/.local/src/syllago/cli/internal/tui/app.go` - Central routing hub; nearly every item touches this file for navigation, state management, and message routing
- `/home/hhewett/.local/src/syllago/cli/internal/tui/detail.go` - Most complex model; items 4.1, 4.6, 4.10, 4.12, 4.16, 4.17 all modify it
- `/home/hhewett/.local/src/syllago/cli/internal/tui/keys.go` - Key binding registry; items 4.3, 4.5, 4.6, 4.11 add bindings here
- `/home/hhewett/.local/src/syllago/cli/internal/tui/search.go` - Live search logic; items 4.2 and 4.4 fundamentally change search behavior
- `/home/hhewett/.local/src/syllago/cli/internal/tui/filebrowser.go` - Items 4.3 and 4.20 fix key conflicts and add safety guardrails
