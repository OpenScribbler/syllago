# TUI Phase 6: Gallery Grid + Registry Validation

Two deliverables: a new Gallery Grid layout for collection views (Loadouts, Registries) and backend registry validation that warns on unnamed hooks/MCP items.

**Depends on:** Phases 1-5 (all complete). No dependency on Phase 7 modals — actions in Phase 6 are read-only browsing; apply/remove/install actions come in Phase 7.

---

## Part A: Gallery Grid Layout

### What Changes

The Loadouts and Registries sub-tabs under Collections currently render using the explorer layout (items list + preview split). Phase 6 replaces these with a **Gallery Grid** — visual cards arranged in a responsive grid with a contents sidebar.

Library stays as-is (full-width sortable table with drill-in).

### Layout

```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Library     Registries     Loadouts              [a] Add      [n] Create   │
╰──────────────────────────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────╮
│ Python-Web                                                                   │
│ Loadout  local  7 items  Target: Claude Code  Status: not applied            │
│                                                                              │
├──────────────────────────────────────────────────┬───────────────────────────┤
│ Loadouts (3)                                     │ Contents (7)              │
│                                                  │                           │
│ ╭──────────────────╮  ╭──────────────────╮       │ Skills                    │
│ │ > Python-Web     │  │   React-Frontend │       │   Refactor-Python         │
│ │   4 Skills       │  │   6 Skills       │       │   Py-Doc-Gen              │
│ │   2 Rules        │  │   1 Agent        │       │   Django-Patterns         │
│ │   1 Agent        │  │   2 MCP Servers  │       │   Test-Generator          │
│ │   Target: CC     │  │   Target: Cursor │       │ Rules                     │
│ ╰──────────────────╯  ╰──────────────────╯       │   Strict-Types            │
│                                                  │   PEP8-Lint               │
│ ╭──────────────────╮                             │ Agents                    │
│ │   Go-Backend     │                             │   Code-Reviewer           │
│ │   3 Skills       │                             │                           │
│ │   4 Rules        │                             │                           │
│ │   Target: GC     │                             │                           │
│ ╰──────────────────╯                             │                           │
├──────────────────────────────────────────────────┴───────────────────────────┤
╰──────────────────────────────────────────────────────────────────────────────╯
arrows grid  enter select  tab grid/contents  ? help  q quit     syllago v0.7.0
```

The layout has three zones inside a unified bordered frame:

1. **Metadata panel** (top, 3 lines) — info about the selected card
2. **Card grid** (left, ~70%) — visual cards in a responsive grid
3. **Contents sidebar** (right, ~30%) — items inside the selected card, grouped by type

### Architecture

**New files:**

| File | Model | Purpose |
|------|-------|---------|
| `gallery.go` | `galleryModel` | Orchestrates card grid + contents sidebar, owns the bordered frame |
| `cards.go` | `cardGridModel` | Responsive card grid with arrow navigation |
| `contents.go` | `contentsSidebarModel` | Shows items inside selected card, grouped by type |

**Integration with app.go:**

- `App` gets a `gallery galleryModel` field alongside `library` and `explorer`
- `renderContent()` dispatches to `gallery.View()` for Loadouts and Registries tabs
- `routeKey()` and `routeMouse()` dispatch to gallery when active
- `refreshContent()` populates gallery data from catalog

**Message types:**

```go
cardSelectedMsg{card}    // card selection changed, update metadata + sidebar
cardDrillMsg{card}       // Enter on a card — bridge to explorer filtered view
```

### Card Grid Model

```go
type cardGridModel struct {
    cards   []cardData   // all cards
    cursor  int          // selected card index
    cols    int          // cards per row (responsive)
    offset  int          // scroll offset (in rows)
    width   int
    height  int
    focused bool
}

type cardData struct {
    name        string
    subtitle    string              // URL for registries, target provider for loadouts
    counts      map[string]int      // type -> count (e.g., "Skills": 4, "Rules": 2)
    status      string              // "applied", "not applied", "up to date", "outdated"
    items       []catalog.ContentItem // items inside this card
}
```

**Responsive grid:**

| Width | Columns | Card Width |
|-------|---------|------------|
| 120+  | 3       | ~28 chars  |
| 80-119| 2       | ~28 chars  |
| <80   | 1       | full width |

**Navigation:** Arrow keys move cursor through the grid (left/right within row, up/down between rows). `Home`/`End` jump to first/last. Selection wraps.

**Rendering:** Each card is a bordered box:
```
╭──────────────────╮
│ Python-Web       │   <- name (bold, accent border if selected)
│   4 Skills       │   <- item counts by type
│   2 Rules        │
│   1 Agent        │
│   Target: CC     │   <- subtitle (muted)
╰──────────────────╯
```

Selected card gets `accentColor` border. Others get `borderColor`.

### Contents Sidebar Model

```go
type contentsSidebarModel struct {
    groups  []contentGroup  // items grouped by type
    cursor  int             // for future drill-in
    offset  int
    width   int
    height  int
    focused bool
}

type contentGroup struct {
    typeName string              // "Skills", "Rules", etc.
    items    []catalog.ContentItem
}
```

Updates live as the card selection changes. Shows type headers (bold, primary color) with item names indented below. Scrollable if content exceeds height.

### Metadata Panel for Gallery

Reuse `renderMetaPanel` from `metapanel.go` — but for cards instead of individual items. The metadata panel needs to accept card-level data:

**Loadout cards:**
- Line 1: Name, "Loadout", item count, target provider
- Line 2: Status (applied/not applied), source (local/registry)
- Line 3: Description (from loadout manifest)

**Registry cards:**
- Line 1: Name, "Registry", item count, URL
- Line 2: Last sync time, sync status (up to date/outdated)
- Line 3: Description (from registry metadata)

This may require a `galleryMetaData` struct or extending `metaPanelData` to handle card-level info.

### Loadout Cards Data Source

Loadouts come from `catalog.ByType(catalog.Loadouts)`. Each loadout item has:
- `Name` — loadout name
- `Path` — directory containing `manifest.yml`
- `Files` — files in the loadout directory

To populate card data, parse the manifest (`loadout.Parse(path)`) to get:
- Target provider
- Item references (names + types) for counts
- Description

### Registry Cards Data Source

Registries come from `app.registrySources` (passed from main.go). Each `catalog.RegistrySource` has:
- `Name` — registry name
- `URL` — git URL
- `LocalPath` — cloned location

To populate card data:
- Count items per type from `catalog.Items` filtered by `item.Registry == name`
- Sync status from `gitutil` (check if local is behind remote)

### Keyboard

| Key | Action |
|-----|--------|
| Arrow keys | Navigate card grid |
| `Tab` | Switch focus: card grid <-> contents sidebar |
| `Enter` | Drill into card (bridge to explorer with filter) |
| `Home` / `End` | First / last card |
| `j` / `k` | Scroll contents sidebar (when focused) |

### Mouse

| Element | Action |
|---------|--------|
| Card | Click to select, double-click to drill in |
| Contents sidebar item | Click to drill into that specific item type |
| Scroll wheel on grid | Scroll cards |
| Scroll wheel on sidebar | Scroll contents |

---

## Part B: Registry Validation

### What Changes

Add warnings for hooks and MCP items that have no meaningful display name. This catches content that would show up as generic entries in the TUI (e.g., "before_tool_execute hook for *" instead of a descriptive name).

### Where It Runs

1. **During catalog scan** — `catalog.ScanWithGlobalAndRegistries()` collects warnings into `cat.Warnings`
2. **`syllago doctor` command** — add a "naming quality" check that reports unnamed hooks/MCP
3. **TUI** — show warning count in the help bar or as a toast on startup if warnings exist

### Implementation

**In `cli/internal/catalog/scanner.go`**, after `applyMetaOverrides()`:

```go
// After scanning completes, check for unnamed items
func (c *Catalog) checkNamingWarnings() {
    for _, item := range c.Items {
        if item.Type != Hooks && item.Type != MCP {
            continue
        }
        if item.DisplayName == "" || item.DisplayName == item.Name {
            c.Warnings = append(c.Warnings, fmt.Sprintf(
                "%s %q has no display name — add a name field to .syllago.yaml",
                item.Type, item.Name,
            ))
        }
    }
}
```

**In `cli/cmd/syllago/doctor_cmd.go`**, add a naming quality check section:

```go
// Check: Unnamed hooks/MCP
unnamed := 0
for _, item := range cat.Items {
    if (item.Type == catalog.Hooks || item.Type == catalog.MCP) &&
       (item.DisplayName == "" || item.DisplayName == item.Name) {
        unnamed++
        if verbose {
            fmt.Fprintf(w, "  %s %s: no display name\n", item.Type, item.Name)
        }
    }
}
if unnamed > 0 {
    printWarn(w, fmt.Sprintf("%d hooks/MCP items have no display name", unnamed))
}
```

### What Counts as "Unnamed"

A hook or MCP item is considered unnamed if:
- `DisplayName` is empty, OR
- `DisplayName` equals `Name` (meaning no override was set — the scanner just used the directory name)

Items with heuristic-derived names (from script filenames or event+matcher) are NOT flagged — they have *some* meaningful name, even if not ideal.

### Testing

- Add test cases to `scanner_test.go` for hooks/MCP with and without `.syllago.yaml` names
- Add test case to `doctor_cmd_test.go` for the naming quality check
- Verify warnings don't fire for items with heuristic-derived display names

---

## Files to Create

```
cli/internal/tui/
  gallery.go     — galleryModel: bordered frame + card grid + contents sidebar
  cards.go       — cardGridModel: responsive card layout + navigation
  contents.go    — contentsSidebarModel: grouped item list for selected card
```

## Files to Modify

```
cli/internal/tui/
  app.go         — add gallery field, dispatch to gallery for Loadouts/Registries tabs
  keys.go        — no changes (arrow keys and Tab already defined)

cli/internal/catalog/
  scanner.go     — add checkNamingWarnings() call after scan
  types.go       — add Warnings field to Catalog if not present

cli/cmd/syllago/
  doctor_cmd.go  — add naming quality check section
```

## Build Order

1. **Card data model** — `cardData` struct, loadout manifest parsing for counts
2. **Card grid** — `cards.go` with responsive layout, arrow navigation, rendering
3. **Contents sidebar** — `contents.go` with grouped item display
4. **Gallery model** — `gallery.go` orchestrating grid + sidebar + bordered frame + metadata
5. **App integration** — wire gallery into `app.go` for Loadouts/Registries tabs
6. **Registry validation** — scanner warnings + doctor check
7. **Tests** — unit tests for card navigation, golden files for gallery views

## Done Criteria

- [ ] Loadouts tab shows gallery grid with card per loadout
- [ ] Registries tab shows gallery grid with card per registry
- [ ] Cards show name, item counts by type, target/URL
- [ ] Arrow keys navigate cards, Tab switches to contents sidebar
- [ ] Contents sidebar updates live on card selection
- [ ] Selected card has accent border
- [ ] Responsive: 3 cols at 120+, 2 cols at 80-119, 1 col stacked at <80
- [ ] Metadata panel shows card-level info (reuses metapanel pattern)
- [ ] `syllago doctor` reports unnamed hooks/MCP items
- [ ] Scanner collects naming warnings during scan
- [ ] All existing tests still pass
- [ ] Golden files created for gallery at 80x30 and 120x40
- [ ] `make build` succeeds
- [ ] `make fmt` produces no changes
