# Loadout Wizard Redesign — Implementation Plan

**Design doc:** `docs/plans/2026-03-12-loadout-wizard-redesign-design.md`
**Date:** 2026-03-12

---

## How to Read This Plan

Each task is one focused action (2–5 minutes of implementation). Tasks follow a TDD rhythm:

1. Write a failing test (or update an existing one)
2. `make test` — confirm it fails
3. Implement the change
4. `make test` — confirm it passes
5. Regenerate goldens if visual output changed: `cd cli && go test ./internal/tui/ -update-golden`
6. Commit

Dependencies are listed explicitly so tasks can be queued in a task tracker. Within a phase, tasks with no dependency on each other can be done in any order.

---

## Phase 1: Action Button Pattern

Goal: add visible, clickable action buttons with semantic colors to every page that has keyboard-only actions today.

---

### Task 1.1 — Add 5 action button styles to styles.go

**File:** `cli/internal/tui/styles.go`

**What to add** — append after the `cardSelectedStyle` block:

```go
// Action buttons (page-level, below breadcrumb)
// Format: [hotkey] Label — chip-style with semantic background colors.
actionBtnAddStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#052E16"}).
    Background(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}).
    Padding(0, 1)

actionBtnRemoveStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#450A0A"}).
    Background(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}).
    Padding(0, 1)

actionBtnUninstallStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#431407"}).
    Background(lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FDBA74"}).
    Padding(0, 1)

actionBtnSyncStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#2E1065"}).
    Background(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}).
    Padding(0, 1)

actionBtnDefaultStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}).
    Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
    Padding(0, 1)
```

**Test:** `cli/internal/tui/styles_test.go` already compiles styles — run `make test` to confirm no compile errors.

**Success criteria:**
- `make test` passes
- Styles are defined and render without panics

**Dependencies:** none

---

### Task 1.2 — Add renderActionButtons() helper to pagehelpers.go

**File:** `cli/internal/tui/pagehelpers.go`

**What to add** — append at the bottom of the file:

```go
// ActionButton describes a single clickable action button.
type ActionButton struct {
    Hotkey  string // single character, e.g. "a"
    Label   string // display text after hotkey, e.g. "Create Loadout"
    ZoneID  string // zone.Mark ID, e.g. "action-a"
    Style   lipgloss.Style
}

// renderActionButtons renders a row of chip-style action buttons below a breadcrumb.
// Each button is formatted as "[key] Label" with a semantic background color.
// Buttons are separated by a single space. The row is preceded and followed by a blank line.
//
// Usage:
//
//	renderActionButtons(
//	    ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle},
//	    ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
//	)
func renderActionButtons(buttons ...ActionButton) string {
    if len(buttons) == 0 {
        return ""
    }
    var parts []string
    for _, btn := range buttons {
        label := "[" + btn.Hotkey + "] " + btn.Label
        rendered := btn.Style.Render(label)
        parts = append(parts, zone.Mark(btn.ZoneID, rendered))
    }
    return "\n" + strings.Join(parts, " ") + "\n"
}
```

**Important:** `pagehelpers.go` must import `zone "github.com/lrstanley/bubblezone"` — it is not currently imported there. Add it alongside the existing imports before implementing `renderActionButtons()`.

**Test** — add to `cli/internal/tui/pagehelpers_test.go`:

```go
func TestRenderActionButtons(t *testing.T) {
    t.Run("empty buttons returns empty string", func(t *testing.T) {
        got := renderActionButtons()
        if got != "" {
            t.Errorf("empty buttons should return empty string, got %q", got)
        }
    })
    t.Run("single button contains hotkey and label", func(t *testing.T) {
        got := renderActionButtons(ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle})
        if !strings.Contains(got, "[a]") {
            t.Error("button should contain hotkey")
        }
        if !strings.Contains(got, "Create Loadout") {
            t.Error("button should contain label")
        }
    })
    t.Run("multiple buttons are separated by space", func(t *testing.T) {
        got := renderActionButtons(
            ActionButton{"a", "Add", "action-a", actionBtnAddStyle},
            ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
        )
        if !strings.Contains(got, "[a]") || !strings.Contains(got, "[r]") {
            t.Error("both buttons should appear")
        }
    })
}
```

**Commands:**
```
cd cli && make test
# Expected: TestRenderActionButtons PASS
```

**Success criteria:**
- All three subtests pass
- Function signature matches design spec

**Dependencies:** Task 1.1

---

### Task 1.3 — Add action buttons to loadout card page

> **Note:** The mouse wiring in this task (and Tasks 1.4–1.7) temporarily opens the **old modal** (`newCreateLoadoutModal`). This is intentional — Phase 1 is rendering-only groundwork. Task 2.7 will replace all `newCreateLoadoutModal` calls with the new `createLoadoutScreen`. Until then, clicking the button works but opens the old flow.

**File:** `cli/internal/tui/app.go` — `renderLoadoutCards()` method

**What to change:** After the breadcrumb line (`renderBreadcrumb(...)`), insert the action button row.

Locate `renderLoadoutCards()` (search for `func (a App) renderLoadoutCards()`). After the breadcrumb render, add:

```go
s += renderActionButtons(
    ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle},
) + "\n"
```

**Mouse wiring** — in the `tea.MouseMsg` left-click handler (around line 1434 in app.go, the `if a.screen == screenLoadoutCards` block), add before the existing card-click checks:

```go
if zone.Get("action-a").InBounds(msg) {
    a.createLoadoutModal = newCreateLoadoutModal("", "", a.providers, a.catalog)
    a.focus = focusModal
    return a, nil
}
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
git diff cli/internal/tui/testdata/ | grep "^+" | grep -c "Create Loadout"
# Should show the button appearing in loadout card goldens
```

**Success criteria:**
- `make test` passes (golden files updated)
- `[a] Create Loadout` button visible in loadout card golden output
- Clicking "action-a" zone opens the modal (test in `TestCreateLoadoutModal` or existing mouse test)

**Dependencies:** Task 1.2

---

### Task 1.4 — Add action buttons to loadout items list

> **Note:** Mouse wiring here temporarily opens the old modal. Task 2.7 updates this to the new screen.

**File:** `cli/internal/tui/items.go` — `View()` method

Find the section where the breadcrumb is rendered for loadouts items. After the breadcrumb, when `m.contentType == catalog.Loadouts`, add the action button row:

```go
if m.contentType == catalog.Loadouts {
    s += renderActionButtons(
        ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle},
    ) + "\n"
}
```

**Mouse wiring** — in `app.go`, within the `case screenItems:` left-click block, add:

```go
if a.screen == screenItems && a.items.contentType == catalog.Loadouts {
    if zone.Get("action-a").InBounds(msg) {
        provider := a.items.sourceProvider
        a.createLoadoutModal = newCreateLoadoutModal(provider, a.items.sourceRegistry, a.providers, a.catalog)
        a.focus = focusModal
        return a, nil
    }
}
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- `[a] Create Loadout` button visible in loadout items list golden output
- Not visible on non-loadout items screens

**Dependencies:** Task 1.2

---

### Task 1.5 — Add action buttons to library card page

**File:** `cli/internal/tui/app.go` — `renderLibraryCards()` method

After the breadcrumb render, add:

```go
s += renderActionButtons(
    ActionButton{"a", "Add Content", "action-a", actionBtnAddStyle},
) + "\n"
```

**Mouse wiring** — in the `tea.MouseMsg` left-click handler for `screenLibraryCards`:

```go
if a.screen == screenLibraryCards {
    if zone.Get("action-a").InBounds(msg) {
        a.importer = newImportModel(a.providers, a.catalog.RepoRoot, a.projectRoot)
        a.importer.width = a.width - sidebarWidth - 1
        a.importer.height = a.panelHeight()
        a.screen = screenImport
        a.focus = focusContent
        return a, nil
    }
}
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- `[a] Add Content` button visible in library cards golden output

**Dependencies:** Task 1.2

---

### Task 1.6 — Add action buttons to library items list

**File:** `cli/internal/tui/items.go` — `View()` method

When source is library:

> **Important:** `catalog.Library` is NOT a valid ContentType constant — do not use it as a type check. Library context is identified via `m.hideLibraryBadge` (set to true when the items list was entered from the Library card page). The first code block below is illustrative and will not compile — use only the second code block.

For non-library, non-loadout items lists that come from a library source, show `[a] Add {Type}` and `[r] Remove`:

```go
// Items from a library context (hideLibraryBadge is set when parentLabel is "Library")
if m.hideLibraryBadge {
    typeLabel := m.contentType.Label()
    s += renderActionButtons(
        ActionButton{"a", "Add " + typeLabel, "action-a", actionBtnAddStyle},
        ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
    ) + "\n"
} else if m.contentType != catalog.Loadouts && m.contentType != catalog.SearchResults {
    typeLabel := m.contentType.Label()
    s += renderActionButtons(
        ActionButton{"a", "Add " + typeLabel, "action-a", actionBtnAddStyle},
    ) + "\n"
}
```

**Mouse wiring** — in `app.go`, within the `case screenItems:` left-click block, add zone checks for `action-a` and `action-r` that synthesize `keys.Add` and `keys.Delete` key messages:

```go
if a.screen == screenItems {
    if zone.Get("action-a").InBounds(msg) {
        return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
    }
    if zone.Get("action-r").InBounds(msg) {
        return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
    }
}
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- Library items list shows both `[a]` and `[r]` buttons
- Non-library items list shows only `[a] Add {Type}`
- Clicking action-r on library items list opens the remove confirmation

**Dependencies:** Task 1.2

---

### Task 1.7 — Add action buttons to registries page

**File:** `cli/internal/tui/registries.go` — `View()` method

After the breadcrumb render:

```go
s += renderActionButtons(
    ActionButton{"a", "Add Registry", "action-a", actionBtnAddStyle},
    ActionButton{"s", "Sync", "action-s", actionBtnSyncStyle},
    ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
) + "\n"
```

**Mouse wiring** — in `app.go`, in the `screenRegistries` left-click section:

```go
case screenRegistries:
    if zone.Get("action-a").InBounds(msg) {
        return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
    }
    if zone.Get("action-s").InBounds(msg) {
        return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
    }
    if zone.Get("action-r").InBounds(msg) {
        return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
    }
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- All three buttons appear in registries golden output
- Clicking action-a opens the registry add modal

**Dependencies:** Task 1.2

---

### Task 1.8 — Add action buttons to detail view

**File:** `cli/internal/tui/detail_render.go` — `renderInstallTab()` method

The detail view already has `detail-btn-install`, `detail-btn-uninstall`, and `detail-btn-copy` as individual `zone.Mark()` calls (around line 361–371 in `detail_render.go`). Replace those inline button renders with the new action button system.

**How the existing flags work:** The current code in `renderInstallTab()` builds buttons conditionally:
- Install and Uninstall are always shown (no flag — the user picks which to act on via the checkbox list)
- Copy is shown only when `m.item.Library` is true
- Save, Env Setup, Share don't exist yet as buttons — they're keyboard-only today

The new code adds all context-dependent buttons using the action button system:

```go
// Actions section header
s += "\n"
s += labelStyle.Render("Actions") + "\n"
s += helpStyle.Render(strings.Repeat("─", 20)) + "\n"

// Action buttons — built from item context
var actionBtns []ActionButton
actionBtns = append(actionBtns, ActionButton{"i", "Install", "detail-btn-install", actionBtnAddStyle})
actionBtns = append(actionBtns, ActionButton{"u", "Uninstall", "detail-btn-uninstall", actionBtnUninstallStyle})
if m.item.Library {
    actionBtns = append(actionBtns, ActionButton{"c", "Copy", "detail-btn-copy", actionBtnDefaultStyle})
}
if m.item.Library {
    actionBtns = append(actionBtns, ActionButton{"s", "Save", "detail-btn-save", actionBtnDefaultStyle})
}
if m.item.Type == catalog.MCP && m.hasUnsetEnvVars() {
    actionBtns = append(actionBtns, ActionButton{"e", "Env Setup", "detail-btn-env", actionBtnDefaultStyle})
}
if m.item.Library {
    actionBtns = append(actionBtns, ActionButton{"p", "Share", "detail-btn-share", actionBtnDefaultStyle})
}
s += renderActionButtons(actionBtns...) + "\n"
```

**Zone IDs are unchanged** (`detail-btn-install`, `detail-btn-uninstall`, `detail-btn-copy`, etc.) — the existing mouse wiring in `app.go` (around line 1502–1513) already handles these zones and continues to work without modification.

**Mouse wiring already exists in app.go:**
```go
// Already wired in app.go (no change needed):
if a.screen == screenDetail {
    btnChars := map[string]string{
        "detail-btn-install":   "i",
        "detail-btn-uninstall": "u",
        "detail-btn-copy":      "c",
        "detail-btn-save":      "s",
    }
    for zoneID, char := range btnChars {
        if zone.Get(zoneID).InBounds(msg) {
            // ... synthesizes key message
        }
    }
}
```

If `detail-btn-env` and `detail-btn-share` are new zone IDs not yet in that map, add them to the `btnChars` map in `app.go`:
```go
"detail-btn-env":       "e",
"detail-btn-share":     "p",
```

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- Detail view goldens show restyled action buttons using semantic colors
- Existing zone IDs unchanged; mouse clicks still work via existing wiring in `app.go`

**Dependencies:** Task 1.2

---

### Task 1.8.5 — Add action buttons to sandbox page

**File:** `cli/internal/tui/sandbox_settings.go` — `View()` method

The sandbox page currently has no action buttons. Its available actions are: add an item to a list (Enter), delete the last item (`d`). The design doc lists "Sandbox" in the pages-that-get-action-buttons table.

**What actions to expose:**

Looking at `sandbox_settings.go`, the active actions when `editMode == 0` are:
- Enter / Space — open the add-item editor for the focused row
- `d` — delete the last item from the focused row
- `s` — save

Add two action buttons:

```go
// After the breadcrumb in View(), before the row list:
s += renderActionButtons(
    ActionButton{"a", "Add", "sandbox-btn-add", actionBtnAddStyle},
    ActionButton{"d", "Delete Last", "sandbox-btn-delete", actionBtnRemoveStyle},
) + "\n"
```

**Mouse wiring** — `sandboxSettingsModel.Update()` already handles `tea.MouseMsg` for row clicks. Add button zone checks there:

```go
case tea.MouseMsg:
    if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
        // Existing row click handling...
        for i := 0; i < sandboxRowCount; i++ {
            if zone.Get(fmt.Sprintf("sandbox-row-%d", i)).InBounds(msg) {
                m.cursor = i
                m.editMode = i + 1
                m.editInput = ""
                return m, nil
            }
        }
        // New: action button clicks
        if zone.Get("sandbox-btn-add").InBounds(msg) {
            m.editMode = m.cursor + 1
            m.editInput = ""
            return m, nil
        }
        if zone.Get("sandbox-btn-delete").InBounds(msg) {
            m.deleteSelected()
            m.save()
            return m, nil
        }
    }
```

**Note:** The sandbox page is wired through `App` — no changes needed in `app.go`'s mouse handler since `sandboxSettingsModel.Update()` handles its own mouse events.

**Golden test update:**
```
cd cli && go test ./internal/tui/ -update-golden
```

**Success criteria:**
- `make test` passes
- Sandbox golden output shows `[a] Add` and `[d] Delete Last` buttons
- Clicking Add starts the add-item editor for the currently focused row
- Clicking Delete Last removes the last item from the focused row

**Dependencies:** Task 1.2

---

### Task 1.9 — Golden baseline tests for all action button pages at all 4 sizes

**File:** `cli/internal/tui/golden_sizes_test.go` (or appropriate golden test file)

Add test cases that navigate to each page affected by action buttons and capture golden output at 60x20, 80x30, 120x40, and 160x50:

```go
func TestActionButtonGoldens(t *testing.T) {
    sizes := []struct{ w, h int }{{60, 20}, {80, 30}, {120, 40}, {160, 50}}
    pages := []struct {
        name     string
        navigate func(app App) App
    }{
        {"loadout-cards", func(app App) App {
            // navigate to screenLoadoutCards
            nTypes := sidebarContentCount()
            app = pressN(app, keyDown, nTypes+1) // Loadouts in sidebar
            m, _ := app.Update(keyEnter)
            return m.(App)
        }},
        {"library-cards", func(app App) App {
            nTypes := sidebarContentCount()
            app = pressN(app, keyDown, nTypes) // Library
            m, _ := app.Update(keyEnter)
            return m.(App)
        }},
        {"registries", func(app App) App {
            // navigate to screenRegistries via sidebar
            // NOTE: app.sidebar.registriesIdx() does not exist — use key-press navigation instead.
            // The registries entry is in the sidebar after all content types and Library/Loadouts.
            // Navigate down to the registries sidebar item and press Enter.
            // Verify the correct position by checking sidebarContentCount() + offset for Registries.
            idx := sidebarContentCount() + 3 // adjust based on actual sidebar layout
            app = pressN(app, keyDown, idx)
            m, _ := app.Update(keyEnter)
            return m.(App)
        }},
    }

    for _, sz := range sizes {
        for _, pg := range pages {
            t.Run(fmt.Sprintf("%s-%dx%d", pg.name, sz.w, sz.h), func(t *testing.T) {
                app := testAppSize(t, sz.w, sz.h)
                app = pg.navigate(app)
                got := app.View()
                requireGolden(t, fmt.Sprintf("action-btn-%s-%dx%d", pg.name, sz.w, sz.h), got)
            })
        }
    }
}
```

**Commands:**
```
cd cli && go test ./internal/tui/ -update-golden
make test
```

**Success criteria:**
- All golden files created for all sizes and pages
- `make test` passes on second run (goldens match)

**Dependencies:** 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.8.5

---

## Phase 2: Wizard Screen (Replace Modal)

Goal: replace `createLoadoutModal` with a full-screen `createLoadoutScreen` using a persistent split-view layout.

---

### Task 2.1 — Add screenCreateLoadout to the screen enum

**File:** `cli/internal/tui/app.go`

Add to the `screen` const block:

```go
const (
    screenCategory screen = iota
    screenItems
    screenDetail
    screenImport
    screenUpdate
    screenSettings
    screenRegistries
    screenSandbox
    screenLibraryCards
    screenLoadoutCards
    screenCreateLoadout // new
)
```

**Test:** Compile-only check — no new test needed yet (no behavior change).

```
cd cli && make build
```

**Success criteria:**
- `make build` succeeds (no compile errors)

**Dependencies:** Phase 1 complete (or can be done independently)

---

### Task 2.2 — Define createLoadoutScreen struct

**File:** `cli/internal/tui/loadout_create.go`

Add the new model alongside the existing `createLoadoutModal`. The new model holds the same wizard state but adds split-view and preview fields:

```go
// createLoadoutScreen is the full-screen replacement for createLoadoutModal.
// Renders in the content pane (sidebar visible) with a persistent split-view.
type createLoadoutScreen struct {
    step createLoadoutStep

    // Context passed in at creation
    prefilledProvider string
    scopeRegistry     string

    // Step 1: provider picker
    providerList   []provider.Provider
    providerCursor int

    // Step 2: content type selection
    typeEntries []typeCheckEntry
    typeCursor  int

    // Step 3: per-type item selection
    entries        []loadoutItemEntry
    selectedTypes  []catalog.ContentType
    typeStepIndex  int
    typeItemMap    map[catalog.ContentType][]int
    typeItemMapAll map[catalog.ContentType][]int
    showAllCompat  bool
    perTypeCursor  map[catalog.ContentType]int
    perTypeScroll  map[catalog.ContentType]int
    perTypeSearch  map[catalog.ContentType]string
    searchActive   bool
    searchInput    textinput.Model

    // Step 4: name/description
    nameInput textinput.Model
    descInput textinput.Model
    nameFirst bool

    // Step 5: destination
    destOptions  []string
    destCursor   int
    destDisabled []bool
    destHints    []string

    // Step 6: review
    reviewItemCursor int // cursor over the item list in review step
    reviewBtnCursor  int // 0=Back, 1=Create

    // Split-view preview
    splitView      splitViewModel
    previewLoading bool

    // Outcome
    confirmed    bool
    message      string
    messageIsErr bool

    // Layout
    width  int
    height int
}
```

**Also add** the constructor `newCreateLoadoutScreen()`:

```go
func newCreateLoadoutScreen(
    prefilledProvider string,
    scopeRegistry string,
    allProviders []provider.Provider,
    cat *catalog.Catalog,
    width, height int,
) createLoadoutScreen {
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

    m := createLoadoutScreen{
        prefilledProvider: prefilledProvider,
        scopeRegistry:     scopeRegistry,
        providerList:      allProviders,
        searchInput:       si,
        nameInput:         ni,
        descInput:         di,
        nameFirst:         true,
        perTypeCursor:     make(map[catalog.ContentType]int),
        perTypeScroll:     make(map[catalog.ContentType]int),
        perTypeSearch:     make(map[catalog.ContentType]string),
        typeItemMap:       make(map[catalog.ContentType][]int),
        typeItemMapAll:    make(map[catalog.ContentType][]int),
        width:             width,
        height:            height,
    }

    m.destOptions = []string{"Project (loadouts/ in repo)", "Library (~/.syllago/content/loadouts/)"}
    m.destDisabled = []bool{false, false}
    m.destHints = []string{"", ""}
    if scopeRegistry != "" {
        m.destOptions = append(m.destOptions, fmt.Sprintf("Registry: %s", scopeRegistry))
        m.destDisabled = append(m.destDisabled, false)
        m.destHints = append(m.destHints, "")
    }

    m.entries = buildLoadoutItemEntries(cat, scopeRegistry)
    m.splitView = newSplitView(nil, "wiz")
    m.splitView.width = width
    m.splitView.height = height - 5 // reserve space for header + help bar

    if prefilledProvider != "" {
        m.buildTypeEntries()
        m.step = clStepTypes
    } else {
        m.step = clStepProvider
    }

    return m
}
```

**Also migrate** the helper methods from `createLoadoutModal` to `createLoadoutScreen` — copy and rename the receiver: `buildTypeEntries`, `buildTypeItemMaps`, `currentTypeItems`, `currentType`, `currentTypeSelectedCount`, `isItemCompatible`, `filteredTypeItems`, `selectedItems`, `updateDestConstraints`, `currentStepNum`, `dynamicTotalSteps`.

**New vs migrated — explicit inventory:**

| Symbol | Status | Notes |
|--------|--------|-------|
| `createLoadoutScreen` struct | NEW | replaces `createLoadoutModal` |
| `newCreateLoadoutScreen()` | NEW | replaces `newCreateLoadoutModal()` |
| `typeCheckEntry` | MIGRATED | copy from modal, receiver change only |
| `loadoutItemEntry` | MIGRATED | copy from modal, receiver change only |
| `buildLoadoutItemEntries()` | MIGRATED | copy from modal |
| `buildTypeEntries()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `buildTypeItemMaps()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `currentTypeItems()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `currentType()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `currentTypeSelectedCount()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `isItemCompatible()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `filteredTypeItems()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `selectedItems()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `updateDestConstraints()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `currentStepNum()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `dynamicTotalSteps()` | MIGRATED | copy, receiver → `createLoadoutScreen` |
| `previewCmdForCursor()` | NEW | emits `splitViewCursorMsg` for preview loading |
| `renderLeftPane()` | NEW | extracts modal View() body without border/padding |
| `renderSplitView()` | NEW | composes left + right panes |
| `renderSplitTitleBar()` | NEW | "Items \| Preview" tab-style title bar |
| `renderReviewLeftPane()` | NEW | review step left pane with navigable item list |
| `reviewItems()` | NEW | flat list of selected items for review step |
| `previewCmdForReviewCursor()` | NEW | emits cursor msg from review step |

**Zone ID conventions for this screen** — all wizard zones use the `wiz-*` namespace:

| Element | Zone ID pattern | Example |
|---------|----------------|---------|
| List/radio options (provider, types, items, dest) | `wiz-opt-{index}` | `wiz-opt-0` |
| Text input fields | `wiz-field-{name}` | `wiz-field-name`, `wiz-field-desc`, `wiz-field-search` |
| Review step item list | `wiz-review-{index}` | `wiz-review-0` |
| Review step buttons | `wiz-btn-{action}` | `wiz-btn-back`, `wiz-btn-create` |

**Destination hint text:** The empty hint strings (`""`) are intentional for Project and Library — they are self-explanatory options. For Registry, the hint is also empty here; Task 2.4's `renderLeftPane()` renders the dest options with an inline description if the hint is non-empty. If future UX review wants a hint like `"loadout saved to registry clone directory"`, update `m.destHints[2]` in the constructor. For now, empty is acceptable.

**`updateDestConstraints()` call:** The constructor calls `m.entries = buildLoadoutItemEntries(...)` then returns without calling `m.updateDestConstraints()`. This is correct — constraints are evaluated lazily when the user reaches the destination step. `updateDestConstraints()` is called from `Update()` when items are selected/deselected.

**Test:** Add one smoke test to `cli/internal/tui/loadout_create_test.go`:

```go
func TestCreateLoadoutScreenSmoke(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)

    t.Run("starts at types step when pre-filled", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        if s.step != clStepTypes {
            t.Errorf("step = %v, want clStepTypes", s.step)
        }
    })
    t.Run("starts at provider step when no prefill", func(t *testing.T) {
        s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
        if s.step != clStepProvider {
            t.Errorf("step = %v, want clStepProvider", s.step)
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- Both subtests pass
- `createLoadoutScreen` struct and constructor compile without errors

**Dependencies:** Task 2.1

---

### Task 2.3 — Implement createLoadoutScreen.Update() — keyboard input

**File:** `cli/internal/tui/loadout_create.go`

Port the keyboard handling from `createLoadoutModal.Update()` to `createLoadoutScreen.Update()`. The logic is identical; only the receiver type changes. Key differences from the modal:

- No `active` field check at the top (screens are always active when shown)
- Return type is `(createLoadoutScreen, tea.Cmd)` not `(createLoadoutModal, tea.Cmd)`
- On `clStepItems`, when cursor moves, emit a `splitViewCursorMsg` to trigger preview loading:

```go
// In clStepItems, after cursor movement:
case key.Matches(msg, keys.Up):
    if cursor > 0 {
        m.perTypeCursor[ct] = cursor - 1
        return m, m.previewCmdForCursor()
    }
case key.Matches(msg, keys.Down):
    if cursor < len(filtered)-1 {
        m.perTypeCursor[ct] = cursor + 1
        return m, m.previewCmdForCursor()
    }
```

Add the preview command helper:

```go
// previewCmdForCursor emits a splitViewCursorMsg for the current cursor item.
// App uses this to load the primary file preview into the right pane.
func (m createLoadoutScreen) previewCmdForCursor() tea.Cmd {
    ct := m.currentType()
    filtered := m.filteredTypeItems()
    cursor := m.perTypeCursor[ct]
    if cursor < 0 || cursor >= len(filtered) {
        return nil
    }
    idx := filtered[cursor]
    e := m.entries[idx]
    item := splitViewItem{
        Label: e.item.Name,
        Path:  primaryFilePath(e.item),
    }
    return func() tea.Msg {
        return splitViewCursorMsg{index: cursor, item: item}
    }
}
```

Add `primaryFilePath` helper (or reuse an existing one from `detail_fileviewer.go`):

> **Important:** `path/filepath` is not currently imported in `loadout_create.go`. Add `"path/filepath"` to the import block in loadout_create.go before implementing `primaryFilePath`.

```go
// primaryFilePath returns the absolute path to the primary file for an item.
// Falls back to empty string if no files.
func primaryFilePath(item catalog.ContentItem) string {
    if len(item.Files) == 0 || item.Path == "" {
        return ""
    }
    return filepath.Join(item.Path, item.Files[0])
}
```

**Complete key handling for `clStepItems`** — the `Update()` switch for this step must handle all the keys shown in the help text. The cursor movement above is only part of it. The full set:

```go
case clStepItems:
    ct := m.currentType()
    filtered := m.filteredTypeItems()
    cursor := m.perTypeCursor[ct]

    // When search is active, forward all input to the search field.
    // Only Esc exits search mode.
    if m.searchActive {
        if msg.Type == tea.KeyEsc {
            m.searchActive = false
            m.searchInput.Blur()
            // Rebuild filtered list (search text cleared implicitly by the Blur)
            return m, nil
        }
        var cmd tea.Cmd
        m.searchInput, cmd = m.searchInput.Update(msg)
        // Persist search text to perTypeSearch so filteredTypeItems() picks it up
        m.perTypeSearch[ct] = m.searchInput.Value()
        return m, cmd
    }

    switch {
    case key.Matches(msg, keys.Up):
        if cursor > 0 {
            m.perTypeCursor[ct] = cursor - 1
            return m, m.previewCmdForCursor()
        }
    case key.Matches(msg, keys.Down):
        if cursor < len(filtered)-1 {
            m.perTypeCursor[ct] = cursor + 1
            return m, m.previewCmdForCursor()
        }
    case key.Matches(msg, keys.Space):
        if cursor >= 0 && cursor < len(filtered) {
            entryIdx := filtered[cursor]
            if m.isItemCompatible(entryIdx) {
                m.entries[entryIdx].selected = !m.entries[entryIdx].selected
                m.updateDestConstraints()
            }
        }
        return m, nil
    case key.Matches(msg, keys.Add) || msg.String() == "a":
        // Toggle all visible items (each independently)
        for _, fi := range filtered {
            if m.isItemCompatible(fi) {
                m.entries[fi].selected = !m.entries[fi].selected
            }
        }
        m.updateDestConstraints()
        return m, nil
    case msg.String() == "t":
        // Toggle compatibility filter
        m.showAllCompat = !m.showAllCompat
        return m, nil
    case msg.String() == "/":
        // Activate search
        m.searchActive = true
        m.searchInput.Focus()
        return m, nil
    case key.Matches(msg, keys.Right) || msg.String() == "l":
        // Focus preview pane
        m.splitView.focusedPane = panePreview
        return m, nil
    case key.Matches(msg, keys.Left) || msg.String() == "h":
        // Focus list pane
        m.splitView.focusedPane = paneList
        return m, nil
    case msg.Type == tea.KeyEnter:
        // Advance to next type or to name step
        m.typeStepIndex++
        if m.typeStepIndex >= len(m.selectedTypes) {
            m.step = clStepName
        }
        return m, nil
    case msg.Type == tea.KeyEsc:
        if m.typeStepIndex > 0 {
            m.typeStepIndex--
        } else {
            m.step = clStepTypes
        }
        return m, nil
    }
```

**Note on `keys.Add`:** If `keys.Add` is already bound to `a` in `keys.go`, use `key.Matches(msg, keys.Add)`. Otherwise fall through to `msg.String() == "a"`. Check `keys.go` before implementing — don't add a redundant binding.

**Note on toggle-all behavior:** "Toggle all" toggles each item independently (selected → unselected, unselected → selected). It does not set all to the same state. This means a mixed selection becomes fully inverted.

**Test:** Port the existing `TestCreateLoadoutModalSteps`, `TestCreateLoadoutTypeSelection`, and `TestCreateLoadoutPerTypeItems` tests to use `createLoadoutScreen`. Rename to `TestCreateLoadoutScreenSteps` etc. The assertions are the same — only the constructor name changes.

```go
func TestCreateLoadoutScreenSteps(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)

    t.Run("starts at provider step when no prefill", func(t *testing.T) {
        s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
        if s.step != clStepProvider {
            t.Errorf("step = %v, want clStepProvider", s.step)
        }
    })

    t.Run("starts at types step when pre-filled provider", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        if s.step != clStepTypes {
            t.Errorf("step = %v, want clStepTypes", s.step)
        }
    })

    t.Run("enter on provider step advances to types", func(t *testing.T) {
        s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
        s, _ = s.Update(keyEnter)
        if s.step != clStepTypes {
            t.Errorf("step = %v, want clStepTypes after Enter", s.step)
        }
    })

    t.Run("esc on types step goes back to provider", func(t *testing.T) {
        s := newCreateLoadoutScreen("", "", providers, cat, 80, 30)
        s.step = clStepTypes
        s, _ = s.Update(keyEsc)
        if s.step != clStepProvider {
            t.Errorf("step = %v, want clStepProvider after Esc on types", s.step)
        }
    })

    t.Run("space on items step toggles selection", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        s.buildTypeItemMaps()
        if len(s.filteredTypeItems()) == 0 {
            t.Skip("no items to toggle")
        }
        initial := s.entries[s.filteredTypeItems()[0]].selected
        s, _ = s.Update(keySpace)
        after := s.entries[s.filteredTypeItems()[0]].selected
        if after == initial {
            t.Error("Space should toggle selection")
        }
    })

    t.Run("t key toggles showAllCompat", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        initial := s.showAllCompat
        s, _ = s.Update(keyRune('t'))
        if s.showAllCompat == initial {
            t.Error("t key should toggle showAllCompat")
        }
    })

    t.Run("slash activates search", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        s, _ = s.Update(keyRune('/'))
        if !s.searchActive {
            t.Error("/ key should activate search")
        }
    })

    t.Run("esc clears search when active", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        s.searchActive = true
        s, _ = s.Update(keyEsc)
        if s.searchActive {
            t.Error("Esc should deactivate search")
        }
    })

    t.Run("l/right moves focus to preview pane", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        s, _ = s.Update(keyRune('l'))
        if s.splitView.focusedPane != panePreview {
            t.Error("l should focus preview pane")
        }
    })

    t.Run("h/left moves focus to list pane", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
        s.step = clStepItems
        s.splitView.focusedPane = panePreview
        s, _ = s.Update(keyRune('h'))
        if s.splitView.focusedPane != paneList {
            t.Error("h should focus list pane")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- All ported tests pass
- `previewCmdForCursor()` compiles and returns a valid `tea.Cmd`

**Dependencies:** Task 2.2

---

### Task 2.4 — Implement createLoadoutScreen.View() — left pane content

**File:** `cli/internal/tui/loadout_create.go`

Implement `View()` for `createLoadoutScreen`. The left pane renders what the modal did, but without the border/padding constraints. The right pane is owned by `splitView` (see Task 2.5).

```go
func (m createLoadoutScreen) View() string {
    // Breadcrumb
    breadcrumbSegments := []BreadcrumbSegment{
        {"Home", "crumb-home"},
        {"Loadouts", "crumb-parent"},
    }
    stepLabel := fmt.Sprintf("(%d of %d)", m.currentStepNum(), m.dynamicTotalSteps())
    if m.step == clStepItems {
        breadcrumbSegments = append(breadcrumbSegments,
            BreadcrumbSegment{m.currentType().Label(), ""})
    } else {
        breadcrumbSegments = append(breadcrumbSegments,
            BreadcrumbSegment{"Create", ""})
    }
    s := renderBreadcrumb(breadcrumbSegments...) + "  " + helpStyle.Render(stepLabel) + "\n\n"

    // Left pane content
    left := m.renderLeftPane()

    // Split view (left + right)
    // NOTE: renderWithCustomLeft does NOT exist on splitViewModel. Use the method added in Task 2.5:
    s += m.renderSplitTitleBar()
    s += m.renderSplitView(left)

    return s
}
```

Implement `renderLeftPane()` — this renders the same body content as the old `createLoadoutModal.View()` switch block, but without the outer border. Use the existing per-step render logic, adapted to use the full content width instead of `createLoadoutModalWidth`:

```go
func (m createLoadoutScreen) renderLeftPane() string {
    leftW := m.splitView.leftWidth()
    var body string

    switch m.step {
    case clStepProvider:
        body = labelStyle.Render("Pick a provider:") + "\n\n"
        for i, prov := range m.providerList {
            prefix, style := cursorPrefix(i == m.providerCursor)
            detected := ""
            if prov.Detected {
                detected = " " + installedStyle.Render("(detected)")
            }
            row := fmt.Sprintf("  %s%s%s", prefix, style.Render(prov.Name), detected)
            body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
        }

    case clStepTypes:
        body = labelStyle.Render("Uncheck any types to skip.") + "\n\n"
        for i, te := range m.typeEntries {
            checkBox := helpStyle.Render("[x]")
            if !te.checked {
                checkBox = helpStyle.Render("[ ]")
            }
            prefix, style := cursorPrefix(i == m.typeCursor)
            badge := ""
            if te.ct == catalog.Hooks || te.ct == catalog.MCP {
                badge = " " + warningStyle.Render("!!")
            }
            countLabel := helpStyle.Render(fmt.Sprintf("(%d)", te.count))
            row := fmt.Sprintf("  %s%s %s%s %s",
                prefix, checkBox, style.Render(te.ct.Label()), badge, countLabel)
            body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
        }
        body += "\n" + helpStyle.Render("space toggle • a toggle all • enter next")
        if m.message != "" && m.messageIsErr {
            body += "\n" + errorMsgStyle.Render(m.message)
        }

    case clStepItems:
        ct := m.currentType()
        selCount := m.currentTypeSelectedCount()
        body = labelStyle.Render(fmt.Sprintf("Select %s (%d selected)", ct.Label(), selCount)) + "\n"
        if m.searchActive {
            body += zone.Mark("wiz-field-search", m.searchInput.View()) + "\n"
        } else {
            body += "\n"
        }
        filtered := m.filteredTypeItems()
        cursor := m.perTypeCursor[ct]
        visibleH := m.splitView.visibleListRows() - 4
        if visibleH < 3 {
            visibleH = 3
        }
        start := 0
        if len(filtered) > visibleH {
            start = cursor - visibleH/2
            if start < 0 {
                start = 0
            }
            if start+visibleH > len(filtered) {
                start = len(filtered) - visibleH
            }
        }
        end := start + visibleH
        if end > len(filtered) {
            end = len(filtered)
        }
        if start > 0 {
            body += "  " + renderScrollUp(start, false) + "\n"
        }
        maxNameW := leftW - 10
        if maxNameW < 10 {
            maxNameW = 10
        }
        for vi, fi := range filtered[start:end] {
            e := m.entries[fi]
            absIdx := start + vi
            compatible := m.isItemCompatible(fi)
            checkBox := helpStyle.Render("[ ]")
            if e.selected {
                checkBox = helpStyle.Render("[x]")
            }
            prefix, style := cursorPrefix(absIdx == cursor)
            name := e.item.Name
            if len(name) > maxNameW {
                name = name[:maxNameW-3] + "..."
            }
            source := ""
            if e.item.Registry != "" {
                source = " (" + e.item.Registry + ")"
            } else if e.item.Library {
                source = " (library)"
            }
            var row string
            if !compatible {
                row = fmt.Sprintf("  %s%s %s%s (incompatible)",
                    prefix, checkBox, helpStyle.Render(strikethrough(name)), helpStyle.Render(source))
            } else {
                row = fmt.Sprintf("  %s%s %s%s",
                    prefix, checkBox, style.Render(name), helpStyle.Render(source))
            }
            body += zone.Mark(fmt.Sprintf("wiz-opt-%d", absIdx), row) + "\n"
        }
        if end < len(filtered) {
            body += "  " + renderScrollDown(len(filtered)-end, false) + "\n"
        }
        filterMode := "compatible only"
        if m.showAllCompat {
            filterMode = "showing all"
        }
        body += "\n" + helpStyle.Render(fmt.Sprintf("space select • a all • t filter (%s) • / search • enter next", filterMode))

    case clStepName:
        body = labelStyle.Render("Name your loadout:") + "\n\n"
        body += zone.Mark("wiz-field-name", m.nameInput.View()) + "\n"
        body += zone.Mark("wiz-field-desc", m.descInput.View()) + "\n"
        body += "\n" + helpStyle.Render("tab switch field • enter next")
        if m.message != "" && m.messageIsErr {
            body += "\n" + errorMsgStyle.Render(m.message)
        }

    case clStepDest:
        body = labelStyle.Render("Choose destination:") + "\n\n"
        for i, opt := range m.destOptions {
            prefix, style := cursorPrefix(i == m.destCursor)
            if m.destDisabled[i] {
                style = helpStyle
                prefix = "  "
            }
            row := fmt.Sprintf("  %s%s", prefix, style.Render(opt))
            if m.destDisabled[i] && m.destHints[i] != "" {
                row += "\n      " + helpStyle.Render(m.destHints[i])
            }
            body += zone.Mark(fmt.Sprintf("wiz-opt-%d", i), row) + "\n"
        }
        body += "\n" + helpStyle.Render("enter next • esc back")

    case clStepReview:
        body = m.renderReviewLeftPane(leftW)
    }

    return body
}
```

**Test:** Add a View smoke test:

```go
func TestCreateLoadoutScreenView(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)
    s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)

    t.Run("types step renders without panic", func(t *testing.T) {
        got := s.View()
        if got == "" {
            t.Error("View() returned empty string")
        }
        if !strings.Contains(got, "Create") {
            t.Error("breadcrumb should contain Create")
        }
    })
    t.Run("types step shows danger badges", func(t *testing.T) {
        // Force hooks and MCP into typeEntries
        s.typeEntries = []typeCheckEntry{
            {ct: catalog.Hooks, checked: true, count: 1},
            {ct: catalog.MCP, checked: true, count: 1},
            {ct: catalog.Skills, checked: true, count: 2},
        }
        got := s.View()
        if !strings.Contains(got, "!!") {
            t.Error("types step should show !! badge for Hooks and MCP")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- `make test` passes
- `View()` renders breadcrumb, step content, and `!!` badges on Hooks/MCP

**Dependencies:** Task 2.3

---

### Task 2.5 — Implement split-view rendering for wizard steps

**File:** `cli/internal/tui/loadout_create.go`

The wizard needs a custom split-view render because the left pane is not a simple list — it's rich wizard content. The screen's own `View()` composes the split manually using existing `splitViewModel` helpers.

**splitViewModel methods used by the wizard — all already exist in `split_view.go`:**

| Method | Status | Notes |
|--------|--------|-------|
| `m.splitView.leftWidth()` | EXISTS (line 105) | returns left pane width, adaptive ratio |
| `m.splitView.rightWidth()` | EXISTS (line 123) | returns right pane width |
| `m.splitView.visibleListRows()` | EXISTS (line 127) | visible rows for scroll math |
| `m.splitView.renderPreviewContent(rightW)` | EXISTS (line 707) | renders preview body without title |
| `m.splitView.SetPreview(content)` | EXISTS (line 70) | sets preview content and resets scroll |
| `m.splitView.focusedPane` | EXISTS (field, line 48) | `paneList` or `panePreview` |
| `paneList` / `panePreview` | EXISTS (consts, lines 26–28) | split pane enum values |
| `splitViewMinWidth` | EXISTS (const, line 59) | 70 — threshold for split vs single-pane |

No new methods need to be added to `split_view.go`. The wizard composes these directly.

**Recommended approach:** Add a method to `createLoadoutScreen` that does the composition:

```go
// renderSplitView composes the left wizard pane and right preview pane.
// Left pane always shows wizard content. Right pane shows file preview
// when a cursor item has a previewable path (items and review steps).
func (m createLoadoutScreen) renderSplitView(leftContent string) string {
    leftW := m.splitView.leftWidth()
    contentW := m.width

    if contentW < splitViewMinWidth {
        // Narrow: single pane, no preview
        return leftContent
    }

    rightW := contentW - leftW - 1 // -1 for separator

    leftLines := strings.Split(leftContent, "\n")
    rightLines := strings.Split(m.splitView.renderPreviewContent(rightW), "\n")

    displayH := m.height - 5 // header (breadcrumb + step label + blank) + help bar (2)
    if displayH < 5 {
        displayH = 5
    }

    // Pad to displayH
    for len(leftLines) < displayH {
        leftLines = append(leftLines, "")
    }
    for len(rightLines) < displayH {
        rightLines = append(rightLines, "")
    }

    sep := helpStyle.Render("│")
    var rows []string
    for i := 0; i < displayH; i++ {
        l := leftLines[i]
        r := rightLines[i]
        visW := lipgloss.Width(l)
        if visW < leftW {
            l = l + strings.Repeat(" ", leftW-visW)
        }
        rows = append(rows, l+sep+r)
    }
    return strings.Join(rows, "\n")
}
```

Add a title bar for the split:

```go
func (m createLoadoutScreen) renderSplitTitleBar() string {
    // Show "Items | Preview" title bar on items and review steps
    if m.step != clStepItems && m.step != clStepReview {
        return ""
    }
    listLabel := "Items"
    if m.step == clStepReview {
        listLabel = "Review"
    }
    sep := helpStyle.Render(" | ")
    leftStyle := activeTabStyle
    rightStyle := inactiveTabStyle
    if m.splitView.focusedPane == panePreview {
        leftStyle = inactiveTabStyle
        rightStyle = activeTabStyle
    }
    return leftStyle.Render(listLabel) + sep + rightStyle.Render("Preview") + "\n\n"
}
```

Update `View()` to use these:

```go
func (m createLoadoutScreen) View() string {
    // Breadcrumb + step label
    // ...
    s += m.renderSplitTitleBar()
    left := m.renderLeftPane()
    s += m.renderSplitView(left)
    return s
}
```

**Test:** Add a test confirming split-view renders at wide and narrow:

```go
func TestCreateLoadoutScreenSplitView(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)

    t.Run("wide terminal shows separator", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 120, 40)
        s.step = clStepItems
        s.buildTypeItemMaps()
        got := s.View()
        if !strings.Contains(got, "│") {
            t.Error("wide terminal should show split-view separator")
        }
    })
    t.Run("narrow terminal single pane", func(t *testing.T) {
        s := newCreateLoadoutScreen("claude-code", "", providers, cat, 60, 20)
        s.step = clStepItems
        s.buildTypeItemMaps()
        got := s.View()
        // Should not panic; single-pane mode
        if got == "" {
            t.Error("narrow terminal View() should not be empty")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- Wide: `│` separator visible in items and review steps
- Narrow: no separator, no panic
- Title bar shows "Items | Preview" on items step

**Dependencies:** Task 2.4

---

### Task 2.6 — Implement preview loading for items step

**File:** `cli/internal/tui/app.go` and `cli/internal/tui/loadout_create.go`

When the cursor moves in the items step, App receives a `splitViewCursorMsg` and loads the preview content:

In `app.go`, in the `splitViewCursorMsg` handler:

```go
case splitViewCursorMsg:
    if a.screen == screenDetail {
        var cmd tea.Cmd
        a.detail, cmd = a.detail.Update(msg)
        return a, cmd
    }
    if a.screen == screenCreateLoadout {
        // Load file preview for the cursor item
        if msg.item.Path != "" {
            content, err := os.ReadFile(msg.item.Path)
            if err != nil {
                // Preview read failure is silent — show empty preview rather than crash
                a.createLoadout.splitView.SetPreview("")
            } else {
                a.createLoadout.splitView.SetPreview(string(content))
            }
        } else {
            a.createLoadout.splitView.SetPreview("")
        }
        return a, nil
    }
```

**File read error handling:** If `os.ReadFile` fails (file not found, permissions, etc.), the preview silently shows empty. This is intentional — preview is a convenience, not a required step. The user can still select items without a preview. No error toast is shown for preview failures.

Add `createLoadout createLoadoutScreen` field to `App` struct (alongside the existing `createLoadoutModal createLoadoutModal`, which is removed in Task 2.10):

```go
createLoadout     createLoadoutScreen
// Keep createLoadoutModal for now — remove in Task 2.10
```

**Test:**

```go
func TestCreateLoadoutScreenPreview(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)

    // Set up the app directly on the create loadout screen
    // (avoids needing to navigate through the full sidebar flow)
    app := testAppSize(t, 80, 30)
    app.screen = screenCreateLoadout
    app.createLoadout = newCreateLoadoutScreen("claude-code", "", providers, cat,
        app.width-sidebarWidth-1, app.panelHeight())
    app.createLoadout.step = clStepItems
    app.createLoadout.buildTypeItemMaps()

    t.Run("cursor move sends splitViewCursorMsg and loads preview", func(t *testing.T) {
        filtered := app.createLoadout.filteredTypeItems()
        if len(filtered) == 0 {
            t.Skip("no items to preview")
        }

        // Send Down key — should trigger previewCmdForCursor
        m, cmd := app.Update(keyDown)
        app = m.(App)

        // Execute the returned command to get the splitViewCursorMsg
        if cmd == nil {
            t.Skip("no command returned — cursor already at bottom or no items")
        }
        msg := cmd()
        cursorMsg, ok := msg.(splitViewCursorMsg)
        if !ok {
            t.Fatalf("expected splitViewCursorMsg, got %T", msg)
        }

        // Process the cursor message — App should load the preview
        m, _ = app.Update(cursorMsg)
        app = m.(App)

        // Items with a real file path should have non-empty preview content
        // (testCatalog creates real files via t.TempDir)
        if cursorMsg.item.Path != "" {
            if app.createLoadout.splitView.previewContent == "" {
                t.Error("preview content should be loaded after splitViewCursorMsg")
            }
        }
    })

    t.Run("unreadable path gives empty preview without panic", func(t *testing.T) {
        cursorMsg := splitViewCursorMsg{
            index: 0,
            item:  splitViewItem{Label: "test", Path: "/nonexistent/path/file.md"},
        }
        app.screen = screenCreateLoadout
        m, _ := app.Update(cursorMsg)
        app = m.(App)
        // Should not panic; preview should be empty
        if app.createLoadout.splitView.previewContent != "" {
            t.Error("unreadable file should result in empty preview")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- Moving cursor in items step triggers `splitViewCursorMsg`
- App loads file content into `splitView.previewContent`
- Preview content visible in right pane

**Dependencies:** Task 2.5

---

### Task 2.7 — Wire screenCreateLoadout into App routing (entry points)

**File:** `cli/internal/tui/app.go`

Replace all `newCreateLoadoutModal(...)` calls with `newCreateLoadoutScreen(...)` and `a.screen = screenCreateLoadout`.

**Entry point 1** — loadout cards `screenLoadoutCards`, `keys.Add`:

```go
if key.Matches(msg, keys.Add) {
    a.createLoadout = newCreateLoadoutScreen("", "", a.providers, a.catalog,
        a.width-sidebarWidth-1, a.panelHeight())
    a.screen = screenCreateLoadout
    a.focus = focusContent
    return a, nil
}
```

**Entry point 2** — loadout items list `screenItems` with `contentType == catalog.Loadouts`:

```go
if key.Matches(msg, keys.Add) && a.items.contentType == catalog.Loadouts {
    provider := a.items.sourceProvider
    a.createLoadout = newCreateLoadoutScreen(provider, a.items.sourceRegistry,
        a.providers, a.catalog, a.width-sidebarWidth-1, a.panelHeight())
    a.screen = screenCreateLoadout
    a.focus = focusContent
    return a, nil
}
```

**Entry point 3** — items list with `sourceRegistry != ""` and `keys.CreateLoadout`:

```go
if key.Matches(msg, keys.CreateLoadout) && a.items.sourceRegistry != "" {
    a.createLoadout = newCreateLoadoutScreen("", a.items.sourceRegistry,
        a.providers, a.catalog, a.width-sidebarWidth-1, a.panelHeight())
    a.screen = screenCreateLoadout
    a.focus = focusContent
    return a, nil
}
```

**Entry point 4** — action button clicks (zones `action-a` on loadout pages, from Phase 1). Update Phase 1 zone handlers to use the new screen instead of modal.

**View routing** — add `screenCreateLoadout` to `View()`:

```go
case screenCreateLoadout:
    contentView = a.createLoadout.View()
```

**Footer breadcrumb** — add `screenCreateLoadout` to the `breadcrumb()` function in app.go (around line 3474):

```go
case screenCreateLoadout:
    return "Loadouts > Create"
```

**Key routing** — add to the `switch a.screen` block:

```go
case screenCreateLoadout:
    if key.Matches(msg, keys.Back) && a.createLoadout.step == clStepProvider {
        a.screen = screenLoadoutCards
        a.focus = focusContent
        return a, nil
    }
    if key.Matches(msg, keys.Back) && a.createLoadout.step == clStepTypes &&
        a.createLoadout.prefilledProvider != "" {
        a.screen = screenLoadoutCards
        a.focus = focusContent
        return a, nil
    }
    var cmd tea.Cmd
    a.createLoadout, cmd = a.createLoadout.Update(msg)
    if a.createLoadout.confirmed {
        return a, a.doCreateLoadoutFromScreen(a.createLoadout)
    }
    return a, cmd
```

**WindowSizeMsg** — add to the size handler:

```go
a.createLoadout.width = contentW
a.createLoadout.height = ph
a.createLoadout.splitView.width = contentW
a.createLoadout.splitView.height = ph - 5
```

**Test:**

```go
func TestCreateLoadoutScreenRouting(t *testing.T) {
    app := testApp(t)

    // Navigate to loadout cards
    nTypes := sidebarContentCount()
    app = pressN(app, keyDown, nTypes+1)
    m, _ := app.Update(keyEnter)
    app = m.(App)
    assertScreen(t, app, screenLoadoutCards)

    // Press 'a' — should open create loadout screen (not modal)
    m, _ = app.Update(keyRune('a'))
    app = m.(App)
    assertScreen(t, app, screenCreateLoadout)

    // Esc on first step returns to loadout cards
    m, _ = app.Update(keyEsc)
    app = m.(App)
    assertScreen(t, app, screenLoadoutCards)
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- `a` key on loadout cards opens `screenCreateLoadout`
- `a` key on loadout items list opens `screenCreateLoadout` with prefilled provider
- `Esc` on first step navigates back to the previous screen
- `screenCreateLoadout` renders in `View()` without panic

**Dependencies:** Task 2.6

---

### Task 2.8 — Wire doCreateLoadout for the new screen type

**File:** `cli/internal/tui/app.go`

Add `doCreateLoadoutFromScreen()` that accepts `createLoadoutScreen` instead of `createLoadoutModal`:

```go
// doCreateLoadoutFromScreen writes a loadout.yaml from the screen wizard state.
// Identical logic to doCreateLoadout() — only the input type differs.
func (a *App) doCreateLoadoutFromScreen(m createLoadoutScreen) tea.Cmd {
    contentRoot := a.catalog.RepoRoot
    scopeRegistry := m.scopeRegistry
    return func() tea.Msg {
        name := strings.TrimSpace(m.nameInput.Value())
        desc := strings.TrimSpace(m.descInput.Value())
        provSlug := m.prefilledProvider

        manifest := loadout.Manifest{
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

        var destDir string
        switch m.destCursor {
        case 0:
            destDir = filepath.Join(contentRoot, "loadouts", provSlug)
        case 1:
            home, homeErr := os.UserHomeDir()
            if homeErr != nil {
                return doCreateLoadoutMsg{err: fmt.Errorf("finding home directory: %w", homeErr)}
            }
            destDir = filepath.Join(home, ".syllago", "content", "loadouts", provSlug)
        case 2:
            dir, err := registry.CloneDir(scopeRegistry)
            if err != nil {
                return doCreateLoadoutMsg{err: err}
            }
            destDir = filepath.Join(dir, "loadouts", provSlug)
        }

        itemDir := filepath.Join(destDir, name)
        if err := os.MkdirAll(itemDir, 0755); err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("creating loadout dir: %w", err)}
        }
        data, err := yaml.Marshal(manifest)
        if err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("marshaling manifest: %w", err)}
        }
        outPath := filepath.Join(itemDir, "loadout.yaml")
        if err := os.WriteFile(outPath, data, 0644); err != nil {
            return doCreateLoadoutMsg{err: fmt.Errorf("writing loadout.yaml: %w", err)}
        }
        return doCreateLoadoutMsg{name: name, provider: provSlug}
    }
}
```

The `doCreateLoadoutMsg` handler in `Update()` already navigates to the new loadout's detail — no changes needed there.

**Test:** Adapt `TestCreateLoadoutRescanFindsNewLoadout` to use the screen:

```go
func TestCreateLoadoutScreenRescanFindsNewLoadout(t *testing.T) {
    app := testApp(t)
    contentRoot := app.catalog.RepoRoot
    beforeCount := len(app.catalog.ByType(catalog.Loadouts))

    screen := newCreateLoadoutScreen("claude-code", "", app.providers, app.catalog, 80, 30)
    screen.nameInput.SetValue("new-screen-loadout")
    screen.destCursor = 0
    screen.confirmed = true

    cmd := app.doCreateLoadoutFromScreen(screen)
    msg := cmd()

    result := msg.(doCreateLoadoutMsg)
    if result.err != nil {
        t.Fatalf("doCreateLoadoutFromScreen failed: %v", result.err)
    }

    cat, err := catalog.Scan(contentRoot, contentRoot)
    if err != nil {
        t.Fatalf("rescan failed: %v", err)
    }
    afterCount := len(cat.ByType(catalog.Loadouts))
    if afterCount != beforeCount+1 {
        t.Errorf("loadout count = %d, want %d", afterCount, beforeCount+1)
    }
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- `doCreateLoadoutFromScreen` writes a valid `loadout.yaml`
- Catalog rescan finds the new loadout
- `doCreateLoadoutMsg` handler navigates to the new loadout's detail view

**Dependencies:** Task 2.7

---

### Task 2.9 — Implement mouse support for all wizard interactive elements

**File:** `cli/internal/tui/app.go` — mouse handler section

Add a `screenCreateLoadout` mouse handling block. The pattern follows the existing `createLoadoutModal` mouse block but uses `wiz-*` zone IDs and synthesizes key messages instead of coordinate math.

**Search activation via click:** The design's Mouse Parity Checklist requires "Click search zone" to activate search. The `wiz-field-search` zone is only rendered when `searchActive == true` (it shows the textinput). To make search *activation* clickable, `renderLeftPane()` must also render a clickable `[/] Search` hint zone when search is *inactive*. Add this to `clStepItems` in `renderLeftPane()`:

```go
// In clStepItems rendering, after the header, before the item list:
if m.searchActive {
    body += zone.Mark("wiz-field-search", m.searchInput.View()) + "\n"
} else {
    // Render a clickable search hint so mouse users can activate search
    body += zone.Mark("wiz-field-search", helpStyle.Render("/ search...")) + "\n"
}
```

This way `zone.Get("wiz-field-search").InBounds(msg)` is always a valid click target on the items step, whether or not search is currently active.

**Breadcrumb click navigation:** Breadcrumb clicks on `crumb-home` and `crumb-parent` are handled globally in `app.go` (around lines 1449–1476) for all screens. The `screenCreateLoadout` breadcrumb uses the same `crumb-home` and `crumb-parent` zone IDs, so breadcrumb navigation is automatically handled without additional code. No new task is needed for this.

```go
if a.screen == screenCreateLoadout {
    switch a.createLoadout.step {
    case clStepProvider:
        for i := range a.createLoadout.providerList {
            if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
                a.createLoadout.providerCursor = i
                m, cmd := a.createLoadout.Update(tea.KeyMsg{Type: tea.KeyEnter})
                a.createLoadout = m
                return a, cmd
            }
        }
    case clStepTypes:
        for i := range a.createLoadout.typeEntries {
            if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
                a.createLoadout.typeCursor = i
                a.createLoadout.typeEntries[i].checked = !a.createLoadout.typeEntries[i].checked
                return a, nil
            }
        }
    case clStepItems:
        ct := a.createLoadout.currentType()
        filtered := a.createLoadout.filteredTypeItems()
        for i := range filtered {
            if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) {
                a.createLoadout.perTypeCursor[ct] = i
                entryIdx := filtered[i]
                if a.createLoadout.isItemCompatible(entryIdx) {
                    a.createLoadout.entries[entryIdx].selected = !a.createLoadout.entries[entryIdx].selected
                    a.createLoadout.updateDestConstraints()
                }
                return a, a.createLoadout.previewCmdForCursor()
            }
        }
        // Search field click — activates the in-wizard item search (not the global search bar).
        // The zone "wiz-field-search" is only rendered when searchActive is already true
        // (see renderLeftPane). To allow clicking to *activate* search, also render a
        // "[/] Search" zone even when inactive. See note below.
        if zone.Get("wiz-field-search").InBounds(msg) {
            a.createLoadout.searchActive = true
            a.createLoadout.searchInput.Focus()
            return a, nil
        }
    case clStepName:
        if zone.Get("wiz-field-name").InBounds(msg) {
            a.createLoadout.nameFirst = true
            a.createLoadout.descInput.Blur()
            a.createLoadout.nameInput.Focus()
            return a, nil
        }
        if zone.Get("wiz-field-desc").InBounds(msg) {
            a.createLoadout.nameFirst = false
            a.createLoadout.nameInput.Blur()
            a.createLoadout.descInput.Focus()
            return a, nil
        }
    case clStepDest:
        for i := range a.createLoadout.destOptions {
            if zone.Get(fmt.Sprintf("wiz-opt-%d", i)).InBounds(msg) &&
                !a.createLoadout.destDisabled[i] {
                a.createLoadout.destCursor = i
                return a, nil
            }
        }
    case clStepReview:
        if zone.Get("wiz-btn-back").InBounds(msg) {
            a.createLoadout.reviewBtnCursor = 0
            m, cmd := a.createLoadout.Update(tea.KeyMsg{Type: tea.KeyEnter})
            a.createLoadout = m
            return a, cmd
        }
        if zone.Get("wiz-btn-create").InBounds(msg) {
            a.createLoadout.reviewBtnCursor = 1
            m, cmd := a.createLoadout.Update(tea.KeyMsg{Type: tea.KeyEnter})
            a.createLoadout = m
            if a.createLoadout.confirmed {
                return a, a.doCreateLoadoutFromScreen(a.createLoadout)
            }
            return a, cmd
        }
        // Item list clicks in review step
        for i, item := range a.createLoadout.reviewItems() {
            if zone.Get(fmt.Sprintf("wiz-review-%d", i)).InBounds(msg) {
                a.createLoadout.reviewItemCursor = i
                // Load preview for clicked item
                if item.path != "" {
                    content, _ := os.ReadFile(item.path)
                    a.createLoadout.splitView.SetPreview(string(content))
                }
                return a, nil
            }
        }
    }
    // Click in pane to focus — routes left-click to splitView so it can detect
    // which pane was clicked and update focusedPane accordingly.
    if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
        var svCmd tea.Cmd
        a.createLoadout.splitView, svCmd = a.createLoadout.splitView.Update(msg)
        return a, svCmd
    }
    // Scroll wheel in split view
    if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
        var svCmd tea.Cmd
        a.createLoadout.splitView, svCmd = a.createLoadout.splitView.Update(msg)
        return a, svCmd
    }
    return a, nil
}
```

**Test:**

```go
func TestCreateLoadoutScreenMouse(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)

    newApp := func(t *testing.T) App {
        t.Helper()
        app := testAppSize(t, 80, 30)
        app.screen = screenCreateLoadout
        app.createLoadout = newCreateLoadoutScreen("claude-code", "", providers, cat,
            app.width-sidebarWidth-1, app.panelHeight())
        return app
    }

    // Helper: build a left-click release message at the center of a zone.
    clickZone := func(zoneID string) tea.MouseMsg {
        z := zone.Get(zoneID)
        return tea.MouseMsg{
            Action: tea.MouseActionRelease,
            Button: tea.MouseButtonLeft,
            X:      (z.StartX + z.EndX) / 2,
            Y:      z.StartY,
        }
    }

    t.Run("clicking type checkbox toggles it", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepTypes
        if len(app.createLoadout.typeEntries) == 0 {
            t.Skip("no type entries")
        }
        initial := app.createLoadout.typeEntries[0].checked
        // Render to register zones
        _ = app.View()
        m, _ := app.Update(clickZone("wiz-opt-0"))
        app = m.(App)
        if app.createLoadout.typeEntries[0].checked == initial {
            t.Error("click on wiz-opt-0 should toggle type checkbox")
        }
    })

    t.Run("clicking provider option selects and advances", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepProvider
        _ = app.View()
        m, _ := app.Update(clickZone("wiz-opt-0"))
        app = m.(App)
        // Clicking a provider should advance to types step
        if app.createLoadout.step != clStepTypes {
            t.Errorf("step = %v, want clStepTypes after clicking provider", app.createLoadout.step)
        }
    })

    t.Run("clicking name field focuses it", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepName
        app.createLoadout.nameFirst = false // start on desc
        _ = app.View()
        m, _ := app.Update(clickZone("wiz-field-name"))
        app = m.(App)
        if !app.createLoadout.nameFirst {
            t.Error("clicking wiz-field-name should set nameFirst=true")
        }
    })

    t.Run("clicking dest option selects it", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepDest
        app.createLoadout.destCursor = 0
        _ = app.View()
        // Click option 1 (Library)
        m, _ := app.Update(clickZone("wiz-opt-1"))
        app = m.(App)
        if app.createLoadout.destCursor != 1 {
            t.Errorf("destCursor = %d, want 1 after clicking option 1", app.createLoadout.destCursor)
        }
    })

    t.Run("clicking Back button in review goes back", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepReview
        app.createLoadout.reviewBtnCursor = 1 // start on Create
        _ = app.View()
        m, _ := app.Update(clickZone("wiz-btn-back"))
        app = m.(App)
        // Should navigate back to dest step
        if app.createLoadout.step != clStepDest {
            t.Errorf("step = %v, want clStepDest after clicking Back", app.createLoadout.step)
        }
    })

    t.Run("clicking search zone activates search", func(t *testing.T) {
        app := newApp(t)
        app.createLoadout.step = clStepItems
        app.createLoadout.buildTypeItemMaps()
        app.createLoadout.searchActive = false
        _ = app.View()
        m, _ := app.Update(clickZone("wiz-field-search"))
        app = m.(App)
        if !app.createLoadout.searchActive {
            t.Error("clicking wiz-field-search should activate search")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- All interactive elements in Mouse Parity Checklist from design doc are clickable
- No panics on mouse events at any step

**Dependencies:** Task 2.7

---

### Task 2.10 — Delete old modal code

> **Timing:** This task runs AFTER Task 2.7 has updated all entry points (action button zones, key handlers) to use `createLoadoutScreen`. At that point, `createLoadoutModal` is no longer referenced from any entry point and is safe to delete. Do NOT run this task before Task 2.7 is complete — the Phase 1 action button mouse handlers still reference `newCreateLoadoutModal()` until Task 2.7 updates them.

**File:** `cli/internal/tui/loadout_create.go`

Remove entirely:
- `createLoadoutModal` struct definition
- `newCreateLoadoutModal()` constructor
- All methods on `createLoadoutModal`
- `overlayView()` method
- `createLoadoutModalWidth` / `createLoadoutModalHeight` constants

**File:** `cli/internal/tui/app.go`

Remove:
- `createLoadoutModal createLoadoutModal` field from `App` struct
- The `a.createLoadoutModal.active` block in the `tea.MouseMsg` handler (around line 1277)
- The `if a.createLoadoutModal.active` block in the `tea.KeyMsg` handler (around line 1699)
- `if a.createLoadoutModal.active { body = a.createLoadoutModal.overlayView(body) }` in `View()`
- The old `doCreateLoadout(m createLoadoutModal)` method (keep only `doCreateLoadoutFromScreen`)

Remove the import of `overlay "github.com/rmhubbert/bubbletea-overlay"` from `loadout_create.go` if it's no longer used there.

**Test:** Port the remaining tests in `loadout_create_test.go` that still reference `createLoadoutModal` to use `createLoadoutScreen`. Delete any tests that test the old modal overlay rendering.

**Commands:**
```
cd cli && make test
# All tests should pass; no compile errors
```

**Success criteria:**
- `make test` passes
- No references to `createLoadoutModal` remain in the codebase
- `focusModal` is no longer set for loadout creation (only for other modals that still exist)

**Dependencies:** Task 2.9

---

### Task 2.11 — Implement review step with navigable item list

**File:** `cli/internal/tui/loadout_create.go`

The review step needs a navigable item list in the left pane so users can cursor-navigate to dangerous items (Hooks/MCP) and see their content in the right pane. Add helper type and method:

```go
// reviewItem is one line in the review step's navigable item list.
type reviewItem struct {
    name string
    ct   catalog.ContentType
    path string // primary file path, for preview
}

// reviewItems returns all selected items as a flat list for the review step.
func (m createLoadoutScreen) reviewItems() []reviewItem {
    var items []reviewItem
    for _, ct := range catalog.AllContentTypes() {
        for _, e := range m.entries {
            if !e.selected || e.item.Type != ct {
                continue
            }
            items = append(items, reviewItem{
                name: e.item.Name,
                ct:   e.item.Type,
                path: primaryFilePath(e.item),
            })
        }
    }
    return items
}
```

Implement `renderReviewLeftPane()` (used by `renderLeftPane()` in `clStepReview`):

```go
func (m createLoadoutScreen) renderReviewLeftPane(leftW int) string {
    // Provider display name
    provName := m.prefilledProvider
    for _, p := range m.providerList {
        if p.Slug == m.prefilledProvider {
            provName = p.Name
            break
        }
    }
    destLabel := "Project"
    if m.destCursor < len(m.destOptions) {
        destLabel = m.destOptions[m.destCursor]
    }

    var b strings.Builder
    b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Name:"), m.nameInput.Value()))
    if desc := strings.TrimSpace(m.descInput.Value()); desc != "" {
        b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Desc:"), desc))
    }
    b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Provider:"), provName))
    b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Dest:"), destLabel))

    items := m.reviewItems()
    if len(items) == 0 {
        b.WriteString("\n" + warningStyle.Render("  No items selected") + "\n")
    } else {
        b.WriteString("\n  " + labelStyle.Render("Contents:") + "\n")
        for i, item := range items {
            prefix, style := cursorPrefix(i == m.reviewItemCursor)
            badge := ""
            if item.ct == catalog.Hooks || item.ct == catalog.MCP {
                badge = " " + warningStyle.Render("!!")
            }
            typeLabel := helpStyle.Render("[" + item.ct.Label() + "]")
            row := fmt.Sprintf("  %s%s%s %s",
                prefix, style.Render(item.name), badge, typeLabel)
            b.WriteString(zone.Mark(fmt.Sprintf("wiz-review-%d", i), row) + "\n")
        }
    }

    // Back/Create buttons pinned to bottom
    innerH := m.height - 10
    contentLines := strings.Count(b.String(), "\n")
    spacer := innerH - contentLines - 1
    if spacer < 0 {
        spacer = 0
    }
    b.WriteString(strings.Repeat("\n", spacer))

    // Render Back/Create buttons
    backStyle := buttonDisabledStyle
    createStyle := buttonDisabledStyle
    if m.reviewBtnCursor == 0 {
        backStyle = buttonStyle
    } else {
        createStyle = buttonStyle
    }
    backBtn := zone.Mark("wiz-btn-back", backStyle.Render("Back"))
    createBtn := zone.Mark("wiz-btn-create", createStyle.Render("Create"))
    b.WriteString("  " + backBtn + "  " + createBtn + "\n")

    return b.String()
}
```

Update `createLoadoutScreen.Update()` for `clStepReview` — add cursor navigation over the item list:

```go
case clStepReview:
    items := m.reviewItems()
    switch {
    case msg.Type == tea.KeyEsc:
        m.step = clStepDest
        return m, nil
    case key.Matches(msg, keys.Up):
        if m.reviewItemCursor > 0 {
            m.reviewItemCursor--
            return m, m.previewCmdForReviewCursor()
        }
    case key.Matches(msg, keys.Down):
        if m.reviewItemCursor < len(items)-1 {
            m.reviewItemCursor++
            return m, m.previewCmdForReviewCursor()
        }
    case key.Matches(msg, keys.Left):
        if m.reviewBtnCursor > 0 {
            m.reviewBtnCursor--
        }
    case key.Matches(msg, keys.Right):
        if m.reviewBtnCursor < 1 {
            m.reviewBtnCursor++
        }
    case msg.Type == tea.KeyEnter:
        if m.reviewBtnCursor == 0 {
            m.step = clStepDest
        } else {
            m.confirmed = true
        }
        return m, nil
    }
```

Add preview command for review cursor:

```go
func (m createLoadoutScreen) previewCmdForReviewCursor() tea.Cmd {
    items := m.reviewItems()
    if m.reviewItemCursor < 0 || m.reviewItemCursor >= len(items) {
        return nil
    }
    item := items[m.reviewItemCursor]
    svItem := splitViewItem{Label: item.name, Path: item.path}
    idx := m.reviewItemCursor
    return func() tea.Msg {
        return splitViewCursorMsg{index: idx, item: svItem}
    }
}
```

**Test:**

```go
func TestCreateLoadoutScreenReview(t *testing.T) {
    cat := testCatalog(t)
    providers := testProviders(t)
    s := newCreateLoadoutScreen("claude-code", "", providers, cat, 80, 30)
    // Select hook item for security badge test
    for i := range s.entries {
        if s.entries[i].item.Type == catalog.Hooks {
            s.entries[i].selected = true
        }
    }
    s.nameInput.SetValue("test-loadout")
    s.step = clStepReview

    t.Run("review shows !! badge for hooks", func(t *testing.T) {
        got := s.View()
        if !strings.Contains(got, "!!") {
            t.Error("review should show !! badge for hooks items")
        }
    })
    t.Run("review cursor navigates items", func(t *testing.T) {
        items := s.reviewItems()
        if len(items) < 1 {
            t.Skip("no items in review")
        }
        initial := s.reviewItemCursor
        s, _ = s.Update(keyDown)
        if len(items) > 1 && s.reviewItemCursor == initial {
            t.Error("down should move review cursor")
        }
    })
    t.Run("Back/Create buttons render", func(t *testing.T) {
        got := s.View()
        if !strings.Contains(got, "Back") || !strings.Contains(got, "Create") {
            t.Error("review should show Back and Create buttons")
        }
    })
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- Review step shows item list with `!!` badges on Hooks/MCP
- Cursor navigation moves through items
- Up/Down in right pane loads file preview for cursor item
- Back/Create buttons present and functional

**Dependencies:** Task 2.10

---

### Task 2.12 — helpText() and keyboard shortcut integration

**File:** `cli/internal/tui/loadout_create.go`

Add `helpText()` to `createLoadoutScreen`:

```go
func (m createLoadoutScreen) helpText() string {
    switch m.step {
    case clStepProvider:
        return "up/down navigate • enter select • esc back"
    case clStepTypes:
        return "up/down navigate • space toggle • a toggle all • enter next • esc back"
    case clStepItems:
        if m.searchActive {
            return "type to search • esc clear search"
        }
        filterMode := "compatible"
        if m.showAllCompat {
            filterMode = "all"
        }
        return fmt.Sprintf("space select • a all • t filter (%s) • / search • enter next • esc back", filterMode)
    case clStepName:
        return "tab switch field • enter next • esc back"
    case clStepDest:
        return "up/down navigate • enter select • esc back"
    case clStepReview:
        return "up/down items • left/right buttons • enter confirm • esc back"
    }
    return ""
}
```

**File:** `cli/internal/tui/app.go` — `renderFooter()` method

Add `screenCreateLoadout` to the footer help text routing:

```go
case screenCreateLoadout:
    helpText = a.createLoadout.helpText()
```

**File:** `cli/internal/tui/help_overlay.go`

Add a `screenCreateLoadout` section to the help overlay so `?` shows the wizard shortcuts.

**Test:**

```go
func TestCreateLoadoutScreenHelpText(t *testing.T) {
    s := newCreateLoadoutScreen("claude-code", "", nil, &catalog.Catalog{}, 80, 30)
    steps := []createLoadoutStep{clStepTypes, clStepItems, clStepName, clStepDest, clStepReview}
    for _, step := range steps {
        s.step = step
        h := s.helpText()
        if h == "" {
            t.Errorf("step %v: helpText() should not be empty", step)
        }
    }
}
```

**Commands:**
```
cd cli && make test
```

**Success criteria:**
- Footer bar shows correct help text for each wizard step
- `?` opens help overlay with wizard section

**Dependencies:** Task 2.11

---

### Task 2.13 — Golden file tests for all wizard steps at all 4 sizes

**File:** `cli/internal/tui/golden_test.go` or `golden_sizes_test.go`

Add golden file tests for each step type at all 4 terminal sizes. Navigate the app to `screenCreateLoadout` and capture `View()` output at each step:

```go
func TestCreateLoadoutScreenGoldens(t *testing.T) {
    sizes := []struct{ w, h int }{{60, 20}, {80, 30}, {120, 40}, {160, 50}}
    steps := []struct {
        name  string
        setup func(s *createLoadoutScreen)
    }{
        {"provider", func(s *createLoadoutScreen) {
            s.prefilledProvider = ""
            s.step = clStepProvider
        }},
        {"types", func(s *createLoadoutScreen) {
            s.step = clStepTypes
        }},
        {"items", func(s *createLoadoutScreen) {
            s.buildTypeItemMaps()
            s.step = clStepItems
        }},
        {"name", func(s *createLoadoutScreen) {
            s.step = clStepName
        }},
        {"dest", func(s *createLoadoutScreen) {
            s.step = clStepDest
        }},
        {"review", func(s *createLoadoutScreen) {
            s.nameInput.SetValue("test-loadout")
            s.step = clStepReview
        }},
        {"review-with-hooks", func(s *createLoadoutScreen) {
            for i := range s.entries {
                if s.entries[i].item.Type == catalog.Hooks {
                    s.entries[i].selected = true
                }
            }
            s.nameInput.SetValue("hook-loadout")
            s.step = clStepReview
        }},
    }

    for _, sz := range sizes {
        for _, step := range steps {
            t.Run(fmt.Sprintf("%s-%dx%d", step.name, sz.w, sz.h), func(t *testing.T) {
                app := testAppSize(t, sz.w, sz.h)
                cat := app.catalog
                providers := app.providers
                s := newCreateLoadoutScreen("claude-code", "", providers, cat, sz.w-sidebarWidth-1, app.panelHeight())
                step.setup(&s)
                app.screen = screenCreateLoadout
                app.createLoadout = s
                got := app.View()
                requireGolden(t, fmt.Sprintf("wizard-%s-%dx%d", step.name, sz.w, sz.h), got)
            })
        }
    }
}
```

**Commands:**
```
cd cli && go test ./internal/tui/ -update-golden
make test
# Second run should pass (goldens match)
```

**Success criteria:**
- 28 golden files created (7 step variants × 4 sizes)
- All golden files show correct breadcrumb, step content, split view
- `!!` badges appear in types and review-with-hooks goldens
- At 60x20 no separator (single-pane mode)
- At 120x40 and 160x50 split view visible with separator

**Dependencies:** Task 2.12

---

## Phase 3: Doc/Rules Updates

Goal: update all design and rules documentation to codify the new patterns so future AI-assisted changes follow them automatically.

---

### Task 3.1 — Update tui-page-pattern.md page inventory

**File:** `/home/hhewett/.local/src/syllago/.claude/rules/tui-page-pattern.md`

Add `screenCreateLoadout` to the page inventory table:

```markdown
| Create Loadout | `screenCreateLoadout` | Wizard | loadout_create.go | Yes (`Home > Loadouts > Create`) | No (wizard) | No |
```

Add a note about the wizard pattern (add after the existing Card Grid Pattern section):

```markdown
## Wizard Screen Pattern

Full-screen wizard steps render in the content pane (sidebar visible). Each step uses a persistent split-view:

- Left pane: step content (list, form, or summary)
- Right pane: file preview (on item selection and review steps; empty on form steps)

```go
type wizardScreen struct {
    step       wizardStep
    splitView  splitViewModel
    width      int
    height     int
    confirmed  bool
    message    string
    messageIsErr bool
}
```

Key differences from modal wizards:
- No `active bool` field — screens are always active when shown
- `Esc` on first step navigates back to previous screen (not `a.focus = focusSidebar`)
- `splitViewCursorMsg` triggers preview loading (routed in App.Update)
- `helpText()` is context-sensitive per step
```

**Success criteria:**
- Page inventory includes `screenCreateLoadout`
- Wizard screen pattern documented

**Dependencies:** Phase 2 complete

---

### Task 3.2 — Update tui-keyboard-bindings.md

**File:** `/home/hhewett/.local/src/syllago/.claude/rules/tui-keyboard-bindings.md`

Update the active key bindings table to clarify that `a` opens the Create Loadout screen (not a modal):

```markdown
| a | -- | add {type} | add content | create loadout screen | add registry | -- |
```

Add a note under "Tab Behavior":

```markdown
- `screenCreateLoadout` — Tab switches between name and description inputs on the Name step only; on all other steps Tab has no action (not a Tab-toggle screen and not a detail-tab screen).
```

Add to "Search Availability":

```markdown
`/` activates item search inside the Create Loadout wizard (items step only). It activates the in-wizard search, not the global search bar.
```

**Success criteria:**
- Keyboard bindings table reflects the screen-based wizard

**Dependencies:** Phase 2 complete

---

### Task 3.3 — Add action button pattern to a new rule file

**File:** `/home/hhewett/.local/src/syllago/.claude/rules/tui-action-buttons.md`

Create a new rule file documenting the action button pattern:

```markdown
# Action Button Pattern

Action buttons are chip-style clickable labels rendered below the breadcrumb on every page that has keyboard-only actions.

## Format

`[hotkey] Action Label` — each button is a `zone.Mark("action-{key}", ...)` clickable region with a semantic background color.

## Placement

Below breadcrumb, with blank line before and after. Use `renderActionButtons()` from `pagehelpers.go`.

```go
s += renderActionButtons(
    ActionButton{"a", "Create Loadout", "action-a", actionBtnAddStyle},
    ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
) + "\n"
```

## Color Mapping

| Action category | Style | Examples |
|----------------|-------|---------|
| Create/Add/Install | `actionBtnAddStyle` (green) | Create Loadout, Add Content, Install |
| Remove | `actionBtnRemoveStyle` (red) | Remove |
| Uninstall | `actionBtnUninstallStyle` (orange) | Uninstall |
| Sync | `actionBtnSyncStyle` (purple) | Sync |
| Utility | `actionBtnDefaultStyle` (gray) | Copy, Save, Env Setup, Share |

## Mouse Wiring

Action button zones follow the pattern `"action-{key}"`. In `app.go`, check the zone in the appropriate screen's left-click handler and synthesize the corresponding key message:

```go
if zone.Get("action-a").InBounds(msg) {
    return a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
}
```

## What Gets Buttons

All pages with keyboard-only actions: loadout cards, loadout items, library cards, library items, items list, registries, detail view, sandbox. NOT: homepage content cards, import wizard, settings (these are either read-only or single-purpose forms).

## What Does NOT Get Buttons

- `H` toggle hidden
- `?` help overlay
- `/` search
- Navigation keys (arrows, Enter, Esc)
```

**Success criteria:**
- New rule file describes the complete action button pattern
- Color mapping matches what was implemented in `styles.go`

**Dependencies:** Task 1.1, Task 1.2

---

### Task 3.4 — Update docs/design/tui-spec.md with Create Loadout screen spec

**File:** `docs/design/tui-spec.md`

Add a new section for the Create Loadout screen. Find the existing "Loadout Cards" section and add a "Create Loadout Screen" subsection after it:

```markdown
### Create Loadout Screen

**Route:** `a` on Loadout Cards or Loadout Items list
**Screen enum:** `screenCreateLoadout`
**File:** `cli/internal/tui/loadout_create.go`

Full-screen wizard in the content pane (sidebar visible). Steps rendered in the left pane of a persistent split-view. The right pane shows a file preview when a cursor item has a primary file (items and review steps).

#### Steps

| Step | Left pane | Right pane |
|------|-----------|------------|
| Provider picker | Cursor list of providers | Empty |
| Type selection | Checkbox list with `!!` badges on Hooks/MCP | Empty |
| Per-type items | Checkbox list + search | Primary file of cursor item |
| Name/description | Text inputs | Empty |
| Destination | Radio list | Empty |
| Review | Summary + navigable item list | Primary file of cursor item |

#### Security Indicators

- **Type selection step:** Hooks and MCP Config type headers show `!!` badge in `warningStyle`
- **Review step:** Hooks and MCP items in the item list show `!!` badge; navigating to them loads their primary file in the right pane

#### Navigation

- `Esc` on first step → back to previous screen
- `Esc` on later steps → previous step
- `Enter` → next step
- After Create on review step → `doCreateLoadoutMsg` → navigate to new loadout's detail view

#### Breadcrumb

`Home > Loadouts > Create` (type name appended on per-type item steps: `Home > Loadouts > Rules`)
```

**Success criteria:**
- tui-spec.md has a complete Create Loadout Screen section

**Dependencies:** Phase 2 complete

---

### Task 3.5 — Update styles.go comments

**File:** `cli/internal/tui/styles.go`

Add a comment block before the action button styles added in Task 1.1:

```go
// Action button styles (page-level, rendered by renderActionButtons() in pagehelpers.go)
// Used on: loadout cards, loadout items, library cards, library items,
//           items list, registries, detail view.
// Color semantics: green=add/create, red=remove, orange=uninstall, purple=sync, gray=utility.
var (
    actionBtnAddStyle      = ...
    actionBtnRemoveStyle   = ...
    actionBtnUninstallStyle = ...
    actionBtnSyncStyle     = ...
    actionBtnDefaultStyle  = ...
)
```

**Success criteria:**
- styles.go comments explain the purpose and usage of each action button style

**Dependencies:** Task 1.1

---

## Commit Sequence

Each task should be its own commit with a message following the pattern:

```
feat(tui): <short description>
```

Examples:
- `feat(tui): add action button styles to styles.go`
- `feat(tui): add renderActionButtons() helper to pagehelpers.go`
- `feat(tui): add action buttons to loadout card and items pages`
- `feat(tui): add createLoadoutScreen struct and constructor`
- `feat(tui): implement createLoadoutScreen keyboard handling`
- `feat(tui): implement createLoadoutScreen split-view rendering`
- `feat(tui): wire screenCreateLoadout into App routing`
- `feat(tui): remove createLoadoutModal, replace with screen`
- `feat(tui): add golden tests for wizard steps at all sizes`
- `docs(rules): add action button pattern rule file`
- `docs(tui-spec): add Create Loadout screen spec`

---

## Task Dependency Graph

```
Phase 1:
  1.1 (styles)
    └── 1.2 (renderActionButtons)
          ├── 1.3 (loadout cards buttons)       — temporary modal wiring; updated in 2.7
          ├── 1.4 (loadout items buttons)        — temporary modal wiring; updated in 2.7
          ├── 1.5 (library cards buttons)
          ├── 1.6 (library items buttons)
          ├── 1.7 (registries buttons)
          ├── 1.8 (detail buttons)
          └── 1.8.5 (sandbox buttons)
                └── 1.9 (golden baselines for all)

Phase 2:
  2.1 (screen enum)
    └── 2.2 (struct + constructor)
          └── 2.3 (keyboard Update)
                └── 2.4 (View left pane)
                      └── 2.5 (split-view rendering)
                            └── 2.6 (preview loading)
                                  └── 2.7 (App routing entry points)
                                        └── 2.8 (doCreateLoadoutFromScreen)
                                              └── 2.9 (mouse support)
                                                    └── 2.10 (delete old modal)
                                                          └── 2.11 (review step item list)
                                                                └── 2.12 (helpText + footer)
                                                                      └── 2.13 (golden tests)

Phase 3 (all depend on Phase 2 complete):
  3.1 (tui-page-pattern.md)
  3.2 (tui-keyboard-bindings.md)
  3.3 (tui-action-buttons.md) — also needs 1.1, 1.2
  3.4 (tui-spec.md)
  3.5 (styles.go comments) — also needs 1.1
```
