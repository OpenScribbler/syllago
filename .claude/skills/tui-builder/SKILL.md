---
name: tui-builder
description: Use when building, modifying, or debugging the syllago TUI. Provides component patterns, visual design system, architecture decisions, testing strategy, and layout rules based on research into 10+ production Go TUI applications. TRIGGER when working on any file in cli/internal/tui/.
---

# TUI Builder — Bubble Tea + Lip Gloss Reference

Complete reference for building syllago's TUI. Based on deep research into superfile, gh-dash, soft-serve, lazygit, k9s, huh, pug, and the Charm ecosystem. Updated after each build phase.

**Research docs** (for deeper details):
- `docs/research/go-tui-patterns.md` — architecture and libraries
- `docs/research/tui-visual-patterns.md` — visual design patterns
- `docs/research/tui-testing-patterns.md` — testing strategy
- `docs/research/tui-messaging-patterns.md` — error/success/loading UX

---

## Architecture

### Model Ownership (Top-Down Tree)

Root model owns child models as struct fields. `Update()` routes messages to the active child. `View()` composes children's rendered strings.

```go
type App struct {
    topBar    topBarModel     // two-tier tab navigation
    metadata  metadataModel
    explorer  explorerModel   // items list + content zone
    gallery   galleryModel    // card grid + contents sidebar
    helpBar   helpBarModel
    modal     *modalModel     // nil when not shown
    toast     *toastModel     // nil when not shown
    focus     focusZone
    width, height int
}
```

### Message Routing Priority

1. **Global keys** (Ctrl+C quit, ? help) — always handled first
2. **Modal** — when active, consumes ALL input except global keys
3. **Toast** — error toasts: Esc dismisses, c copies. Success: any key dismisses + passes through
4. **Focused panel** — only the focused component receives remaining input

### Focus System

```go
type focusZone int
const (
    focusTopBar focusZone = iota
    focusItems
    focusContent
    focusGallery
    focusContents  // gallery contents sidebar
)
```

Tab cycles focus between zones. Modal/toast override focus when active.

### Component Interface

```go
type Component interface {
    Init() tea.Cmd
    Update(tea.Msg) (Component, tea.Cmd)
    View() string
    SetSize(width, height int)
}
```

Parent calls `SetSize()` on children during `WindowSizeMsg` handling. Children never calculate their own size.

---

## Navigation — Two-Tier Tabs

**Dropdowns were abandoned** — they're a GUI pattern that fights the terminal. The topbar uses a two-tier tab bar inside a bordered frame instead.

### Layout

```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Library     Registries     Loadouts              [a] Add      [n] Create   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

- **Row 1 (groups):** `[1] Collections  [2] Content  [3] Config` — centered, button-styled with backgrounds
- **Separator:** `├────────┤` connecting to left/right border
- **Row 2 (sub-tabs + actions):** sub-tabs left-aligned (text-only), action buttons right-aligned (background-styled)
- **Top border:** `╭──syllago──...╮` with colored logo inline (mint `syl` + viola `lago`)
- **Height:** always 5 lines (border + groups + separator + tabs + border)

### Groups and Sub-Tabs

| Group | Hotkey | Sub-Tabs | Actions |
|-------|--------|----------|---------|
| Collections | `1` | Library, Registries, Loadouts | [a] Add, [n] Create |
| Content | `2` | Skills, Agents, MCP, Rules, Hooks, Commands | [a] Add, [n] Create |
| Config | `3` | Settings, Sandbox | (none) |

**Default on launch:** Collections > Library (everything at a glance).

### Keyboard

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch group (resets sub-tab to first) |
| `Tab` / `Shift+Tab` | Cycle sub-tabs within active group (wraps) |
| `h` / `l` (or ←/→) | Switch pane focus (items ↔ preview) |
| `j` / `k` (or ↑/↓) | Navigate items list / scroll preview |
| `a` | Add action (context-sensitive to current group+tab) |
| `n` | Create action (context-sensitive to current group+tab) |
| `r` | Rename selected item |
| `/` | Search/filter (Library table) |
| `s` / `S` | Cycle sort column / reverse sort (Library table) |
| `PgUp` / `PgDn` | Page navigation in any scrollable pane |
| `Enter` | Drill into item detail / select file |
| `Esc` | Close detail view / cancel search |

### Mouse

Click any group tab, sub-tab, or action button. Zone IDs: `group-N`, `tab-G-N`, `btn-add`, `btn-create`.

### Messages

```go
tabChangedMsg{group, tab, tabLabel}   // group or sub-tab changed
actionPressedMsg{action, group, tab}  // action button activated
```

---

## Component Patterns

### Hotkey Labels — Brackets Standard

**All hotkeys use square brackets:** `[1]`, `[a]`, `[n]`, `[i]`, `[esc]`. Never parentheses. This is the universal pattern throughout the TUI.

```go
// Right:  [a] Add    [n] Create    [1] Collections
// Wrong:  (a) Add    a: Add        + Add
```

### Buttons — Background-Color Blocks

No borders. `Background()` + `Padding(0, 2)`. Include bracketed hotkey.

```go
activeButtonStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
    Background(accentColor).
    Padding(0, 2).
    MarginRight(1)
```

### Group Tabs — Button-Style with Backgrounds

Higher-level navigation uses background colors to differentiate from text-only sub-tabs:

```go
activeGroupStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
    Background(primaryColor). // cyan
    Padding(0, 2)

inactiveGroupStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#575653", Dark: "#B7B5AC"}).
    Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"}).
    Padding(0, 2)
```

### Sub-Tabs — Text-Only

Lower-level navigation within a group. No backgrounds.

```go
activeTabStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(primaryColor). // cyan
    Padding(0, 2)

inactiveTabStyle = lipgloss.NewStyle().
    Faint(true).
    Padding(0, 2)
```

### Selected Row — Full-Width Background Fill

```go
selectedRowStyle = lipgloss.NewStyle().Background(selectedBG).Bold(true)
```

### Panel Borders — borderedPanel() Helper

Use `borderedPanel()` from `styles.go` for all bordered panels. It wraps lipgloss `Border()` with both `Width`/`MaxWidth` and `Height`/`MaxHeight` to guarantee exact dimensions (no wrapping, no overflow).

```go
borderedPanel(content, innerW, innerH, borderFgColor)
```

Focus indicated by border foreground color: `focusedBorderFg` (cyan) vs `unfocusedBorderFg` (gray).

**CRITICAL**: Never use raw `lipgloss.Width().Height().Render()` for bordered panels — `Width()` wraps (doesn't truncate) and `Height()` pads (doesn't clamp). This causes layout overflow that pushes the header offscreen.

### Modals — Text Input Modal Pattern

- Centered overlay with rounded border in accent color
- Text input field + Cancel/Save buttons
- Full keyboard support: type to edit, Enter confirms, Esc cancels, Tab switches fields
- Full mouse support: click buttons, click field to focus
- When modal is active, it consumes ALL input except Ctrl+C

### Toasts — Below Topbar

- Success: green, auto-dismiss 3s, any key passes through
- Warning: amber, auto-dismiss 5s
- Error: red bold, persists until Esc/c, `c` copies to clipboard

### Empty States

Always include guidance: `"No skills yet. Press [a] to add your first one."`

### Loading States

Three-state pattern (REQUIRED): loading → empty → results.

---

## Visual Design System

### Color Palette — Flexoki

All colors from the [Flexoki](https://stephango.com/flexoki) palette. Light uses -600 values, dark uses -400 values.

**Syllago brand colors** (logo only — do not use elsewhere):
| Name | Light | Dark |
|------|-------|------|
| Logo mint | `#047857` | `#6EE7B7` |
| Logo viola | `#6D28D9` | `#C4B5FD` |

**Theme colors** (Flexoki — use for everything except logo):
| Name | Role | Light | Dark | Flexoki Source |
|------|------|-------|------|----------------|
| Primary | Active tabs, headings, section titles | `#24837B` | `#3AA99F` | cyan-600/400 |
| Accent | Focus borders, buttons, selection | `#5E409D` | `#8B7EC8` | purple-600/400 |
| Muted | Help text, inactive, separators | `#6F6E69` | `#878580` | base-600/500 |
| Success | Installed, success toasts | `#66800B` | `#879A39` | green-600/400 |
| Danger | Error toasts, error borders | `#AF3029` | `#D14D41` | red-600/400 |
| Warning | Warning toasts, update badge | `#BC5215` | `#DA702C` | orange-600/400 |

**Structural colors** (Flexoki base tones):
| Name | Role | Light | Dark | Flexoki Source |
|------|------|-------|------|----------------|
| Border | Panel borders, separators | `#CECDC3` | `#343331` | base-200/850 |
| Selected BG | Row/tab selection | `#E6E4D9` | `#343331` | base-100/850 |
| Modal BG | Modal background | `#F2F0E5` | `#282726` | base-50/900 |
| Primary text | Body text | `#100F0F` | `#CECDC3` | black/base-200 |

**Rules:**
- Never use raw hex in code — define named variables in `styles.go`
- No emojis — use colored text symbols
- Check existing palette before adding colors
- New colors MUST come from the Flexoki extended palette

### Typography

- **Bold** for titles, active group tabs, active sub-tabs, selected items
- **Faint** for inactive tabs, help text, muted elements
- **Underline** — avoid

---

## Layout Rules

### lipgloss Composition

```go
body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
full := lipgloss.JoinVertical(lipgloss.Left, topbar, body, helpbar)
```

### Critical Dimension Rules

1. Subtract frame size from content width
2. Set `Width()` explicitly — lipgloss won't auto-wrap
3. Propagate `WindowSizeMsg` to ALL children
4. Use `lipgloss.Width()` not `len()` — ANSI sequences add invisible bytes
5. Parent calculates child sizes, calls `SetSize()` on each child

### Responsive Breakpoints

| Width | Behavior |
|-------|----------|
| < 80 | "Terminal too small" warning |
| 80-119 | Standard two-pane layout, Library table drops Description column |
| 120+ | Full layout with all columns including Description |

---

## Testing Strategy

### Three-Layer Pyramid

| Layer | What | Speed | When |
|-------|------|-------|------|
| **Unit** | `Update()` calls, state transitions | ~ms | Every component, every phase |
| **Golden** | `View()` comparison, layout regression | ~ms | Each screen at 60x20, 80x30, 120x40 |
| **Integration** | `teatest`, async workflows | ~sec | Phase 7+ (modals, install workflow) |

### Deterministic Output Setup (REQUIRED)

```go
func TestMain(m *testing.M) {
    lipgloss.SetColorProfile(termenv.Ascii)
    lipgloss.SetHasDarkBackground(true)
    zone.NewGlobal()
    os.Setenv("NO_COLOR", "1")
    os.Setenv("TERM", "dumb")
    _ = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000", Dark: "#fff"}).Render("warmup")
    os.Exit(m.Run())
}
```

### Golden File Pattern

Strip ANSI before storing. `requireGolden()` and `normalizeSnapshot()` in `testhelpers_test.go`.

Naming: `{component}-{variant}-{width}x{height}.golden`

### Test Helpers

```go
func testApp(t *testing.T) App              // empty catalog, 80x30
func testAppSize(t *testing.T, w, h int) App
func keyRune(r rune) tea.KeyMsg
func keyPress(k tea.KeyType) tea.KeyMsg
func pressN(app tea.Model, key tea.Msg, n int) tea.Model
func assertContains(t *testing.T, view, substr string)
```

---

## Mouse Support

Every interactive element supports both keyboard AND mouse via `lrstanley/bubblezone`:

```go
zone.Mark("item-0", itemContent)  // in View()
zone.Scan(fullOutput)             // in root View()
zone.Get("item-0").InBounds(msg)  // in Update()
```

Zone IDs: `group-N`, `tab-G-N`, `btn-add`, `btn-create`, `item-N`, `modal-zone`

---

## Gotchas

- **AdaptiveColor mutates renderer state.** Fix: warmup render in `TestMain()`.
- **bubbletea v1 uses `tea.KeyMsg`**, not `tea.KeyPressMsg` (v2 API). Check version before using spec code.
- **Golden files with raw ANSI are fragile.** Strip ANSI before storing.
- **`lipgloss.Width()` is ANSI-aware, `len()` is not.** Always use `lipgloss.Width()` for rendered strings.
- **Children that never receive WindowSizeMsg render at zero size.**
- **goimports strips "unused" imports between edits.** Add import and usage in a single Edit call.
- **Cursor initialization with Reset():** When `active = -1`, `Open()` must default cursor to 0, not -1.
- **lipgloss Width() wraps, MaxWidth() truncates.** For bordered panels, always use both Width+MaxWidth and Height+MaxHeight. Without MaxHeight, content overflow pushes the header offscreen.
- **Manual string splitting destroys zone markers.** Never split rendered strings containing zone.Mark() on newlines or truncate by rune — the invisible zero-width markers get broken. Use lipgloss styling for dimension control instead.
- **Sort indicators overflow short columns.** "Files ▲" is 7 visual chars but the Files column is 5. Use `headerCell()` which truncates the label to make room for the indicator within the column width.
- **App-level keys intercept search input.** When the library search input is active, keys like 'a' (add), 'q' (quit), '1' (group switch) must be passed through to the search handler instead of triggering app shortcuts. Check `table.searching` before handling global letter keys.
- **Help bar separator is middle dot (·)** not asterisk (*). Cleaner look.
- **Metadata bar steals table height.** When adding a fixed-height panel below a variable-height component (like table + metadata bar), always reduce the variable component's height in both `SetSize()` AND `View()`. If only one is updated, the table renders at the wrong height on resize vs initial draw.
- **Modal `lipgloss.Place()` centering.** Use `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, rendered)` for modal centering — not manual padding math. Handles terminal resize automatically.
- **YAML folded scalars add trailing newlines.** Descriptions from `loadout.yaml` using `>` have `\n` at the end. Always sanitize catalog text fields before rendering in the TUI.
- **Unified frame requires manual border construction.** `borderedPanel()` can't create shared borders between sections. Build frames manually: `╭`/`╰` for corners, `├──┤` for internal separators, `├──┬──┤`/`╰──┴──╯` for split pane junctions.
- **Metadata panel height is constant (metaBarLines=3 + 1 separator).** Type-specific line 3 is blank for simple types, not omitted. This prevents the frame from shifting when scrolling between hooks and skills.
- **Explorer needs providers for metadata.** Pass providers + repoRoot to `newExplorerModel()` so it can compute installed status via `computeMetaPanelData()`.

---

## Phase Log

*Updated after each build phase with lessons learned and pattern adjustments.*

### Pre-Phase (Research) — 2026-03-24
- Surveyed 10+ Go TUI projects for patterns
- Identified dropdown gap in Bubble Tea ecosystem
- Established three-layer testing pyramid
- Created research docs

### Phase 1 (Shell + Styles + Tests) — 2026-03-24
- styles.go, keys.go, helpbar.go, app.go — foundation built
- TestMain warmup render is critical for AdaptiveColor stability
- bubbletea v1 uses `tea.KeyMsg`, not `tea.KeyPressMsg` — spec had wrong API
- `gofmt` only changes tab alignment in comments, no functional impact
- NewApp signature must match main.go callsite exactly

### Phase 2 (Topbar + Navigation) — 2026-03-24
- **Dropdowns abandoned** — fought terminal strengths. Replaced with two-tier tabs.
- Two-tier pattern: group tabs (button-style) + sub-tabs (text-only)
- Bordered frame with `╭──syllago──╮` inline logo, `├────┤` separator
- Brackets `[N]` are the standard hotkey indicator, not parentheses
- Collections is group [1] (default), Content is [2] — Library is the landing page
- Flexoki color scheme adopted for all non-logo colors
- Active tabs use cyan foreground (Flexoki primary), not background highlight
- Group tabs use button-style backgrounds for visual hierarchy over sub-tabs
- Inactive group contrast: base-700 text on base-150 bg (light), base-300 on base-800 (dark)
- Action buttons are context-sensitive per group, right-aligned on tab row
- `actionPressedMsg` carries group+tab context for future wizard routing

### Phase 3 (Explorer + Library Table + Naming) — 2026-03-24
- **Explorer layout**: items list (left) + preview (right) with bordered panes, focus switching
- **Library table view**: full-width table with columns: Name, Type, Scope, Files, Installed, Description
- **Drill-in detail**: Enter on Library table row opens file tree + preview; Esc returns to table
- **File tree component**: expandable directories with ▸/▾ toggles, reusable for Phase 5
- **Tab/h-l swap**: Tab cycles sub-tabs (higher-level), h/l switches panes (spatial)
- **Search**: `/` activates search input in Library table, filters by name/description/type
- **Sort**: `s` cycles sort column, `S` reverses, header shows ▲/▼ indicator
- **Scroll indicators**: "(N more above/below)" in both items lists and preview panes
- **Help bar wrapping**: 2-line help bar at 80 cols, 1-line at 120+; dynamic Height()
- **Minimum width raised to 80**: Content group's 6 tabs need the space
- **Middle dot separator**: `·` in help bar (not `*`)
- **Click-to-focus panes**: Click anywhere in a pane to focus it, scroll wheel follows mouse position
- **borderedPanel() helper**: Replaced lipgloss Width/Height with Width+MaxWidth+Height+MaxHeight for exact dimensions
- **Hook/MCP naming**: Scanner derives DisplayName from .syllago.yaml, script filenames, event+matcher. New `syllago rename` CLI command. TUI rename modal planned.
- **Key routing for search**: When search input is active, app bypasses letter shortcuts (a, q, s, 1/2/3) to let them reach the search handler

### Phase 3.5 (Naming Feature + MCP Scanner Fix) — 2026-03-25
- **Rename modal**: `textInputModal` in modal.go — centered overlay with lipgloss.Place(), background-tinted input fields (dim cyan active `inputActiveBG`, dim grey inactive `inputInactiveBG`), buttons with background+padding (no borders)
- **Clickable column headers**: Zone-marked headers (`col-name`, `col-type`, etc.) — click sorts, click again reverses
- **MCP scanner fix**: Detects provider grouping dirs (no config.json + has subdirs with config.json) and recurses — fixes mcp/<provider>/<server-name>/ layout
- **--name flag on add**: Sets DisplayName for imported items
- **Search includes DisplayName**: Renamed items are searchable by their display name
- **Modal message flow**: `openModalMsg` triggers modal open, `modalResultMsg` returns result to app

### Phase 4 (Metadata Bar) — 2026-03-25
- **Metadata bar**: 3-line panel at bottom of Library table (inside bordered panel), reserved via `metaBarHeight` constant
  - Line 1: separator line (`──────...`) using `sectionRuleStyle`
  - Line 2: display name (bold) · type · provider · file count · installed providers — dot-separated chips
  - Line 3: path (with ~ shortening) · description — dot-separated, muted
- **Height management**: Table height reduced by `metaBarHeight` when items exist; metadata bar hidden when table is empty
- **Data source**: Reads from `table.Selected()` item + pre-computed `tableRow` for installed column
- **Installed highlight**: Uses `primaryColor` (cyan) for provider abbreviations when installed
- **Path display**: `os.UserHomeDir()` for ~ shortening in rendered output

### Phase 4.5 (Metadata Panel + UX Polish) — 2026-03-25
- **Metadata panel as reusable component**: Extracted into `metapanel.go` — `renderMetaPanel()`, `computeMetaPanelData()`, `metaPanelData` struct. Used by both library and explorer views.
- **Unified bordered frame**: Metadata and content share one border (`╭ meta ├──┤ table ╰` for browse, `╭ meta ├──┬──┤ items│preview ╰──┴──╯` for detail). No separate panels stacked — shared borders via manual frame construction.
- **Fixed-width metadata fields**: Name (40), Type (14), Files (9), Origin (19), then greedy for Installed/Registry. Prevents jank when scrolling between items of different lengths.
- **Type-specific line 3**: Hooks show Event/Matcher/Handler, MCP shows Server/Command, Loadouts show Target/item counts. Other types get blank line (constant height).
- **Middle-truncation paths**: `truncateMiddle()` keeps first 2 + last 3 path segments with `…` in middle.
- **Metadata on all Content tabs**: Explorer model gains providers + repoRoot fields. Same unified frame renders on Skills, Agents, MCP, Rules, Hooks, Commands tabs.
- **Transparent modal**: `overlayModal()` centers modal on screen, background content visible above/below modal rows.
- **R refresh hotkey**: `rescanCatalog()` re-reads all content from disk. App stores `contentRoot` for re-scanning.
- **q backs out**: Only quits from Collections > Library browse. Detail view → browse, other tabs → Library.
- **Double-click drill-in**: Second click on already-selected row triggers drill-in (same as Enter).
- **[r] Rename button**: Styled button on metadata line 3, clickable zone `meta-rename`, triggers `libraryRenameMsg`.
- **Data sanitization**: `sanitizeLine()` strips \n, \r, \t from all text fields. YAML folded scalars (`>`) add trailing newlines that break table height.
- **No lipgloss Width()**: Eliminated all `Width()` calls — use `MaxWidth()` + manual space padding. `Width()` word-wraps, creating multi-line output that breaks height calculations.
