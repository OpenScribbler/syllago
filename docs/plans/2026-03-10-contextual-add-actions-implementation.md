# Implementation Plan: Contextual Add Actions

**Date:** 2026-03-10
**Feature branch:** `feat/contextual-add-actions`
**Design doc:** `docs/prompts/contextual-add-actions-brainstorm.md`

---

## Overview

This plan covers three features (Contextual Add Actions, Smart Registry Add, Create Loadout Wizard), three bug fixes, and cross-cutting cleanup. Tasks are bite-sized (2–5 min each) and organized into phases with explicit dependencies.

Phases 1–2 are independent of each other and of later phases. Phase 3 depends on Phase 2 (key bindings). Phases 4–8 each depend on the phase directly before them plus the key binding infrastructure from Phase 2.

---

## Phase 1: Bug Fixes

These are small, independent, and touch real production paths. Do them first as a warmup.

### 1.1 — B1: Fix security notice text

**File:** `cli/cmd/syllago/registry_cmd.go`
**Depends on:** nothing

The security notice at line 91 says `"hooks, prompts"`. Prompts was removed in commit `0b76352`.

**Change:** In `registryAddCmd.RunE`, find:
```
│  hooks, prompts) that will be available for install.  │
```
Replace with:
```
│  hooks, commands) that will be available for install.  │
```

**Verify:** `grep -n "prompts" cli/cmd/syllago/registry_cmd.go` should return nothing after this change (except any comments).

**Test:** `make test` — no test exists for this string, verify manually by reading the modified file.

**Commit:** `fix: security notice mentions prompts, should be commands`

---

### 1.2 — B3: Orphaned clone cleanup on failed config save

**File:** `cli/cmd/syllago/registry_cmd.go`
**Depends on:** nothing

Currently, if `config.Save` fails after `registry.Clone` succeeds, the cloned directory is left on disk forever. Fix: remove the clone directory when config save fails.

**Change:** In `registryAddCmd.RunE`, the config save block currently reads:
```go
if err := config.Save(root, cfg); err != nil {
    return fmt.Errorf("saving config: %w", err)
}
```

Replace with:
```go
if err := config.Save(root, cfg); err != nil {
    // Config save failed — clean up the clone so it doesn't become orphaned.
    dir, _ := registry.CloneDir(name)
    os.RemoveAll(dir)
    return fmt.Errorf("saving config: %w", err)
}
```

**Note:** `os` is already imported. `registry.CloneDir` returns `(string, error)`; the `_` discards the error because we're already in an error path and cleanup is best-effort.

**Test:** No automated test covers this (it would require a mock filesystem). Add a code comment noting this is best-effort cleanup. `make test` must still pass.

**Commit:** `fix: clean up orphaned clone when config save fails after registry add`

---

### 1.3 — B2: Registry naming — preserve org/repo format

This bug has three sub-tasks. Do them in order within this fix.

**Depends on:** nothing, but must complete all three sub-tasks before committing.

#### 1.3a — Update `NameFromURL` to return org/repo

**File:** `cli/internal/registry/registry.go`

Current `NameFromURL` returns only the last path segment. New behavior: return `owner/repo` for URLs that have an org/owner component.

**Replace** the existing `NameFromURL` function:
```go
// NameFromURL derives a registry name from a git URL.
// Returns "owner/repo" format when the URL has an org/owner prefix,
// or just "repo" for bare single-segment names.
// Examples:
//
//	"git@github.com:acme/my-tools.git"          → "acme/my-tools"
//	"https://github.com/acme/my-tools"           → "acme/my-tools"
//	"https://github.com/acme/my-tools.git"       → "acme/my-tools"
//	"https://example.com/my-tools.git"           → "my-tools"
func NameFromURL(url string) string {
    url = strings.TrimSuffix(url, "/")
    url = strings.TrimSuffix(url, ".git")

    // git@ SSH format: git@host:owner/repo
    if i := strings.Index(url, ":"); i >= 0 {
        path := url[i+1:]
        parts := strings.Split(path, "/")
        if len(parts) >= 2 {
            return parts[len(parts)-2] + "/" + parts[len(parts)-1]
        }
        return parts[len(parts)-1]
    }

    // HTTPS format: https://host/owner/repo or https://host/repo
    if i := strings.Index(url, "://"); i >= 0 {
        path := url[i+3:]
        // strip host
        if j := strings.Index(path, "/"); j >= 0 {
            path = path[j+1:]
        }
        parts := strings.Split(strings.Trim(path, "/"), "/")
        if len(parts) >= 2 {
            return parts[len(parts)-2] + "/" + parts[len(parts)-1]
        }
        if len(parts) == 1 && parts[0] != "" {
            return parts[0]
        }
    }

    // Fallback: last segment after any / or :
    last := url
    if i := strings.LastIndexAny(url, "/:"); i >= 0 {
        last = url[i+1:]
    }
    return last
}
```

#### 1.3b — Update `IsValidItemName` to allow `/` in registry names

**File:** `cli/internal/catalog/scanner.go`

The current `validItemNameRe` rejects `/`. Registry names now use `owner/repo` format. Content item names must still not contain `/` (they're used as JSON key paths). We need a separate validation function for registry names.

**Add** after the existing `IsValidItemName` function:
```go
// validRegistryNameRe matches registry names in owner/repo format.
// Allows letters, numbers, - and _ in each segment, with an optional / separator.
var validRegistryNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)?$`)

// IsValidRegistryName checks if a name is valid for use as a registry name.
// Allows the owner/repo format in addition to plain names.
func IsValidRegistryName(name string) bool {
    return validRegistryNameRe.MatchString(name)
}
```

#### 1.3c — Update `Clone` and `CloneDir` to handle nested paths; update `registry_cmd.go` validation

**File:** `cli/internal/registry/registry.go`

`CloneDir` uses `filepath.Join(cache, name)`. With `name = "acme/my-tools"`, this already produces `~/.syllago/registries/acme/my-tools` because `filepath.Join` handles `/` natively. The `os.MkdirAll(filepath.Dir(dir), 0755)` call in `Clone` creates the parent, so this already works correctly.

What needs updating: the `IsValidItemName` call inside `Clone` uses the content-item validator, which rejects `/`. Replace it with the new `IsValidRegistryName`.

**Note:** `registry.go` already imports `"github.com/OpenScribbler/syllago/cli/internal/catalog"`, so no new import is needed.

**In `Clone`**, replace:
```go
if !catalog.IsValidItemName(name) {
    return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
}
```
with:
```go
if !catalog.IsValidRegistryName(name) {
    return fmt.Errorf("registry name %q is invalid (use letters, numbers, - and _ with optional owner/repo format)", name)
}
```

**File:** `cli/cmd/syllago/registry_cmd.go`

In `registryAddCmd.RunE`, replace the name validation call and error message:
```go
if !catalog.IsValidItemName(name) {
    return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
}
```
with:
```go
if !catalog.IsValidRegistryName(name) {
    return fmt.Errorf("registry name %q is invalid (use letters, numbers, - and _ with optional owner/repo format)", name)
}
```

Also update the duplicate-name check hint message to reflect that names may now include `/`:
```go
return fmt.Errorf("registry %q already exists (use --name to override or remove it first)", name)
```

**Tests — `cli/internal/registry/registry_test.go`**: An existing `TestNameFromURL` function exists with 5 cases that expect the OLD short-form values (e.g., `"my-tools"`). **Replace the entire function** with the new test cases that expect org/repo format:

```go
func TestNameFromURL(t *testing.T) {
    tests := []struct {
        url  string
        want string
    }{
        {"https://github.com/acme/my-tools.git", "acme/my-tools"},
        {"https://github.com/acme/my-tools", "acme/my-tools"},
        {"https://github.com/acme/my-tools/", "acme/my-tools"},
        {"git@github.com:acme/my-tools.git", "acme/my-tools"},
        {"git@github.com:acme/my-tools", "acme/my-tools"},
        {"git@github.com:acme/my_tools.git", "acme/my_tools"},
        {"https://example.com/my-tools.git", "my-tools"},
        {"https://example.com/my-tools", "my-tools"},
    }
    for _, tt := range tests {
        t.Run(tt.url, func(t *testing.T) {
            if got := NameFromURL(tt.url); got != tt.want {
                t.Errorf("NameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
            }
        })
    }
}
```

**Tests to add — `cli/internal/catalog/scanner_test.go`** (add to existing):
```go
func TestIsValidRegistryName(t *testing.T) {
    valid := []string{"acme/my-tools", "my-tools", "acme/my_tools-v2", "user123/repo-name"}
    for _, name := range valid {
        if !IsValidRegistryName(name) {
            t.Errorf("IsValidRegistryName(%q) = false, want true", name)
        }
    }
    invalid := []string{"", "a/b/c", "a b", "a.b", "a*b"}
    for _, name := range invalid {
        if IsValidRegistryName(name) {
            t.Errorf("IsValidRegistryName(%q) = true, want false", name)
        }
    }
}
```

**Run:** `make test`

**Commit:** `feat: registry names use owner/repo format (NameFromURL, IsValidRegistryName)`

---

## Phase 2: Key Binding Infrastructure

### 2.1 — Add new key bindings to keys.go

**File:** `cli/internal/tui/keys.go`
**Depends on:** nothing

Add four new bindings to the `keyMap` struct and `keys` var. These are screen-specific — they won't conflict because they're only checked in specific `case` blocks in `App.Update`.

**Note on `l` conflict:** `l` is currently bound as an alias for `Right` (`key.WithKeys("right", "l")`). The `l` for "create loadout" is only active on registry-related screens where `Right` is not meaningful (it's only used in card grids and detail tabs, not on the registries screen or its drill-in items view). We resolve this by checking the specific `"l"` rune directly on the screens where it applies, rather than using a `key.Matches` call against the `Right` binding. Add a dedicated `CreateLoadout` binding that does NOT include `l` as a `Right` alias.

**In `keyMap` struct**, add after `ToggleHidden`:
```go
Add           key.Binding
Delete        key.Binding
Refresh       key.Binding
CreateLoadout key.Binding
```

**In `keys` var**, add after `ToggleHidden`:
```go
Add: key.NewBinding(
    key.WithKeys("a"),
    key.WithHelp("a", "add"),
),
Delete: key.NewBinding(
    key.WithKeys("d"),
    key.WithHelp("d", "remove"),
),
Refresh: key.NewBinding(
    key.WithKeys("r"),
    key.WithHelp("r", "sync"),
),
CreateLoadout: key.NewBinding(
    key.WithKeys("l"),
    key.WithHelp("l", "create loadout"),
),
```

**Remove `l` from the `Right` binding.** Change:
```go
Right: key.NewBinding(
    key.WithKeys("right", "l"),
    key.WithHelp("right/l", "enter"),
),
```
to:
```go
Right: key.NewBinding(
    key.WithKeys("right"),
    key.WithHelp("right", "enter"),
),
```

**Why remove `l` from Right:** The `l` vim key for "move right" is a nice-to-have but not worth a conflict. Users can use the `Right` arrow key. The `l` binding for "create loadout" is a first-class feature.

**Test:** `make test` (key binding changes are integration-tested via golden files).

**Commit:** `feat(tui/keys): add a/d/r/l key bindings; remove l as Right alias`

---

## Phase 3: Registry Card View

This phase replaces the table-based `registriesModel` with a card grid, moves navigation into `App.Update()` (matching the Library/Loadouts pattern), and updates golden files.

### 3.1 — Rewrite `registries.go` as a pure view model

**File:** `cli/internal/tui/registries.go`
**Depends on:** Phase 2 (new key bindings available, though not wired yet)

The new `registriesModel` is a pure render model — no navigation logic, no cursor management. Navigation lives in `App.Update()` the same way `screenLibraryCards` and `screenLoadoutCards` work. The model holds data and renders cards.

**Replace the entire file contents** with:

```go
package tui

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    zone "github.com/lrstanley/bubblezone"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/config"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
)

// registryEntry holds display data for one registry card.
type registryEntry struct {
    name        string
    url         string
    ref         string
    cloned      bool
    itemCount   int
    version     string
    description string
}

type registriesModel struct {
    entries  []registryEntry
    width    int
    height   int
    repoRoot string
}

func newRegistriesModel(repoRoot string, cfg *config.Config, cat *catalog.Catalog) registriesModel {
    entries := make([]registryEntry, len(cfg.Registries))
    for i, r := range cfg.Registries {
        entry := registryEntry{
            name:      r.Name,
            url:       r.URL,
            ref:       r.Ref,
            cloned:    registry.IsCloned(r.Name),
            itemCount: cat.CountRegistry(r.Name),
        }
        if manifest, err := registry.LoadManifest(r.Name); err == nil && manifest != nil {
            entry.version = manifest.Version
            entry.description = manifest.Description
        }
        entries[i] = entry
    }
    return registriesModel{
        entries:  entries,
        repoRoot: repoRoot,
    }
}

func (m registriesModel) Update(msg tea.Msg) (registriesModel, tea.Cmd) {
    // Navigation is handled by App.Update(). This model has no cursor state.
    return m, nil
}

func (m registriesModel) helpText() string {
    return "up/down/left/right navigate • enter browse • a add • d remove • r sync • esc back"
}

func (m registriesModel) View(cursor int) string {
    home := zone.Mark("crumb-home", helpStyle.Render("Home"))
    s := home + helpStyle.Render(" > ") + titleStyle.Render("Registries") + "\n\n"

    if len(m.entries) == 0 {
        s += helpStyle.Render("  No registries configured.") + "\n\n"
        s += helpStyle.Render("  Press a to add a registry.") + "\n"
        return s
    }

    cardWidth := 36
    cols := 2
    if m.width < 80 {
        cols = 1
    }

    for i, entry := range m.entries {
        if i > 0 && i%cols == 0 {
            s += "\n"
        }

        selected := i == cursor
        card := renderRegistryCard(entry, cardWidth, selected)
        s += zone.Mark(fmt.Sprintf("registry-card-%d", i), card)
        if cols > 1 && i%cols == 0 && i+1 < len(m.entries) {
            s += "  "
        } else if i%cols == cols-1 || i == len(m.entries)-1 {
            s += "\n"
        }
    }

    return s
}

func renderRegistryCard(entry registryEntry, width int, selected bool) string {
    cardStyle := cardNormalStyle
    if selected {
        cardStyle = cardSelectedStyle
    }

    status := helpStyle.Render("missing")
    if entry.cloned {
        status = installedStyle.Render("cloned")
    }

    version := "─"
    if entry.version != "" {
        version = entry.version
    }

    urlDisplay := entry.url
    maxURL := width - 4
    if len(urlDisplay) > maxURL {
        urlDisplay = urlDisplay[:maxURL-3] + "..."
    }

    desc := entry.description
    if len(desc) > width-4 {
        desc = desc[:width-7] + "..."
    }

    title := truncate(entry.name, width-4)
    meta := fmt.Sprintf("%s  v%s  %d items", status, helpStyle.Render(version), entry.itemCount)

    var lines []string
    lines = append(lines, titleStyle.Render(title))
    lines = append(lines, meta)
    lines = append(lines, helpStyle.Render(urlDisplay))
    if desc != "" {
        lines = append(lines, helpStyle.Render(desc))
    }

    return cardStyle.Width(width).Render(strings.Join(lines, "\n"))
}
```

**Note:** `cardNormalStyle` and `cardSelectedStyle` must be added to `styles.go` (next task).

### 3.2 — Add card styles to styles.go

**File:** `cli/internal/tui/styles.go`
**Depends on:** 3.1

Find the existing style block (near `installedStyle`, `itemStyle`, etc.) and add:

```go
cardNormalStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(mutedColor).
    Padding(0, 1)

cardSelectedStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(accentColor).
    Padding(0, 1).
    Bold(true)

// Both styles have identical Padding(0, 1) to prevent layout jitter when switching selection.
```

### 3.3 — Update `App` struct and `App.Update()` for card-based registry navigation

**File:** `cli/internal/tui/app.go`
**Depends on:** 3.1, 3.2, Phase 2

The `registriesModel` no longer has a `cursor` field. The cursor lives on `App` as `cardCursor` (already exists and is used for library/loadout cards). We need to reuse `cardCursor` for registries too, but `cardCursor` already serves library/loadout cards. Because the user can only be on one screen at a time, `cardCursor` is safe to reuse across all card screens.

**Important:** The old `registriesModel` has a `selectedName()` method called at `app.go:1375` (`name := a.registries.selectedName()`). Since the new model has no cursor or `selectedName()`, this call must be replaced with `a.registries.entries[a.cardCursor].name`. Find and update this call site.

**In `App.View()`** (find where `a.registries.View()` is called): update the call to pass `a.cardCursor`:

Find:
```go
case screenRegistries:
    content = a.registries.View()
```
Replace with:
```go
case screenRegistries:
    content = a.registries.View(a.cardCursor)
```

**In `App.Update()`, `case screenRegistries:`**: replace the existing handler block (lines ~1367–1391) with full card navigation matching the `screenLibraryCards` pattern:

```go
case screenRegistries:
    if key.Matches(msg, keys.Back) {
        a.screen = screenCategory
        a.focus = focusSidebar
        return a, nil
    }
    cols := 2
    if a.width < 80 {
        cols = 1
    }
    if key.Matches(msg, keys.Up) {
        if a.cardCursor >= cols {
            a.cardCursor -= cols
        }
        return a, nil
    }
    if key.Matches(msg, keys.Down) {
        if a.cardCursor+cols < len(a.registries.entries) {
            a.cardCursor += cols
        }
        return a, nil
    }
    if key.Matches(msg, keys.Left) {
        if a.cardCursor > 0 {
            a.cardCursor--
        }
        return a, nil
    }
    if key.Matches(msg, keys.Right) {
        if a.cardCursor+1 < len(a.registries.entries) {
            a.cardCursor++
        }
        return a, nil
    }
    if key.Matches(msg, keys.Enter) && len(a.registries.entries) > 0 {
        entry := a.registries.entries[a.cardCursor]
        regItems := a.visibleItems(a.catalog.ByRegistry(entry.name))
        items := newItemsModel(catalog.SearchResults, regItems, a.providers, a.catalog.RepoRoot)
        items.sourceRegistry = entry.name
        items.width = a.width - sidebarWidth - 1
        items.height = a.panelHeight()
        a.items = items
        a.cardParent = 0
        a.screen = screenItems
        a.focus = focusContent
        return a, nil
    }
    return a, nil
```

**Also update** the `WindowSizeMsg` handler: the existing `a.registries.width = contentW` and `a.registries.height = ph` lines can remain (the model still stores dimensions for layout).

**Also update** the mouse handler: find the block that handled `registry-row-N` clicks (it was in `registriesModel.Update`; now that `registriesModel.Update` is a no-op, add card click handling in the `tea.MouseMsg` section of `App.Update`). Find the section near the loadout card click handler and add:

```go
// Check registry card clicks
if a.screen == screenRegistries {
    for i := range a.registries.entries {
        if zone.Get(fmt.Sprintf("registry-card-%d", i)).InBounds(msg) {
            if i == a.cardCursor {
                // Second click on already-selected card → drill in
                return a.Update(tea.KeyMsg{Type: tea.KeyEnter})
            }
            a.cardCursor = i
            return a, nil
        }
    }
}
```

### 3.4 — Update breadcrumb for registry items view

**File:** `cli/internal/tui/items.go`
**Depends on:** 3.3

When viewing items from a registry drill-in (`items.sourceRegistry != ""`), the breadcrumb currently says `Home > Registries > {registry}`. Verify this is correct by checking the `View()` method on `itemsModel`. Find the breadcrumb rendering block and ensure it uses `zone.Mark("crumb-registries", ...)` so the existing click handler in `App.Update()` works. No change needed if it already does this — just verify.

### 3.5 — Regenerate golden files

**Depends on:** 3.1–3.4
**Command:**
```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/tui/ -update-golden
```

Then verify the diff: the registries screen golden file should now show cards instead of a table. All other golden files should be unchanged.

**Run:** `make test` — must pass fully.

**Commit:** `feat(tui): replace registry table with card grid`

---

## Phase 4: Registry Actions (add/remove/sync modals)

### 4.1 — Add registry action message types

**File:** `cli/internal/tui/app.go`
**Depends on:** Phase 3

Add message types for async registry operations. Place these near the other message type definitions (search for `type importDoneMsg` to find the right area).

**Important:** Also add `"github.com/OpenScribbler/syllago/cli/internal/registry"` to the import block in `app.go` — it's not currently imported but is needed by `doRegistryAdd` (Phase 4.3) and `doCreateLoadout` (Phase 7.3).

```go
// registryAddDoneMsg is sent when a background registry clone completes.
type registryAddDoneMsg struct {
    name string
    err  error
}

// registryRemoveDoneMsg is sent when a registry remove operation completes.
type registryRemoveDoneMsg struct {
    name string
    err  error
}

// registrySyncDoneMsg is sent when a registry sync operation completes.
type registrySyncDoneMsg struct {
    name string
    err  error
}
```

### 4.2 — Add registry action modals to `modal.go` or a new file

**File:** `cli/internal/tui/modal.go` (add to existing file)
**Depends on:** 4.1, Phase 2

Add a `registryAddModal` for entering a git URL (single text input, modal wizard), and extend `modalPurpose` for remove confirmation.

First, **find the `modalPurpose` type** in `modal.go` and add new constants:
```go
modalRegistryRemove modalPurpose = iota + <next_value>
```
(Check what the last `iota` value is and continue from there. The type is a plain `int`-based iota, so just append to the `const` block.)

Add `modalRegistryRemove` to the `const` block after the last existing entry:
```go
modalRegistryRemove
```

Then **add a `registryAddModal`** struct (new full type at the bottom of `modal.go`):

```go
// registryAddModal is a single-step modal for entering a git URL to add as a registry.
type registryAddModal struct {
    active       bool
    urlInput     textinput.Model
    nameInput    textinput.Model
    focusedField int // 0 = url, 1 = name (optional override)
    message      string
    messageIsErr bool
}

func newRegistryAddModal() registryAddModal {
    ui := textinput.New()
    ui.Prompt = labelStyle.Render("URL: ")
    ui.Placeholder = "https://github.com/owner/repo.git"
    ui.CharLimit = 500
    ui.Focus()

    ni := textinput.New()
    ni.Prompt = labelStyle.Render("Name (optional): ")
    ni.Placeholder = "auto-derived from URL"
    ni.CharLimit = 100

    return registryAddModal{
        active:   true,
        urlInput: ui,
        nameInput: ni,
    }
}

func (m registryAddModal) Update(msg tea.Msg) (registryAddModal, tea.Cmd) {
    if !m.active {
        return m, nil
    }
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch {
        case msg.Type == tea.KeyEsc:
            m.active = false
            return m, nil
        case msg.Type == tea.KeyTab:
            if m.focusedField == 0 {
                m.focusedField = 1
                m.urlInput.Blur()
                m.nameInput.Focus()
            } else {
                m.focusedField = 0
                m.nameInput.Blur()
                m.urlInput.Focus()
            }
            return m, nil
        case msg.Type == tea.KeyEnter:
            // Confirm: URL must be non-empty
            url := strings.TrimSpace(m.urlInput.Value())
            if url == "" {
                m.message = "URL is required"
                m.messageIsErr = true
                return m, nil
            }
            m.active = false
            return m, nil
        }
        var cmd tea.Cmd
        if m.focusedField == 0 {
            m.urlInput, cmd = m.urlInput.Update(msg)
        } else {
            m.nameInput, cmd = m.nameInput.Update(msg)
        }
        return m, cmd
    }
    return m, nil
}

const registryAddModalWidth = 56

func (m registryAddModal) View() string {
    title := titleStyle.Render("Add Registry")
    body := labelStyle.Render("Enter a git URL to add as a registry.\n")
    body += "Tab to set an optional name override.\n\n"
    body += m.urlInput.View() + "\n"
    body += m.nameInput.View()
    if m.message != "" {
        if m.messageIsErr {
            body += "\n" + errorMsgStyle.Render(m.message)
        } else {
            body += "\n" + successMsgStyle.Render(m.message)
        }
    }
    content := title + "\n\n" + body
    return lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(modalBorderColor).
        Background(modalBgColor).
        Padding(1, 2).
        Width(registryAddModalWidth).
        Render(content)
}

func (m registryAddModal) overlayView(background string) string {
    return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}
```

### 4.3 — Add `registryAddModal` field to `App` and wire it up

**File:** `cli/internal/tui/app.go`
**Depends on:** 4.2

**Add fields to `App` struct:**
```go
registryAddModal     registryAddModal
registryOpInProgress bool // true while a registry add/remove/sync is running; blocks double-actions
```
(place alongside the other modal fields: `modal`, `saveModal`, `envModal`, `instModal`)

**Wire up in `App.Update()` — modal input routing:**
In the keyboard routing section (near `if a.modal.active {`), add:
```go
if a.registryAddModal.active {
    var cmd tea.Cmd
    a.registryAddModal, cmd = a.registryAddModal.Update(msg)
    if !a.registryAddModal.active {
        a.focus = focusContent
        url := strings.TrimSpace(a.registryAddModal.urlInput.Value())
        if url != "" {
            // Start async clone
            nameOverride := strings.TrimSpace(a.registryAddModal.nameInput.Value())
            return a, a.doRegistryAdd(url, nameOverride)
        }
    }
    return a, cmd
}
```

**Wire up in `App.View()`:** In the overlay rendering section (find where `a.modal.active` is checked for overlay), add:
```go
if a.registryAddModal.active {
    return zone.Scan(a.registryAddModal.overlayView(base))
}
```

**Wire up `a` key on `screenRegistries`:** In `App.Update()`, inside `case screenRegistries:`, after the `Back` check:
```go
if key.Matches(msg, keys.Add) && !a.registryOpInProgress {
    a.registryAddModal = newRegistryAddModal()
    a.focus = focusModal
    return a, nil
}
```

**Wire up `d` key on `screenRegistries`:** After the `Add` key check:
```go
if key.Matches(msg, keys.Delete) && len(a.registries.entries) > 0 && !a.registryOpInProgress {
    entry := a.registries.entries[a.cardCursor]
    body := fmt.Sprintf("Remove registry %q?\n\nThis deletes the local clone.\nInstalled content is not affected.", entry.name)
    a.modal = newConfirmModal("Remove Registry", body)
    a.modal.purpose = modalRegistryRemove
    a.focus = focusModal
    return a, nil
}
```

**Wire up `r` key on `screenRegistries`:** After the `Delete` key check:
```go
if key.Matches(msg, keys.Refresh) && len(a.registries.entries) > 0 && !a.registryOpInProgress {
    entry := a.registries.entries[a.cardCursor]
    a.statusMessage = fmt.Sprintf("Syncing %s...", entry.name)
    a.registryOpInProgress = true
    return a, func() tea.Msg {
        err := registry.Sync(entry.name)
        return registrySyncDoneMsg{name: entry.name, err: err}
    }
}
```

**Add `doRegistryAdd` helper to `app.go`:**

**Note:** Set an in-progress status before dispatching the async clone so the user sees feedback immediately. Also set `a.registryOpInProgress = true` to guard against double-actions (see double-action prevention below).

```go
// doRegistryAdd starts an async registry clone and config save.
func (a *App) doRegistryAdd(gitURL, nameOverride string) tea.Cmd {
    a.statusMessage = fmt.Sprintf("Adding registry: %s...", gitURL)
    a.registryOpInProgress = true
    root := a.catalog.RepoRoot
    cfg := a.registryCfg
    return func() tea.Msg {
        name := nameOverride
        if name == "" {
            name = registry.NameFromURL(gitURL)
        }
        if !catalog.IsValidRegistryName(name) {
            return registryAddDoneMsg{err: fmt.Errorf("invalid registry name %q", name)}
        }
        // Check for duplicate
        for _, r := range cfg.Registries {
            if r.Name == name {
                return registryAddDoneMsg{err: fmt.Errorf("registry %q already exists", name)}
            }
        }
        if err := registry.Clone(gitURL, name, ""); err != nil {
            return registryAddDoneMsg{name: name, err: err}
        }
        cfg.Registries = append(cfg.Registries, config.Registry{Name: name, URL: gitURL})
        if err := config.Save(root, cfg); err != nil {
            // Best-effort cleanup (B3 fix)
            dir, _ := registry.CloneDir(name)
            os.RemoveAll(dir)
            return registryAddDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
        }
        return registryAddDoneMsg{name: name}
    }
}
```

### 4.4 — Handle registry action result messages in `App.Update()`

**File:** `cli/internal/tui/app.go`
**Depends on:** 4.3

Add result message handlers in the `switch msg := msg.(type)` block (near the other async result handlers like `importDoneMsg`):

```go
case registryAddDoneMsg:
    a.registryOpInProgress = false // unblock double-action guard
    if msg.err != nil {
        a.statusMessage = fmt.Sprintf("Add failed: %s", msg.err)
    } else {
        a.statusMessage = fmt.Sprintf("Added registry: %s", msg.name)
        // Rebuild registry config and rescan
        cfg, err := config.Load(a.catalog.RepoRoot)
        if err == nil {
            a.registryCfg = cfg
        }
        a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
        a.registries.width = a.width - sidebarWidth - 1
        a.registries.height = a.panelHeight()
        a.sidebar.registryCount = len(a.registryCfg.Registries)
        cat, scanErr := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
        if scanErr == nil {
            a.catalog = cat
            a.refreshSidebarCounts()
        }
    }
    return a, nil

case registryRemoveDoneMsg:
    a.registryOpInProgress = false // unblock double-action guard
    if msg.err != nil {
        a.statusMessage = fmt.Sprintf("Remove failed: %s", msg.err)
    } else {
        a.statusMessage = fmt.Sprintf("Removed registry: %s", msg.name)
        cfg, err := config.Load(a.catalog.RepoRoot)
        if err == nil {
            a.registryCfg = cfg
        }
        a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
        a.registries.width = a.width - sidebarWidth - 1
        a.registries.height = a.panelHeight()
        a.sidebar.registryCount = len(a.registryCfg.Registries)
        if a.cardCursor >= len(a.registries.entries) && a.cardCursor > 0 {
            a.cardCursor--
        }
        cat, scanErr := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
        if scanErr == nil {
            a.catalog = cat
            a.refreshSidebarCounts()
        }
    }
    return a, nil

case registrySyncDoneMsg:
    a.registryOpInProgress = false // unblock double-action guard
    if msg.err != nil {
        a.statusMessage = fmt.Sprintf("Sync failed for %s: %s", msg.name, msg.err)
    } else {
        a.statusMessage = fmt.Sprintf("Synced: %s", msg.name)
        // Rescan after sync so item counts update
        cat, scanErr := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
        if scanErr == nil {
            a.catalog = cat
            a.refreshSidebarCounts()
        }
        a.registries = newRegistriesModel(a.catalog.RepoRoot, a.registryCfg, a.catalog)
        a.registries.width = a.width - sidebarWidth - 1
        a.registries.height = a.panelHeight()
    }
    return a, nil
```

**Wire up `modalRegistryRemove` in `handleConfirmAction()`:**
```go
case modalRegistryRemove:
    a.registryOpInProgress = true // set before dispatching async remove
    a.statusMessage = fmt.Sprintf("Removing registry: %s...", a.registries.entries[a.cardCursor].name)
    name := a.registries.entries[a.cardCursor].name
    root := a.catalog.RepoRoot
    cfg := a.registryCfg
    return func() tea.Msg {
        // Remove from config
        var filtered []config.Registry
        for _, r := range cfg.Registries {
            if r.Name != name {
                filtered = append(filtered, r)
            }
        }
        cfg.Registries = filtered
        if err := config.Save(root, cfg); err != nil {
            return registryRemoveDoneMsg{name: name, err: fmt.Errorf("saving config: %w", err)}
        }
        if err := registry.Remove(name); err != nil {
            return registryRemoveDoneMsg{name: name, err: fmt.Errorf("removing clone: %w", err)}
        }
        return registryRemoveDoneMsg{name: name}
    }
```

### 4.5 — Also wire `a` key from registry items view (for `l`)

**File:** `cli/internal/tui/app.go`
**Depends on:** 4.3

In `case screenItems:`, add handling for the `CreateLoadout` key (`l`) when viewing registry items. This is the "create loadout scoped to registry" entry point — deferred to Phase 7. For now, add the key check with a `// TODO: Phase 7` comment so we know where to hook it:

```go
// In case screenItems, after the ToggleHidden handler:
if key.Matches(msg, keys.CreateLoadout) && a.items.sourceRegistry != "" {
    // TODO(Phase 7): open create-loadout wizard scoped to this registry
    a.statusMessage = "Create loadout wizard coming soon"
    return a, nil
}
```

### 4.6 — Regenerate golden files

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/tui/ -update-golden
```

**Run:** `make test`

**Commit:** `feat(tui): registry add/remove/sync actions with modals`

---

## Phase 5: Add Wizard Pre-filtering

### 5.1 — Add pre-filter support to `importModel`

**File:** `cli/internal/tui/import.go`
**Depends on:** Phase 2

The `importModel` needs to accept an optional pre-filter that skips wizard steps the caller already knows. The cleanest approach: add optional fields to the model that, when set, skip their corresponding steps. `newImportModel` keeps its existing signature for the sidebar "Add" path; a new constructor variant handles the pre-filtered path.

**Add to `importModel` struct** (after the existing fields):
```go
// Pre-filter fields: when set, the corresponding wizard step is skipped.
preFilterType     catalog.ContentType // when non-zero, skip stepType
preFilterRegistry string              // when set, scope to this registry's items
```

**Add a new constructor** after `newImportModel`:
```go
// newImportModelWithFilter creates an importModel pre-filtered for a specific
// context. Set typeFilter to skip the type selection step; set registryFilter
// to scope the source to a specific registry.
func newImportModelWithFilter(
    providers []provider.Provider,
    repoRoot, projectRoot string,
    typeFilter catalog.ContentType,
    registryFilter string,
) importModel {
    m := newImportModel(providers, repoRoot, projectRoot)
    m.preFilterType = typeFilter
    m.preFilterRegistry = registryFilter
    if typeFilter != "" {
        // Pre-select the content type and advance past stepType.
        m.contentType = typeFilter
        // Find the index of typeFilter in m.types so typeCursor is consistent.
        for i, t := range m.types {
            if t == typeFilter {
                m.typeCursor = i
                break
            }
        }
        // Start at stepSource (the user still picks Local/Git/Create).
        // stepType will be auto-skipped in the step advance logic.
    }
    return m
}
```

**Update step transition logic in `importModel.Update`:** When the wizard would normally advance to `stepType`, check if `preFilterType` is set and skip to the next step. Find the section that sets `m.step = stepType` and add a guard:

```go
// Helper: returns the next step after stepSource, respecting pre-filters.
func (m importModel) nextStepFromSource() importStep {
    if m.preFilterType != "" {
        // Type is already known — skip to stepProvider or stepBrowseStart
        // depending on source selection.
        return m.nextStepAfterType()
    }
    return stepType
}

func (m importModel) nextStepAfterType() importStep {
    // Same logic as what currently happens when the user confirms stepType.
    // Extracted here so pre-filtering can call it.
    switch m.sourceCursor {
    case 0: // Local path
        if m.contentType.IsProviderSpecific() {
            return stepProvider
        }
        return stepBrowseStart
    case 1: // Git URL
        return stepGitURL
    case 2: // Create new
        return stepName
    }
    return stepType
}
```

There are 5 locations where `m.step = stepType` is assigned in `import.go`. Only the ones in `updateSource()` (the Enter handler for stepSource) need the pre-filter bypass:

- **Line 366** (`case 1: // Local path`): change to `m.step = m.nextStepFromSource()`
- **Line 375** (`case 3: // Create New`): change to `m.step = m.nextStepFromSource()`

The other 3 locations are "go back" transitions (Esc from later steps returning to stepType) and should NOT be changed — going back should always go back to the type step, even when pre-filtered:
- **Line 424** (`updateProvider` → Esc back): keep as `m.step = stepType` but guard with `if m.preFilterType != "" { m.step = stepSource } else { m.step = stepType }`
- **Line 445** (`updateBrowseStart` → Esc back, universal type): same guard
- **Line 1189** (other back navigation): audit individually

### 5.2 — Wire contextual `a` on `screenItems`

**File:** `cli/internal/tui/app.go`
**Depends on:** 5.1

In `case screenItems:`, after the `ToggleHidden` handler and before the `Enter` handler, add:

```go
if key.Matches(msg, keys.Add) {
    ct := a.items.contentType
    regFilter := a.items.sourceRegistry
    // SearchResults and Library don't have a single type — use unfiltered wizard.
    if ct == catalog.SearchResults || ct == catalog.Library {
        ct = ""
    }
    a.importer = newImportModelWithFilter(a.providers, a.catalog.RepoRoot, a.projectRoot, ct, regFilter)
    a.importer.width = a.width - sidebarWidth - 1
    a.importer.height = a.panelHeight()
    a.screen = screenImport
    a.focus = focusContent
    return a, nil
}
```

**Also add `a` key on `screenLibraryCards`** (after the `Back` handler in that case block):
```go
if key.Matches(msg, keys.Add) {
    // Library card grid: no type pre-filter, but destination will be library.
    a.importer = newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)
    a.importer.width = a.width - sidebarWidth - 1
    a.importer.height = a.panelHeight()
    a.screen = screenImport
    a.focus = focusContent
    return a, nil
}
```

**Also add `a` key on `screenLoadoutCards`** (after the `Back` handler):
```go
if key.Matches(msg, keys.Add) {
    // TODO(Phase 7): open create-loadout wizard (full, with provider picker).
    a.statusMessage = "Create loadout wizard coming soon"
    return a, nil
}
```

### 5.3 — Update help text on `screenItems` and `screenLibraryCards`

**File:** `cli/internal/tui/items.go`
**Depends on:** 5.2

Find the `renderHelp()` method on `itemsModel`. Add `• a add` to the help string. Confirm the exact current string and splice it in consistently.

**Run:** `make test` (golden files will need updating)

```bash
go test ./internal/tui/ -update-golden
make test
```

**Commit:** `feat(tui): contextual a key on items and library card screens`

---

## Phase 6: Loadouts — Show All Detected Providers

### 6.1 — Update `loadoutCardProviders()` to return all detected providers

**File:** `cli/internal/tui/app.go`
**Depends on:** nothing (standalone improvement)

Find the `loadoutCardProviders()` method. Currently it returns only providers that have loadout items. Update it to return ALL detected providers (those where `Detected == true` in `a.providers`), using a stable order (AllProviders order).

**Replace `loadoutCardProviders()`:**
```go
// loadoutCardProviders returns providers to show on the loadouts card screen.
// Shows all detected providers — even those with no loadouts — so the grid
// is always populated and the user can create loadouts for any provider.
func (a App) loadoutCardProviders() []string {
    // Build a set of providers that have loadout items.
    hasLoadouts := make(map[string]bool)
    for _, item := range a.catalog.ByType(catalog.Loadouts) {
        if item.Provider != "" {
            hasLoadouts[item.Provider] = true
        }
    }
    // Return all detected providers in AllProviders order.
    var result []string
    for _, prov := range a.providers {
        if prov.Detected {
            result = append(result, prov.Slug)
        }
    }
    // Fall back: if no providers detected, use those with loadout content.
    if len(result) == 0 {
        for slug := range hasLoadouts {
            result = append(result, slug)
        }
        sort.Strings(result)
    }
    return result
}
```

### 6.2 — Update loadout card rendering to show "No loadouts" label for empty providers

**File:** Find where loadout cards are rendered in `app.go` (search for `loadout-card-`) — this is in `App.View()`.

Currently the card view only renders providers with loadouts. After 6.1, it renders all detected providers. The card render must show a "no loadouts" indicator for providers where `!hasLoadouts[prov]`.

Find the loadout card rendering block in `App.View()` and update to pass a count (0 for providers without loadouts) and a "no loadouts" label:

```go
case screenLoadoutCards:
    provs := a.loadoutCardProviders()
    // ... (existing card layout loop) ...
    // When rendering each card, pass itemCount = 0 and noLoadouts = true
    // for providers with no loadouts.
```

The card rendering is inline in `renderLoadoutCards()` at `app.go:1995`. The `renderCard` closure uses `provCounts[prov]` which will be 0 for providers without loadouts. Update the inner rendering to show a "No loadouts" label when count is 0:

**In `renderLoadoutCards()`**, replace the `renderCard` closure (lines ~1995-2003):
```go
renderCard := func(idx int, prov string) string {
    count := provCounts[prov]
    var inner string
    if count > 0 {
        inner = labelStyle.Render(prov) + " " + countStyle.Render(fmt.Sprintf("(%d)", count))
        inner += "\n" + helpStyle.Render("Loadouts for "+prov)
    } else {
        inner = labelStyle.Render(prov)
        inner += "\n" + helpStyle.Render("No loadouts")
    }

    style := cardBase.BorderForeground(borderColor)
    if idx == a.cardCursor {
        style = cardBase.BorderForeground(accentColor)
    }
    return zone.Mark(fmt.Sprintf("loadout-card-%s", prov), style.Render(inner))
}
```

### 6.3 — Add `a` key on `screenLoadoutCards` to open Create Loadout wizard stub

Already wired in Phase 5.2 with a `statusMessage` placeholder. This phase confirms the placeholder is in place. The actual wizard is Phase 7.

**Run:** `make test`

```bash
go test ./internal/tui/ -update-golden
make test
```

**Commit:** `feat(tui): loadout card grid shows all detected providers`

---

## Phase 7: Create Loadout Wizard

### 7.1 — Create `loadout_create.go`

**File:** `cli/internal/tui/loadout_create.go` (new file)
**Depends on:** Phase 2, Phase 6

This is the largest new component. Follow the multi-step wizard pattern from `modal.go` (`envSetupModal`, `installModal`). Fixed modal dimensions: `width = 64`, `height = 24`.

```go
package tui

import (
    "fmt"
    "strings"

    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    zone "github.com/lrstanley/bubblezone"
    overlay "github.com/rmhubbert/bubbletea-overlay"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
)

type createLoadoutStep int

const (
    clStepProvider  createLoadoutStep = iota // pick provider (skipped if pre-filled)
    clStepItems                              // checkbox list of items
    clStepName                               // name + description text inputs
    clStepDest                               // pick destination
)

const (
    createLoadoutModalWidth  = 64
    createLoadoutModalHeight = 24
)

// loadoutItemEntry is one item in the checkbox picker.
type loadoutItemEntry struct {
    item     catalog.ContentItem
    selected bool
}

type createLoadoutModal struct {
    active       bool
    step         createLoadoutStep
    totalSteps   int

    // Context passed in at creation
    prefilledProvider string // non-empty = skip provider step
    scopeRegistry     string // non-empty = scope items to this registry

    // Step 1: provider picker
    providerList   []provider.Provider
    providerCursor int

    // Step 2: item checkbox list
    entries     []loadoutItemEntry
    itemCursor  int
    searchText  string
    searchInput textinput.Model

    // Step 3: name/desc
    nameInput textinput.Model
    descInput textinput.Model
    nameFirst bool // true = nameInput focused

    // Step 4: destination
    destOptions []string // "Project", "Library", optionally "Registry: <name>"
    destCursor  int
    destDisabled []bool  // parallel: true = option grayed out
    destHints    []string // parallel: explanation for disabled options

    message      string
    messageIsErr bool

    width, height int
}

// newCreateLoadoutModal creates a new Create Loadout wizard.
// prefilledProvider: if set, skips provider step.
// scopeRegistry: if set, restricts item list to that registry's content.
// allProviders: full list (detected or not) for the provider picker.
// cat: catalog to pull items from.
func newCreateLoadoutModal(
    prefilledProvider string,
    scopeRegistry string,
    allProviders []provider.Provider,
    cat *catalog.Catalog,
) createLoadoutModal {
    si := textinput.New()
    si.Placeholder = "filter items..."
    si.CharLimit = 100

    ni := textinput.New()
    ni.Prompt = labelStyle.Render("Name: ")
    ni.Placeholder = "my-loadout"
    ni.CharLimit = 100
    ni.Focus()

    di := textinput.New()
    di.Prompt = labelStyle.Render("Description: ")
    di.Placeholder = "What this loadout does"
    di.CharLimit = 300

    m := createLoadoutModal{
        active:            true,
        prefilledProvider: prefilledProvider,
        scopeRegistry:     scopeRegistry,
        providerList:      allProviders,
        searchInput:       si,
        nameInput:         ni,
        descInput:         di,
        nameFirst:         true,
    }

    // Build destination options
    m.destOptions = []string{"Project (loadouts/ in repo)", "Library (~/.syllago/content/loadouts/)"}
    m.destDisabled = []bool{false, false}
    m.destHints = []string{"", ""}
    if scopeRegistry != "" {
        m.destOptions = append(m.destOptions, fmt.Sprintf("Registry: %s", scopeRegistry))
        m.destDisabled = append(m.destDisabled, false) // will be updated dynamically
        m.destHints = append(m.destHints, "")
    }

    // Build item entries
    m.entries = buildLoadoutItemEntries(cat, scopeRegistry)

    // Determine starting step
    if prefilledProvider != "" {
        m.step = clStepItems
    } else {
        m.step = clStepProvider
    }

    // Total steps
    if prefilledProvider != "" {
        m.totalSteps = 3
    } else {
        m.totalSteps = 4
    }

    return m
}

// buildLoadoutItemEntries collects catalog items for the picker.
// If scopeRegistry is non-empty, only items from that registry are included.
func buildLoadoutItemEntries(cat *catalog.Catalog, scopeRegistry string) []loadoutItemEntry {
    var entries []loadoutItemEntry
    for _, item := range cat.Items {
        if scopeRegistry != "" && item.Registry != scopeRegistry {
            continue
        }
        entries = append(entries, loadoutItemEntry{item: item, selected: false})
    }
    return entries
}

// currentStep returns a 1-based step number accounting for skipped steps.
func (m createLoadoutModal) currentStepNum() int {
    if m.prefilledProvider != "" {
        // Steps: 1=Items, 2=Name, 3=Dest
        switch m.step {
        case clStepItems:
            return 1
        case clStepName:
            return 2
        case clStepDest:
            return 3
        }
    }
    // Steps: 1=Provider, 2=Items, 3=Name, 4=Dest
    return int(m.step) + 1
}

func (m createLoadoutModal) Update(msg tea.Msg) (createLoadoutModal, tea.Cmd) {
    if !m.active {
        return m, nil
    }
    switch msg := msg.(type) {
    case tea.KeyMsg:
        m.message = "" // clear on any key

        switch m.step {
        case clStepProvider:
            switch {
            case msg.Type == tea.KeyEsc:
                m.active = false
                return m, nil
            case key.Matches(msg, keys.Up):
                if m.providerCursor > 0 {
                    m.providerCursor--
                }
            case key.Matches(msg, keys.Down):
                if m.providerCursor < len(m.providerList)-1 {
                    m.providerCursor++
                }
            case msg.Type == tea.KeyEnter:
                m.prefilledProvider = m.providerList[m.providerCursor].Slug
                m.step = clStepItems
            }

        case clStepItems:
            if m.searchInput.Focused() {
                // Search input active — route keys to it, Esc exits search
                if msg.Type == tea.KeyEsc {
                    m.searchInput.Blur()
                    m.searchInput.SetValue("")
                    return m, nil
                }
                var cmd tea.Cmd
                m.searchInput, cmd = m.searchInput.Update(msg)
                return m, cmd
            }
            switch {
            case msg.Type == tea.KeyEsc:
                if m.prefilledProvider != "" {
                    // Was pre-filled: dismiss entirely
                    m.active = false
                } else {
                    m.step = clStepProvider
                }
                return m, nil
            case key.Matches(msg, keys.Up):
                if m.itemCursor > 0 {
                    m.itemCursor--
                }
            case key.Matches(msg, keys.Down):
                filtered := m.filteredEntries()
                if m.itemCursor < len(filtered)-1 {
                    m.itemCursor++
                }
            case key.Matches(msg, keys.Space):
                filtered := m.filteredEntries()
                if m.itemCursor < len(filtered) {
                    // Toggle selection in the original entries slice
                    targetItem := filtered[m.itemCursor].item
                    for i, e := range m.entries {
                        if e.item.Path == targetItem.Path {
                            m.entries[i].selected = !m.entries[i].selected
                            break
                        }
                    }
                    m.updateDestConstraints()
                }
            case msg.String() == "/":
                m.searchInput.Focus()
            case msg.Type == tea.KeyEnter:
                m.step = clStepName
            }

        case clStepName:
            switch {
            case msg.Type == tea.KeyEsc:
                m.step = clStepItems
                return m, nil
            case msg.Type == tea.KeyTab:
                if m.nameFirst {
                    m.nameFirst = false
                    m.nameInput.Blur()
                    m.descInput.Focus()
                } else {
                    m.nameFirst = true
                    m.descInput.Blur()
                    m.nameInput.Focus()
                }
                return m, nil
            case msg.Type == tea.KeyEnter:
                if strings.TrimSpace(m.nameInput.Value()) == "" {
                    m.message = "Name is required"
                    m.messageIsErr = true
                    return m, nil
                }
                m.step = clStepDest
                return m, nil
            }
            var cmd tea.Cmd
            if m.nameFirst {
                m.nameInput, cmd = m.nameInput.Update(msg)
            } else {
                m.descInput, cmd = m.descInput.Update(msg)
            }
            return m, cmd

        case clStepDest:
            switch {
            case msg.Type == tea.KeyEsc:
                m.step = clStepName
                return m, nil
            case key.Matches(msg, keys.Up):
                if m.destCursor > 0 {
                    m.destCursor--
                    // Skip disabled options
                    for m.destCursor > 0 && m.destDisabled[m.destCursor] {
                        m.destCursor--
                    }
                }
            case key.Matches(msg, keys.Down):
                if m.destCursor < len(m.destOptions)-1 {
                    m.destCursor++
                    for m.destCursor < len(m.destOptions)-1 && m.destDisabled[m.destCursor] {
                        m.destCursor++
                    }
                }
            case msg.Type == tea.KeyEnter:
                if m.destDisabled[m.destCursor] {
                    return m, nil
                }
                // Confirmed — close modal; App.Update handles the result.
                m.active = false
                return m, nil
            }
        }
    }
    return m, nil
}

// filteredEntries returns entries matching the current search text.
func (m createLoadoutModal) filteredEntries() []loadoutItemEntry {
    query := strings.ToLower(m.searchInput.Value())
    if query == "" {
        return m.entries
    }
    var out []loadoutItemEntry
    for _, e := range m.entries {
        if strings.Contains(strings.ToLower(e.item.Name), query) ||
            strings.Contains(strings.ToLower(e.item.Description), query) {
            out = append(out, e)
        }
    }
    return out
}

// selectedItems returns all entries where selected == true.
func (m createLoadoutModal) selectedItems() []loadoutItemEntry {
    var out []loadoutItemEntry
    for _, e := range m.entries {
        if e.selected {
            out = append(out, e)
        }
    }
    return out
}

// updateDestConstraints grays out the "Registry" option when selected items
// span multiple registries or multiple providers.
func (m *createLoadoutModal) updateDestConstraints() {
    if len(m.destOptions) < 3 {
        return // no registry option
    }
    selected := m.selectedItems()
    registries := make(map[string]bool)
    providers := make(map[string]bool)
    for _, e := range selected {
        if e.item.Registry != "" {
            registries[e.item.Registry] = true
        }
        if e.item.Provider != "" {
            providers[e.item.Provider] = true
        }
    }
    regIdx := 2 // "Registry" is the third option
    if len(registries) > 1 {
        m.destDisabled[regIdx] = true
        m.destHints[regIdx] = "Items span multiple registries"
    } else if len(providers) > 1 {
        m.destDisabled[regIdx] = true
        m.destHints[regIdx] = "Items target multiple providers"
    } else {
        m.destDisabled[regIdx] = false
        m.destHints[regIdx] = ""
    }
    // If registry dest is now disabled and cursor is on it, move cursor up.
    if m.destDisabled[regIdx] && m.destCursor == regIdx {
        m.destCursor--
    }
}

func (m createLoadoutModal) View() string {
    stepLabel := fmt.Sprintf("(%d of %d)", m.currentStepNum(), m.totalSteps)
    title := titleStyle.Render("Create Loadout") + "  " + helpStyle.Render(stepLabel)
    var body string

    switch m.step {
    case clStepProvider:
        body = labelStyle.Render("Pick a provider:") + "\n\n"
        for i, prov := range m.providerList {
            prefix := "  "
            style := itemStyle
            if i == m.providerCursor {
                prefix = "> "
                style = selectedItemStyle
            }
            detected := ""
            if prov.Detected {
                detected = " " + installedStyle.Render("(detected)")
            }
            body += fmt.Sprintf("  %s%s%s\n", prefix, style.Render(prov.Name), detected)
        }

    case clStepItems:
        filtered := m.filteredEntries()
        searchLine := m.searchInput.View()
        body = searchLine + "\n\n"
        if len(filtered) == 0 {
            body += helpStyle.Render("  No items found.")
        }
        innerH := createLoadoutModalHeight - 8 // title + search + padding
        shown := filtered
        if len(shown) > innerH {
            start := m.itemCursor - innerH/2
            if start < 0 {
                start = 0
            }
            if start+innerH > len(shown) {
                start = len(shown) - innerH
            }
            shown = shown[start:min(start+innerH, len(shown))]
        }
        for i, e := range shown {
            checkBox := "[ ]"
            if e.selected {
                checkBox = "[x]"
            }
            prefix := "  "
            style := itemStyle
            absIdx := i
            if len(shown) < len(filtered) {
                absIdx = m.itemCursor - innerH/2 + i
                if absIdx < 0 {
                    absIdx = 0
                }
            }
            if absIdx == m.itemCursor {
                prefix = "> "
                style = selectedItemStyle
            }
            source := ""
            if e.item.Registry != "" {
                source = helpStyle.Render(" (" + e.item.Registry + ")")
            } else if e.item.Library {
                source = helpStyle.Render(" (library)")
            }
            typeLabel := helpStyle.Render("[" + string(e.item.Type) + "]")
            body += fmt.Sprintf("  %s%s %s %s%s\n",
                prefix,
                helpStyle.Render(checkBox),
                typeLabel,
                style.Render(e.item.Name),
                source,
            )
        }
        body += "\n" + helpStyle.Render("space select • / filter • enter next")

    case clStepName:
        body = labelStyle.Render("Name your loadout:") + "\n\n"
        body += m.nameInput.View() + "\n"
        body += m.descInput.View() + "\n"
        body += "\n" + helpStyle.Render("tab switch field • enter next")
        if m.message != "" {
            if m.messageIsErr {
                body += "\n" + errorMsgStyle.Render(m.message)
            }
        }

    case clStepDest:
        body = labelStyle.Render("Choose destination:") + "\n\n"
        for i, opt := range m.destOptions {
            prefix := "  "
            style := itemStyle
            if m.destDisabled[i] {
                style = helpStyle
            } else if i == m.destCursor {
                prefix = "> "
                style = selectedItemStyle
            }
            body += fmt.Sprintf("  %s%s\n", prefix, style.Render(opt))
            if m.destDisabled[i] && m.destHints[i] != "" {
                body += "      " + helpStyle.Render(m.destHints[i]) + "\n"
            }
        }
        body += "\n" + helpStyle.Render("enter confirm • esc back")
    }

    content := title + "\n\n" + body
    return lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(modalBorderColor).
        Background(modalBgColor).
        Padding(1, 2).
        Width(createLoadoutModalWidth).
        Height(createLoadoutModalHeight).
        Render(content)
}

func (m createLoadoutModal) overlayView(background string) string {
    return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}
```

**Add `min` helper** if it doesn't exist in the package (Go 1.21+ has it built-in; check the Go version in `go.mod`). If the version is pre-1.21:
```go
func min(a, b int) int {
    if a < b { return a }
    return b
}
```

### 7.2 — Add `createLoadoutModal` field to `App` and wire up

**File:** `cli/internal/tui/app.go`
**Depends on:** 7.1

**Add field to `App` struct:**
```go
createLoadoutModal createLoadoutModal
```

**Wire modal input routing** (in the `if a.modal.active` chain):
```go
if a.createLoadoutModal.active {
    var cmd tea.Cmd
    a.createLoadoutModal, cmd = a.createLoadoutModal.Update(msg)
    if !a.createLoadoutModal.active {
        a.focus = focusContent
        if a.createLoadoutModal.nameInput.Value() != "" {
            return a, a.doCreateLoadout(a.createLoadoutModal)
        }
    }
    return a, cmd
}
```

**Wire overlay rendering** in `App.View()`:
```go
if a.createLoadoutModal.active {
    return zone.Scan(a.createLoadoutModal.overlayView(base))
}
```

**Replace the Phase 5.2 placeholder on `screenLoadoutCards`:**
```go
if key.Matches(msg, keys.Add) {
    a.createLoadoutModal = newCreateLoadoutModal("", "", a.providers, a.catalog)
    a.focus = focusModal
    return a, nil
}
```

**On `screenLoadoutCards`, also handle `a` after drilling into a provider's loadout list** — this is the `screenItems` case when `a.cardParent == screenLoadoutCards`.

**Important:** `parentLabel` holds "Loadouts" (the breadcrumb text), NOT the provider slug. We need a new field on `itemsModel` to carry the provider slug through the drill-in.

**First, add field to `itemsModel` in `items.go`** (after `sourceRegistry`):
```go
sourceProvider string // provider slug when drilled in from loadout cards
```

**Then, in the loadout card Enter handler (`app.go` ~line 1238)**, set it:
```go
items := newItemsModel(catalog.Loadouts, filtered, a.providers, a.catalog.RepoRoot)
items.sourceProvider = prov  // <-- add this line
items.parentLabel = "Loadouts"
```

**Now the `a` key handler in `case screenItems:` can use `sourceProvider`:**
```go
// In case screenItems, a key handler — update the existing handler:
if key.Matches(msg, keys.Add) {
    ct := a.items.contentType
    regFilter := a.items.sourceRegistry
    if ct == catalog.SearchResults || ct == catalog.Library {
        ct = ""
    }
    // If we came from a loadout card drill-in, open create loadout wizard.
    if a.cardParent == screenLoadoutCards && a.items.sourceProvider != "" {
        a.createLoadoutModal = newCreateLoadoutModal(a.items.sourceProvider, regFilter, a.providers, a.catalog)
        a.focus = focusModal
        return a, nil
    }
    // Otherwise: normal add wizard
    a.importer = newImportModelWithFilter(a.providers, a.catalog.RepoRoot, a.projectRoot, ct, regFilter)
    a.importer.width = a.width - sidebarWidth - 1
    a.importer.height = a.panelHeight()
    a.screen = screenImport
    a.focus = focusContent
    return a, nil
}
```

**Replace Phase 4.5 placeholder on `screenItems` for the `l` key (registry scope):**
```go
if key.Matches(msg, keys.CreateLoadout) && a.items.sourceRegistry != "" {
    a.createLoadoutModal = newCreateLoadoutModal("", a.items.sourceRegistry, a.providers, a.catalog)
    a.focus = focusModal
    return a, nil
}
```

### 7.3 — Add `doCreateLoadout` helper

**File:** `cli/internal/tui/app.go`
**Depends on:** 7.2

```go
// doCreateLoadoutMsg is sent when the loadout create operation completes.
type doCreateLoadoutMsg struct {
    name string
    err  error
}

// doCreateLoadout writes a loadout.yaml to the chosen destination.
func (a *App) doCreateLoadout(m createLoadoutModal) tea.Cmd {
    return func() tea.Msg {
        name := strings.TrimSpace(m.nameInput.Value())
        desc := strings.TrimSpace(m.descInput.Value())
        provSlug := m.prefilledProvider

        // Build manifest
        manifest := loadoutManifest{
            Kind:        "loadout",
            Version:     1,
            Provider:    provSlug,
            Name:        name,
            Description: desc,
        }
        for _, e := range m.selectedItems() {
            switch e.item.Type {
            case catalog.Rules:
                manifest.Rules = append(manifest.Rules, e.item.Name)
            case catalog.Hooks:
                manifest.Hooks = append(manifest.Hooks, e.item.Name)
            case catalog.Skills:
                manifest.Skills = append(manifest.Skills, e.item.Name)
            case catalog.Agents:
                manifest.Agents = append(manifest.Agents, e.item.Name)
            case catalog.MCP:
                manifest.MCP = append(manifest.MCP, e.item.Name)
            case catalog.Commands:
                manifest.Commands = append(manifest.Commands, e.item.Name)
            }
        }

        // Determine destination directory
        var destDir string
        switch m.destCursor {
        case 0: // Project
            destDir = filepath.Join(a.projectRoot, "loadouts", provSlug)
        case 1: // Library
            home, _ := os.UserHomeDir()
            destDir = filepath.Join(home, ".syllago", "content", "loadouts", provSlug)
        case 2: // Registry
            dir, err := registry.CloneDir(m.scopeRegistry)
            if err != nil {
                return doCreateLoadoutMsg{err: err}
            }
            destDir = filepath.Join(dir, "loadouts", provSlug)
        }

        if err := os.MkdirAll(destDir, 0755); err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("creating loadout dir: %w", err)}
        }

        itemDir := filepath.Join(destDir, name)
        if err := os.MkdirAll(itemDir, 0755); err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("creating item dir: %w", err)}
        }

        data, err := yaml.Marshal(manifest)
        if err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("marshaling manifest: %w", err)}
        }

        outPath := filepath.Join(itemDir, "loadout.yaml")
        if err := os.WriteFile(outPath, data, 0644); err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("writing loadout.yaml: %w", err)}
        }

        return doCreateLoadoutMsg{name: name}
    }
}
```

**Add import:** `"gopkg.in/yaml.v3"` (already in go.mod via other packages).

**Add type alias for the manifest struct** (to avoid importing the loadout package just for YAML marshaling; or import the package — check whether `loadout.Manifest` has `yaml` tags, which it does per `manifest.go`):

Instead of defining a local struct, import `loadout` and use `loadout.Manifest`:
```go
import "github.com/OpenScribbler/syllago/cli/internal/loadout"
// ...
manifest := loadout.Manifest{
    Kind:    "loadout",
    Version: 1,
    // ...
}
```

**Handle `doCreateLoadoutMsg` in `App.Update()`:**
```go
case doCreateLoadoutMsg:
    if msg.err != nil {
        a.statusMessage = fmt.Sprintf("Create loadout failed: %s", msg.err)
    } else {
        a.statusMessage = fmt.Sprintf("Created loadout: %s", msg.name)
        cat, err := catalog.ScanWithGlobalAndRegistries(a.catalog.RepoRoot, a.projectRoot, a.registrySources)
        if err == nil {
            a.catalog = cat
            a.refreshSidebarCounts()
        }
    }
    return a, nil
```

### 7.4 — Add tests for `createLoadoutModal` step logic

**File:** `cli/internal/tui/loadout_create_test.go` (new)
**Depends on:** 7.1

```go
package tui

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestCreateLoadoutModalSteps(t *testing.T) {
    cat := &catalog.Catalog{}
    providers := []provider.Provider{
        {Name: "Claude Code", Slug: "claude-code", Detected: true},
    }

    t.Run("starts at provider step when no prefill", func(t *testing.T) {
        m := newCreateLoadoutModal("", "", providers, cat)
        if m.step != clStepProvider {
            t.Errorf("step = %v, want clStepProvider", m.step)
        }
        if m.totalSteps != 4 {
            t.Errorf("totalSteps = %d, want 4", m.totalSteps)
        }
    })

    t.Run("skips provider step when pre-filled", func(t *testing.T) {
        m := newCreateLoadoutModal("claude-code", "", providers, cat)
        if m.step != clStepItems {
            t.Errorf("step = %v, want clStepItems", m.step)
        }
        if m.totalSteps != 3 {
            t.Errorf("totalSteps = %d, want 3", m.totalSteps)
        }
    })

    t.Run("dest constraint: registry disabled when items span registries", func(t *testing.T) {
        m := newCreateLoadoutModal("", "reg-a", providers, cat)
        // Manually add two entries from different registries and select both.
        m.entries = []loadoutItemEntry{
            {item: catalog.ContentItem{Name: "a", Registry: "reg-a"}, selected: true},
            {item: catalog.ContentItem{Name: "b", Registry: "reg-b"}, selected: true},
        }
        // Add a registry destination option (normally done in newCreateLoadoutModal with scopeRegistry)
        m.destOptions = append(m.destOptions, "Registry: reg-a")
        m.destDisabled = append(m.destDisabled, false)
        m.destHints = append(m.destHints, "")
        m.updateDestConstraints()
        if !m.destDisabled[2] {
            t.Error("registry dest should be disabled when items span registries")
        }
    })
}
```

**Run:** `make test`

**Commit:** `feat(tui): Create Loadout wizard (multi-step modal)`

---

## Phase 8: Smart Registry Add

This phase adds provider-aware scanning for non-syllago repos. It has two parts: the backend scanning logic and the CLI flow. TUI integration is deferred (it requires a new screen flow that's complex enough to be its own PR).

### 8.1 — Add `NativeContentScan` to the catalog package

**File:** `cli/internal/catalog/native_scan.go` (new file)
**Depends on:** Phase 1.3 (registry naming uses org/repo)

This function scans a directory for provider-native content formats. It returns a summary of what was found without doing any conversion.

```go
package catalog

import (
    "os"
    "path/filepath"
)

// NativeProviderContent holds found content for one provider.
type NativeProviderContent struct {
    ProviderSlug string
    ProviderName string
    // Discovered files grouped by type label (e.g. "rules", "skills").
    ByType map[string][]string // type label → file paths
}

// NativeScanResult holds provider-native content found in a directory.
type NativeScanResult struct {
    Providers []NativeProviderContent
    // HasSyllagoStructure is true if the directory looks like a syllago registry
    // (has registry.yaml or has standard content type directories).
    HasSyllagoStructure bool
}

// ScanNativeContent scans dir for provider-native AI tool content.
// Only scans for providers that syllago supports. Returns findings grouped
// by provider. Does not read file contents — path existence only.
func ScanNativeContent(dir string) NativeScanResult {
    var result NativeScanResult

    // Check for registry.yaml first.
    if _, err := os.Stat(filepath.Join(dir, "registry.yaml")); err == nil {
        result.HasSyllagoStructure = true
        return result
    }

    // Check for syllago content dirs.
    for _, ct := range AllContentTypes() {
        if _, err := os.Stat(filepath.Join(dir, string(ct))); err == nil {
            result.HasSyllagoStructure = true
            return result
        }
    }

    // Scan for provider-native patterns.
    patterns := providerNativePatterns()
    seen := make(map[string]*NativeProviderContent)

    for _, p := range patterns {
        fullPath := filepath.Join(dir, p.path)
        info, err := os.Stat(fullPath)
        if err != nil {
            continue
        }
        pc, ok := seen[p.providerSlug]
        if !ok {
            seen[p.providerSlug] = &NativeProviderContent{
                ProviderSlug: p.providerSlug,
                ProviderName: p.providerName,
                ByType:       make(map[string][]string),
            }
            pc = seen[p.providerSlug]
        }
        if info.IsDir() {
            // List files in the directory and group them.
            entries, err := os.ReadDir(fullPath)
            if err != nil {
                continue
            }
            for _, e := range entries {
                if !e.IsDir() {
                    pc.ByType[p.typeLabel] = append(pc.ByType[p.typeLabel], filepath.Join(p.path, e.Name()))
                }
            }
        } else {
            pc.ByType[p.typeLabel] = append(pc.ByType[p.typeLabel], p.path)
        }
    }

    for _, pc := range seen {
        if len(pc.ByType) > 0 {
            result.Providers = append(result.Providers, *pc)
        }
    }
    return result
}

// nativePattern describes one provider-native path to check.
type nativePattern struct {
    providerSlug string
    providerName string
    path         string // relative to repo root
    typeLabel    string // e.g. "rules", "skills"
}

// providerNativePatterns returns all known provider-native content paths.
// This is the canonical list of what syllago can find in a non-registry repo.
func providerNativePatterns() []nativePattern {
    return []nativePattern{
        // Claude Code
        {providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/commands", typeLabel: "commands"},
        {providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/skills", typeLabel: "skills"},
        {providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/agents", typeLabel: "agents"},
        {providerSlug: "claude-code", providerName: "Claude Code", path: "CLAUDE.md", typeLabel: "rules"},
        // Gemini CLI
        {providerSlug: "gemini-cli", providerName: "Gemini CLI", path: ".gemini", typeLabel: "config"},
        // Cursor
        {providerSlug: "cursor", providerName: "Cursor", path: ".cursorrules", typeLabel: "rules"},
        {providerSlug: "cursor", providerName: "Cursor", path: ".cursor/rules", typeLabel: "rules"},
        // Windsurf
        {providerSlug: "windsurf", providerName: "Windsurf", path: ".windsurfrules", typeLabel: "rules"},
        // Codex
        {providerSlug: "codex", providerName: "Codex", path: ".codex", typeLabel: "config"},
        // Copilot CLI
        {providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".github/copilot-instructions.md", typeLabel: "rules"},
        // Zed
        {providerSlug: "zed", providerName: "Zed", path: ".zed", typeLabel: "settings"},
        // Cline
        {providerSlug: "cline", providerName: "Cline", path: ".clinerules", typeLabel: "rules"},
        // Roo Code
        {providerSlug: "roo-code", providerName: "Roo Code", path: ".roo", typeLabel: "config"},
        // OpenCode
        {providerSlug: "opencode", providerName: "OpenCode", path: ".opencode", typeLabel: "config"},
        // Kiro
        {providerSlug: "kiro", providerName: "Kiro", path: ".kiro", typeLabel: "config"},
    }
}
```

### 8.2 — Add tests for `ScanNativeContent`

**File:** `cli/internal/catalog/native_scan_test.go` (new)
**Depends on:** 8.1

```go
package catalog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestScanNativeContent_SyllagoStructure(t *testing.T) {
    dir := t.TempDir()
    // Create a registry.yaml
    os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("name: test"), 0644)
    result := ScanNativeContent(dir)
    if !result.HasSyllagoStructure {
        t.Error("expected HasSyllagoStructure=true when registry.yaml present")
    }
}

func TestScanNativeContent_CursorRules(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
    result := ScanNativeContent(dir)
    if result.HasSyllagoStructure {
        t.Error("should not be syllago structure")
    }
    if len(result.Providers) != 1 {
        t.Fatalf("expected 1 provider, got %d", len(result.Providers))
    }
    if result.Providers[0].ProviderSlug != "cursor" {
        t.Errorf("expected cursor, got %s", result.Providers[0].ProviderSlug)
    }
}

func TestScanNativeContent_MultiProvider(t *testing.T) {
    dir := t.TempDir()
    os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
    os.WriteFile(filepath.Join(dir, ".windsurfrules"), []byte("# rules"), 0644)
    result := ScanNativeContent(dir)
    if len(result.Providers) != 2 {
        t.Fatalf("expected 2 providers, got %d", len(result.Providers))
    }
}

func TestScanNativeContent_Empty(t *testing.T) {
    dir := t.TempDir()
    result := ScanNativeContent(dir)
    if result.HasSyllagoStructure || len(result.Providers) != 0 {
        t.Error("expected empty result for empty directory")
    }
}
```

### 8.3 — Update `registry add` CLI command with non-syllago detection

**File:** `cli/cmd/syllago/registry_cmd.go`
**Depends on:** 8.1, Phase 1.3

After the clone succeeds and before saving to config, replace the existing "warn but don't fail" content check with the smart detection flow:

**Replace** the block that checks `hasDirs`:

```go
// Smart detection: check if this is a proper syllago registry.
dir, _ := registry.CloneDir(name)
scanResult := catalog.ScanNativeContent(dir)

if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
    // Not a syllago registry, but has provider-native content.
    fmt.Fprintf(output.ErrWriter, "\nThis repo doesn't appear to be a syllago registry.\n")
    fmt.Fprintf(output.ErrWriter, "Found provider-native content:\n\n")
    for _, pc := range scanResult.Providers {
        fmt.Fprintf(output.ErrWriter, "  %s:\n", pc.ProviderName)
        for typeLabel, files := range pc.ByType {
            fmt.Fprintf(output.ErrWriter, "    %s: %d file(s)\n", typeLabel, len(files))
        }
    }
    fmt.Fprintf(output.ErrWriter, "\nThis content cannot be added as a registry (registries require syllago format).\n")
    fmt.Fprintf(output.ErrWriter, "To add this content to your library, use: syllago add <path> (coming soon)\n")
    // Clean up the clone — it's not a registry.
    os.RemoveAll(dir)
    return fmt.Errorf("not a syllago registry — clone removed")
} else if !scanResult.HasSyllagoStructure && len(scanResult.Providers) == 0 {
    fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain any recognized content. Added anyway.\n", name)
}
```

**Explicitly deferred scope:** The design's CLI flow specifies an interactive `[y/N/select]` prompt with per-provider/per-type selection, followed by conversion and copy to `~/.syllago/content/`. This PR intentionally implements only the **detection + informative error** half. The interactive selection and conversion pipeline are deferred to a follow-up PR (the TUI drill-down flow for non-registry repos is also deferred there). The "coming soon" hint in the output is deliberate. The deferred scope is not a bug — Phase 8 delivers the scaffolding (`ScanNativeContent`, the detection call site, tests) that the follow-up PR builds on.

### 8.4 — Add tests for `registry add` with native content detection

**File:** `cli/cmd/syllago/registry_cmd_test.go` (add to existing if it exists, or create)
**Depends on:** 8.3

This requires mocking the git clone and the scan. Since the command directly calls `registry.Clone`, test the `ScanNativeContent` detection in isolation (already covered by 8.2). For the CLI integration, add a note in the test file that the end-to-end flow requires a real git server — skip in unit tests.

**Run:** `make test`

**Commit:** `feat: smart registry add detects non-syllago repos with provider-native content`

---

## Phase 9: Cross-Cutting Polish

### 9.1 — Terminology audit: find "Import" in user-facing strings

**File:** multiple
**Depends on:** nothing

Search for user-facing "Import" strings (not code identifiers):

```bash
grep -rn '".*[Ii]mport.*"' cli/internal/tui/ --include="*.go"
grep -rn '".*[Ii]mport.*"' cli/cmd/ --include="*.go"
```

Review each hit. Internal names (`importModel`, `screenImport`, `importer`) are fine. User-facing strings in breadcrumbs, labels, or help text that say "Import" should say "Add". Change any that are user-visible.

**Specific known location:** Check the breadcrumb in `import.go`'s `View()` method. The brainstorm doc says breadcrumbs already say "Add" — verify this is true.

**Test:** `make test` with golden file update if any string changed.

### 9.2 — Help text audit

**Depends on:** Phase 4, 5, 7

Audit all `helpText()` / `renderHelp()` methods across TUI components. Each screen must include every available key action. The format is `"key action • key action"`.

**Exact changes:**

1. `registriesModel.helpText()` (`registries.go:93`):
   - Before: `"up/down navigate • enter browse items • esc back"`
   - After: `"up/down navigate • enter browse • a add • d remove • r sync • esc back"`

2. `contextHelpText()` (`app.go:2043`) for `screenLibraryCards, screenLoadoutCards`:
   - Before: `"up/down navigate • enter browse • esc back"`
   - After: split into separate cases:
     - `screenLibraryCards`: `"up/down navigate • enter browse • a add • esc back"`
     - `screenLoadoutCards`: `"up/down navigate • enter browse • a create loadout • esc back"`

3. `contextHelpText()` (`app.go:2031`) for `screenItems`:
   - Before: `"/ search • enter detail • esc back • ? help"`
   - After: `"/ search • enter detail • a add • esc back • ? help"`
   - Conditional: when `a.items.sourceRegistry != ""`, append `" • l create loadout"`

### 9.3 — Key binding audit documentation

**File:** `cli/internal/tui/CLAUDE.md`
**Depends on:** Phase 9.2

Add a section documenting which keys are active on which screens. This prevents future conflicts. Append to the existing Keyboard Handling section:

```
## Active Key Bindings by Screen

| Key | screenCategory | screenItems | screenLibraryCards | screenLoadoutCards | screenRegistries |
|-----|---------------|-------------|-------------------|-------------------|-----------------|
| a   | —             | add         | add               | create loadout    | add registry    |
| d   | —             | —           | —                 | —                 | remove registry |
| r   | —             | —           | —                 | —                 | sync registry   |
| l   | —             | create loadout (registry context only) | — | — | — |
| H   | toggle hidden | toggle hidden | — | — | — |
| i   | —             | —           | —                 | —                 | —               |
```

### 9.4 — Post-action state refresh: verify all new actions do it

**File:** `cli/internal/tui/app.go`
**Depends on:** Phase 4, 7, 8

Review each new async action's done-handler and confirm it calls:
1. `a.catalog = cat` after rescan
2. `a.refreshSidebarCounts()`
3. Rebuilds the current screen model (e.g., `a.registries = newRegistriesModel(...)`)
4. Sets a `statusMessage` for user feedback

Checklist:
- `registryAddDoneMsg` handler — done in Phase 4.4
- `registryRemoveDoneMsg` handler — done in Phase 4.4
- `registrySyncDoneMsg` handler — done in Phase 4.4
- `doCreateLoadoutMsg` handler — done in Phase 7.3

If any are missing the catalog rescan, add it.

### 9.5 — Regenerate all golden files and final test run

```bash
cd /home/hhewett/.local/src/syllago/cli
go test ./internal/tui/ -update-golden
cd ..
make test
```

Review the full golden file diff to confirm all visual changes are intentional.

**Commit:** `chore(tui): cross-cutting polish — help text, terminology, key binding docs`

---

## Phase Summary and Dependencies

```
Phase 1 (Bug fixes)     → independent; do first
Phase 2 (Key bindings)  → independent; do after Phase 1
Phase 3 (Registry cards)→ depends on Phase 2
Phase 4 (Registry actions) → depends on Phase 3
Phase 5 (Add pre-filter)→ depends on Phase 2
Phase 6 (Loadout providers) → depends on Phase 2
Phase 7 (Create Loadout wizard) → depends on Phase 5, 6
Phase 8 (Smart registry add) → depends on Phase 1.3
Phase 9 (Cross-cutting) → depends on all prior phases
```

Parallelizable after Phase 2: Phases 3, 5, and 6 can proceed independently.
Phase 8 can be worked on any time after Phase 1.3.

---

## Files Changed Summary

| File | Changes |
|------|---------|
| `cli/cmd/syllago/registry_cmd.go` | B1 security notice, B3 orphan cleanup, B2 name validation, Phase 8.3 smart detection |
| `cli/internal/registry/registry.go` | B2 `NameFromURL` org/repo format |
| `cli/internal/catalog/scanner.go` | B2 add `IsValidRegistryName` |
| `cli/internal/catalog/native_scan.go` | new: `ScanNativeContent` and patterns |
| `cli/internal/catalog/native_scan_test.go` | new: tests for native scan |
| `cli/internal/tui/keys.go` | add `Add`, `Delete`, `Refresh`, `CreateLoadout`; remove `l` from `Right` |
| `cli/internal/tui/registries.go` | replace table with card view (pure render, no cursor) |
| `cli/internal/tui/styles.go` | add `cardNormalStyle`, `cardSelectedStyle` |
| `cli/internal/tui/app.go` | card nav for registries; `a`/`d`/`r`/`l` handlers; new message types; modal fields; `doRegistryAdd`; `doCreateLoadout`; post-action refresh |
| `cli/internal/tui/modal.go` | add `registryAddModal`; add `modalRegistryRemove` purpose |
| `cli/internal/tui/import.go` | add `preFilterType`, `preFilterRegistry`; `newImportModelWithFilter` |
| `cli/internal/tui/loadout_create.go` | new: `createLoadoutModal` multi-step wizard |
| `cli/internal/tui/loadout_create_test.go` | new: wizard step logic tests |
| `cli/internal/tui/CLAUDE.md` | key binding audit table |
| `cli/internal/tui/testdata/*.golden` | regenerated after visual changes |

---

## Commit Sequence

1. `fix: security notice mentions prompts, should be commands`
2. `fix: clean up orphaned clone when config save fails after registry add`
3. `feat: registry names use owner/repo format (NameFromURL, IsValidRegistryName)`
4. `feat(tui/keys): add a/d/r/l key bindings; remove l as Right alias`
5. `feat(tui): replace registry table with card grid`
6. `feat(tui): registry add/remove/sync actions with modals`
7. `feat(tui): contextual a key on items and library card screens`
8. `feat(tui): loadout card grid shows all detected providers`
9. `feat(tui): Create Loadout wizard (multi-step modal)`
10. `feat: smart registry add detects non-syllago repos with provider-native content`
11. `chore(tui): cross-cutting polish — help text, terminology, key binding docs`
