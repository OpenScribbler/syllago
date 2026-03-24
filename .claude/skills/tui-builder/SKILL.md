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
- `docs/research/tui-messaging-patterns.md` — error/success/loading UX (pending)

---

## Architecture

### Model Ownership (Top-Down Tree)

Root model owns child models as struct fields. `Update()` routes messages to the active child. `View()` composes children's rendered strings. This is the standard pattern used by superfile, gh-dash, and most Bubble Tea apps of similar complexity.

```go
type App struct {
    topBar    topBarModel
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

Every project follows the same priority. No exceptions:

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

Every TUI component follows this pattern (adapted from Soft Serve):

```go
type Component interface {
    Init() tea.Cmd
    Update(tea.Msg) (Component, tea.Cmd)
    View() string
    SetSize(width, height int)
}
```

Parent calls `SetSize()` on children during `WindowSizeMsg` handling. Children never calculate their own size.

### Shared Context

Carry dimensions and styles to all components without message passing:

```go
type Common struct {
    Width, Height int
    Styles        *Styles
    Zone          *zone.Manager  // bubblezone for mouse hit-testing
}
```

---

## Component Patterns

### Buttons — Background-Color Blocks

No borders. `Background()` + `Padding(0, 2)`. Include hotkey inline.

```go
ActiveButton = lipgloss.NewStyle().
    Foreground(lipgloss.Color("0")).
    Background(accentColor).
    Padding(0, 2).
    MarginRight(1)

InactiveButton = lipgloss.NewStyle().
    Foreground(lipgloss.Color("252")).
    Background(lipgloss.Color("237")).
    Padding(0, 2).
    MarginRight(1)

// Render: " (enter) Install "  " (esc) Cancel "
```

### Dropdowns — Soft Serve Message Pattern

Based on Soft Serve's `tabs.go`. The dropdown is a reusable component with open/close state:

```go
type SelectMsg int   // command: "select item N"
type ActiveMsg int   // event: "item N is now active"

type Dropdown struct {
    items     []string
    active    int
    isOpen    bool
    cursor    int
    zone      *zone.Manager
}
```

- `tab`/`shift+tab` or `j`/`k` cycles within open dropdown
- Enter selects and closes
- Esc closes without selecting
- Mouse: zone-mark each item, click to select
- When open, dropdown intercepts all keys (modal pattern)
- Render: bordered list below trigger using `lipgloss.Place()` anchored to trigger position

### Selected Row — Full-Width Background Fill

No prefix marker needed. Background alone conveys selection (gh-dash, lazygit, k9s pattern).

```go
SelectedRow = lipgloss.NewStyle().
    Background(selectedBG).  // ANSI 236 dark, adaptive for light
    Bold(true)

NormalRow = lipgloss.NewStyle()
```

### Panel Focus — Border Color Only

Border color changes on focus. Nothing else. Universal across all surveyed projects.

```go
FocusedBorder = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(accentColor)

UnfocusedBorder = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(mutedColor)

// Superfile trick: unfocused sidebar border = background color (invisible)
```

### Tab Bar — Bold + Background Fill Active

```go
ActiveTab = lipgloss.NewStyle().
    Bold(true).
    Background(selectedBG).
    Foreground(primaryText).
    Padding(0, 2)

InactiveTab = lipgloss.NewStyle().
    Faint(true).
    Padding(0, 2)

TabRow = lipgloss.NewStyle().
    BorderBottom(true).
    BorderStyle(lipgloss.ThickBorder()).
    BorderBottomForeground(borderColor)
```

### Modals — Centered with lipgloss.Place

```go
func renderModal(content string, width, height int) string {
    modal := modalStyle.Render(content)
    return lipgloss.Place(
        width, height,
        lipgloss.Center, lipgloss.Center,
        modal,
        lipgloss.WithWhitespaceBackground(lipgloss.Color("235")),
    )
}
```

- Rounded border in accent color
- Fixed width: 56 characters
- Max height: terminal height - 4
- Buttons at bottom: `JoinHorizontal(Center, confirm, cancel)`
- Esc always dismisses. Click outside dismisses.

### Toasts — Positioned Overlay

- Success: green text, `Done:` prefix, auto-dismiss 3s, any key dismisses + passes through
- Warning: amber text, `Warn:` prefix, auto-dismiss 5s
- Error: red text, `Error:` prefix, persists until Esc or c (copy), other keys pass through
- All toasts: `c` copies message to clipboard
- Position: below topbar, full width, pushes content down

### Empty States

Two distinct messages (from bubbles/list pattern):
- **No items**: `"No skills found."` — truly empty
- **No matches**: `"Nothing matched."` — filtered to empty (different style)

Include actionable hints: `"Press 'a' to add your first skill"`

### Loading States

Spinner + descriptive text. The text carries meaning, spinner is liveness indicator:
- `"Syncing team-rules..."` (spinner)
- `"Installing alpha-skill to Claude Code..."` (spinner)

**Three-state pattern (REQUIRED):** Every data-driven view must distinguish loading, empty, and results:
```go
switch {
case m.err != nil:   return renderError(m.err)
case m.loading:      return m.spinner.View() + " Loading..."
case len(m.items)==0: return renderEmptyState()
default:             return renderList(m.items)
}
```

---

## Messaging & UX

### Message Severity Hierarchy (Least to Most Disruptive)

1. **Inline indicator** — spinner/icon next to the item in the list (no interruption)
2. **Toast** — brief overlay, auto-dismisses for success (3s), persists for errors
3. **Persistent error** — stays visible until dismissed or navigated away
4. **Modal alert** — centered overlay for unexpected errors, must dismiss
5. **Confirmation modal** — must choose before proceeding (default = Cancel)
6. **Type-to-confirm** — most severe destructive actions only

**Rule:** Use the least disruptive mechanism that ensures the user gets the information.

### Error Messages — Two-Part Rule

Every error answers: **what happened** + **what to do about it**

| Bad | Good |
|-----|------|
| `"permission denied"` | `"Can't write to ~/.claude/rules/ — check directory permissions"` |
| `"network error"` | `"Can't reach github.com/team/rules — check network or try again"` |

Include error codes in visible text (syllago already has structured codes in `output/errors.go`).

### Confirmation Design

- **Default focus on Cancel** (the safe option), not the destructive action
- **Descriptive labels**: `"Uninstall"` and `"Cancel"`, not `"OK"` and `"Cancel"`
- **Include consequences**: `"This removes the symlink. Content stays in your library."`
- **`ConfirmIf` pattern**: Skip the dialog when the operation is low-risk
- **Undo > confirm** when possible: `"Uninstalled. Press 'z' to undo."` is less disruptive

### Empty State Rules

| Type | Message Pattern |
|------|----------------|
| First use | Guide: `"No skills yet. Press 'a' to add your first one."` |
| Search empty | Suggest: `"No matches for 'xyz'. Press Esc to clear."` |
| User cleared | Confirm: `"All items removed."` |
| Error/failed | Show error, NEVER show as empty |

### Help — Progressive Disclosure

1. **Footer hints** (always visible): top 3-4 keys, context-sensitive
2. **? overlay**: full keyboard reference, scrollable, grouped by category
3. **In-context guidance**: empty states explain what to do, modals explain consequences

**Deeper details:** `docs/research/tui-messaging-patterns.md`

---

## Visual Design System

### Color Palette

All colors use `lipgloss.AdaptiveColor` for light/dark terminal support:

| Name | Role | Light | Dark |
|------|------|-------|------|
| Primary (mint) | Content types, headings | `#047857` | `#6EE7B7` |
| Accent (viola) | Collections, selection, buttons | `#6D28D9` | `#C4B5FD` |
| Muted (stone) | Help text, inactive, separators | `#57534E` | `#A8A29E` |
| Success (green) | Installed, success toasts | `#15803D` | `#4ADE80` |
| Danger (red) | Error toasts, error borders | `#B91C1C` | `#FCA5A5` |
| Warning (amber) | Warning toasts, update badge | `#B45309` | `#FCD34D` |
| Border | Default panel borders | `#D4D4D8` | `#3F3F46` |
| Selected BG | Row/tab selection background | `#D1FAE5` | `#1A3A2A` |

**Rules:**
- Never use raw hex in code — define named variables in `styles.go`
- No emojis — use colored text symbols (checkmark, X, warning triangle)
- Check existing palette before adding colors

### Typography

- **Bold** for titles, active tabs, selected items
- **Faint** for inactive tabs, help text, muted elements
- **Underline** — avoid (used by Textual but not Go TUIs)

---

## Layout Rules

### lipgloss Composition

```go
// Sidebar + content
body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

// Full layout
full := lipgloss.JoinVertical(lipgloss.Left, topbar, body, helpbar)
```

### Critical Dimension Rules

1. **Subtract frame size from content width:** `contentWidth = panelWidth - style.GetHorizontalFrameSize()`
2. **Set Width() explicitly.** Lipgloss won't auto-wrap — it overflows.
3. **Propagate WindowSizeMsg to ALL children.** Children at zero size until they receive it.
4. **Use `lipgloss.Width()` not `len()`.** ANSI sequences add invisible bytes.
5. **Parent calculates child sizes** in WindowSizeMsg handler, calls `SetSize()` on each child.

### Responsive Breakpoints

| Width | Behavior |
|-------|----------|
| < 60 | "Terminal too small" warning |
| 60-79 | Stacked layout (no side-by-side panels) |
| 80-119 | Standard two-pane layout |
| 120+ | Full layout with all columns |

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
func init() {
    lipgloss.SetColorProfile(termenv.Ascii)
    lipgloss.SetHasDarkBackground(true)
    zone.NewGlobal()
    os.Setenv("NO_COLOR", "1")
    os.Setenv("TERM", "dumb")
}
```

Without this, golden files diverge between local dev and CI.

### Golden File Pattern

```go
var updateGolden = flag.Bool("update-golden", false, "update golden files")

func requireGolden(t *testing.T, name string, got string) {
    t.Helper()
    got = normalizeSnapshot(got)  // strip ANSI, normalize paths, trim whitespace
    path := filepath.Join("testdata", name+".golden")
    if *updateGolden {
        os.WriteFile(path, []byte(got), 0o644)
        return
    }
    want, _ := os.ReadFile(path)
    if string(want) != got {
        t.Errorf("golden mismatch: %s\n%s", path, diffStrings(string(want), got))
    }
}
```

**Strip ANSI before storing.** Human-readable diffs, stable across environments. Test styles separately.

### Golden File Naming

`{component}-{variant}-{width}x{height}.golden`

Examples: `shell-empty-80x30.golden`, `explorer-skills-120x40.golden`, `modal-confirm-80x30.golden`

### Test Helpers

```go
func testApp(t *testing.T) App           // 2-item catalog, 1 provider, 80x30
func testAppSize(t *testing.T, w, h int) App
func testAppLarge(t *testing.T) App       // 85+ items for overflow testing
func testAppEmpty(t *testing.T) App       // empty catalog
func keyRune(r rune) tea.KeyMsg           // shorthand for key events
func keyPress(k tea.KeyType) tea.KeyMsg
func pressN(app tea.Model, key tea.Msg, n int) tea.Model
```

### Update Workflow

```bash
cd cli && go test ./internal/tui/ -update-golden
git diff testdata/   # review every change
```

---

## Mouse Support

Every interactive element supports both keyboard AND mouse. Use `lrstanley/bubblezone`:

```go
// In View(): mark clickable regions
zone.Mark("item-0", itemContent)

// In root View(): scan for zones
zone.Scan(fullOutput)

// In Update(): check bounds
if zone.Get("item-0").InBounds(msg) {
    m.cursor = 0
}
```

Zone markers are zero-width ANSI sequences — don't affect `lipgloss.Width()`.

---

## Key Libraries

| Library | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | Elm architecture (Model-View-Update) |
| `charmbracelet/lipgloss` | Styling, layout composition |
| `charmbracelet/bubbles` | Official components (list, viewport, textinput, spinner) |
| `lrstanley/bubblezone` | Mouse hit-testing via zone markers |
| `charmbracelet/x/ansi` | ANSI stripping for golden tests |
| `charmbracelet/x/exp/teatest` | Integration test harness |

---

## Reference Projects

When you need to look up how a pattern is implemented:

| Project | Stars | Best For |
|---------|-------|----------|
| [superfile](https://github.com/yorukot/superfile) | ~17k | Panel focus (border color), prefix cursor, modal buttons |
| [gh-dash](https://github.com/dlvhdr/gh-dash) | ~11k | Tab bars, full-width row selection, side panel layout switching |
| [soft-serve](https://github.com/charmbracelet/soft-serve) | ~6.7k | Tab component code, Common struct, TabComponent interface |
| [lazygit](https://github.com/jesseduffield/lazygit) | ~60k | Context stack, centered floating menus, keybinding hints |
| [k9s](https://github.com/derailed/k9s) | ~30k | Pill breadcrumbs, prompt insertion, tview modal buttons |
| [huh](https://github.com/charmbracelet/huh) | - | Select field, Confirm buttons, form validation, themes |
| [pug](https://github.com/leg100/pug) | ~667 | PaneManager, ChildModel interface, Maker factory |

---

## Gotchas

- **AdaptiveColor mutates renderer state.** `lipgloss.AdaptiveColor` triggers `HasDarkBackground()` which mutates global state. Fix: warmup render in `init()` for deterministic test output.
- **bubblezone v2 may not be compatible with lipgloss v2 Canvas.** Check before upgrading.
- **No first-party dropdown component exists** in Bubble Tea. Build custom using the Soft Serve tab pattern adapted with isOpen state.
- **`SelectTabMsg` does NOT fire `ActiveTabMsg`** in Soft Serve — they're asymmetric. Handle both cases in parent Update().
- **Golden files with raw ANSI are fragile.** Strip ANSI before storing. Test styles separately with targeted unit tests.
- **`lipgloss.Width()` is ANSI-aware, `len()` is not.** Always use `lipgloss.Width()` for rendered strings.
- **Children that never receive WindowSizeMsg render at zero size.** Always propagate to all children.
- **goimports strips "unused" imports between edits.** Add import and usage in a single Edit call, or use a new file.

---

## Phase Log

*Updated after each build phase with lessons learned and pattern adjustments.*

### Pre-Phase (Research) — 2026-03-24
- Surveyed 10+ Go TUI projects for patterns
- Identified dropdown gap in Bubble Tea ecosystem
- Established visual design system based on gh-dash (tabs, selection) + superfile (focus, modals) + huh (buttons)
- Defined three-layer testing pyramid
- Created research docs: go-tui-patterns.md, tui-visual-patterns.md, tui-testing-patterns.md
