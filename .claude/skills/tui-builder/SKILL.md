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
| `h` / `l` (or left/right) | Cycle sub-tabs within active group (wraps) |
| `a` | Add action (context-sensitive to current group+tab) |
| `n` | Create action (context-sensitive to current group+tab) |

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

### Panel Focus — Border Color Only

```go
focusedPanelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).BorderForeground(accentColor)
unfocusedPanelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).BorderForeground(borderColor)
```

### Modals — Centered with lipgloss.Place

- Rounded border in accent color, fixed width 56, max height terminal-4
- Buttons at bottom, Esc always dismisses, click outside dismisses
- Default focus on Cancel (the safe option)

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
