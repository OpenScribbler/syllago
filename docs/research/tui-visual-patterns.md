# TUI Visual Patterns — Stealable Ideas for Syllago

Practical visual patterns from real Go TUI apps (and cross-framework where useful). Focused on what things actually LOOK LIKE and how to reproduce them in lipgloss/bubbletea. Conducted 2026-03-24.

---

## Table of Contents

- [Buttons](#buttons)
- [Dropdowns / Select Menus](#dropdowns--select-menus)
- [Tab Bars / Navigation](#tab-bars--navigation)
- [Panel Focus Indicators](#panel-focus-indicators)
- [Selected Row / Cursor Highlighting](#selected-row--cursor-highlighting)
- [Modals / Dialogs](#modals--dialogs)
- [Breadcrumbs](#breadcrumbs)
- [Help / Status Bars](#help--status-bars)
- [Layout Switching Based on Selection](#layout-switching-based-on-selection)
- [Pattern Recommendations for Syllago](#pattern-recommendations-for-syllago)

---

## Buttons

There are no native button primitives in terminals. Every TUI invents its own. Four approaches exist in the wild:

### 1. Background-Color Block (most common in Go TUIs)

Used by: **huh**, **superfile**, **gh-dash**

Just a styled text block with `Background()` + `Padding()`. No border characters at all.

```
 Confirm   Cancel
```

**huh's Confirm buttons:**
```go
FocusedButton = lipgloss.NewStyle().
    Foreground(cream).       // #FFFDF5
    Background(fuchsia).     // #F780E2
    Padding(0, 2).
    MarginRight(1)

BlurredButton = lipgloss.NewStyle().
    Foreground(normalFg).
    Background(lipgloss.Color("237")).  // dark gray
    Padding(0, 2).
    MarginRight(1)
```

**superfile's modal buttons:**
```go
ModalConfirm = lipgloss.NewStyle().
    Foreground("#383838").    // dark text
    Background("#89dceb")     // light cyan

ModalCancel = lipgloss.NewStyle().
    Foreground("#383838").    // dark text
    Background("#eba0ac")     // pink/peach
```

**gh-dash's footer "buttons":**
```go
// "? help" pill — inverted colors create a button feel
lipgloss.NewStyle().
    Background(theme.FaintText).       // ANSI 245
    Foreground(theme.SelectedBackground). // ANSI 236
    Padding(0, 1).
    Render("? help")
```

**Key pattern:** Active button = accent-colored background + contrasting text. Inactive button = muted gray background. The `Padding(0, 2)` gives horizontal breathing room that makes it feel like a button rather than highlighted text.

### 2. Half-Block Border (Textual/Python — fancier)

Used by: **Textual** (Python framework, not Go, but stealable)

```
▔▔▔▔▔▔▔▔▔▔▔▔
 Button Text
▁▁▁▁▁▁▁▁▁▁▁▁
```

Uses Unicode eighth-block characters:
- `▔` (U+2594, UPPER ONE EIGHTH BLOCK) for top edge
- `▁` (U+2581, LOWER ONE EIGHTH BLOCK) for bottom edge
- `▊` (U+258A, LEFT THREE QUARTERS BLOCK) for side edges

These sit at the edge of the character cell, creating a thin raised-surface illusion. Looks nicer than box-drawing borders but requires Unicode support.

### 3. Bracket Notation (classic)

```
[ OK ]  [ Cancel ]
```

Still used in lightweight TUIs. Simple, universally supported, no color needed. Sometimes with filled markers: `[*OK*]  [ Cancel ]`.

### 4. No Buttons — Keybinding Hints Instead (lazygit)

Lazygit doesn't render button widgets at all. Confirmation dialogs show the prompt text, and the bottom helpbar shows `<enter> confirm  <esc> cancel`. The user never "clicks" a button — they press the key.

**This is the simplest approach and works well for keyboard-first TUIs.**

---

## Dropdowns / Select Menus

**Key finding: there is no working dropdown component in the Bubble Tea ecosystem.** This is a confirmed gap. Every project works around it differently.

### Approach A: Inline Always-Visible List (huh Select)

Not actually a dropdown — the options list is always rendered inline. No open/close state.

```
┃ Choose provider
┃ > Claude Code        ← cursor (green text, ">" prefix)
┃   Cursor             ← normal text, no prefix
┃   Windsurf
┃   Copilot
```

Visual elements:
- `┃` thick left border = focused field indicator (`ThickBorder().BorderLeft(true)`)
- `> ` prefix = cursor indicator (fuchsia in ThemeCharm, `SelectSelector` style)
- Selected option text = green (`SelectedOption` style)
- Unselected option text = normal fg (`UnselectedOption` style)
- **No background color** on cursor row — just text color + prefix icon
- Press `/` to filter (replaces title with search input)

**Lipgloss reproduction:**
```go
selectorStyle := lipgloss.NewStyle().SetString("> ").Foreground(lipgloss.Color("#F780E2"))
selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#02BF87"))
normalStyle   := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
```

### Approach B: Inline Horizontal Selector (huh Select inline mode)

Single line, left/right navigation:

```
← Claude Code →
```

Arrows are fainted at boundaries (first/last option). Good for small option sets (2-5 items).

### Approach C: Centered Floating Menu (lazygit)

When lazygit opens a menu (`?` key), it's a **centered modal overlay** with a bordered list:

```
┌─── Keybindings ──────────────────────┐
│  e    Edit file                      │
│  o    Open file                      │
│  i    Ignore file                    │
│  d    View diff                      │
└──────────────────────────────────────┘
```

- Centered at `width/2, height/2`, sized to `min(4*width/7, 90)` wide
- Single-line box border with title in top edge
- Items: `[cyan key]  [normal description]`
- Selected item: full-width background color (blue)
- Fuzzy search active — typing filters the list
- Tooltip for selected item appears below the list within the same modal

**This is effectively a dropdown that happens to be centered. The pattern is: open a bordered list over existing content, navigate with j/k, select with Enter, dismiss with Esc.**

### Approach D: Panel Replacement (gh-dash, most Charm apps)

When selection happens, the content area swaps entirely. The "dropdown" is just navigating to a different view in the same panel space. No overlay, no floating content.

### Approach E: Prompt Insertion (k9s)

k9s inserts a 3-row text input into the layout (pushes content down). Autocomplete suggestions appear as dimmed text within the same prompt area — NOT as a separate dropdown widget.

```
🐶> pod█                              ← command input
   pods (po)                          ← dimmed suggestion
```

### What Actually Works for a Topbar Dropdown

Based on this research, the most practical approach for syllago's topbar navigation dropdown is **Approach C adapted**: a bordered list that appears directly below the trigger, not centered. Think of it as a small modal anchored to the topbar rather than centered on screen.

```
 syllago  [Loadouts ▾]  provider / item-name
          ┌──────────────────┐
          │ > Loadouts       │  ← cursor row (accent bg or prefix)
          │   Library        │
          │   Registries     │
          └──────────────────┘
```

Implementation: render the dropdown list with `lipgloss.Place()` positioned at the dropdown's X offset, or use the overlay compositor. When open, intercept all keys (same as modal pattern). On selection, fire a navigation message and close.

---

## Tab Bars / Navigation

### gh-dash: Bold + Background Fill (clean, recommended)

```
  Loadouts    Library    Registries                    syllago v0.1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Active tab: **bold + `SelectedBackground` fill** (ANSI 236, dark gray).
Inactive tab: **`Faint(true)`** — dimmed text, no background.
Separator: `ThickBorder()` bottom only.

```go
ActiveTab = lipgloss.NewStyle().
    Bold(true).
    Background(lipgloss.Color("236")).
    Foreground(lipgloss.Color("15")).
    Padding(0, 2)

InactiveTab = lipgloss.NewStyle().
    Faint(true).
    Padding(0, 2)

TabRow = lipgloss.NewStyle().
    BorderBottom(true).
    BorderStyle(lipgloss.ThickBorder()).
    BorderBottomForeground(lipgloss.Color("8"))
```

### k9s: Colored Pill Breadcrumbs

Each breadcrumb is a background-filled block:

```
 pods   mynamespace   pod-name-xyz
```

Active (last) crumb uses a highlight color. Previous crumbs use a muted background. Creates a "pill" or "tag" visual.

### Textual: Underline Bar (works without color)

Active tab has a thick underline using box-drawing characters:

```
  Tab1    Tab2    Tab3
          ╸━━━━╺
```

`╸` (U+2578) and `╺` (U+257A) are half-block line ends, `━` (U+2501) is heavy horizontal. Works in monochrome — underline position alone conveys "active."

### Soft Serve: Tab Labels + Message Pattern

Tab labels are rendered inline. Uses `bubblezone` to mark each label for mouse click detection. Keyboard: `tab`/`shift+tab` cycles. Fires `ActiveTabMsg` on change.

```go
// Message pair for tab selection
type SelectTabMsg int   // command: "select this tab"
type ActiveTabMsg int   // event: "this tab is now active"
```

Parent catches `ActiveTabMsg` to update which content pane is shown.

---

## Panel Focus Indicators

### Border Color Change (universal standard)

Every surveyed project uses this. It's the one pattern that's truly universal:

**superfile:**
- Unfocused: muted gray border (`#6c7086`)
- Focused: bright accent border (`#b4befe` lavender for file panels, `#f38ba8` red for sidebar, `#a6e3a1` green for footer)
- Border STYLE stays the same (rounded single-line throughout)
- Sidebar border when unfocused = same as background color (invisible!)

**lazygit:**
- Unfocused: terminal default color (effectively invisible)
- Focused: green + bold (`activeBorderColor: [green, bold]`)

**k9s:**
- Unfocused: normal border color
- Focused: `FocusColor` (configurable)

**gh-dash:**
- Sidebar separator: single `│` left-border in `PrimaryBorder` color

### What Nobody Does

- **Double borders for focus** — not used by any surveyed project
- **Background color fill on the entire panel** — nobody does this
- **Animated focus transitions** — none

The consensus is clear: **change the border color, nothing else.** Maybe bold the border too. That's it.

---

## Selected Row / Cursor Highlighting

Two schools of thought, both work:

### Full-Width Background Fill (gh-dash, lazygit, k9s)

The selected row gets a colored background across its entire width:

**gh-dash:**
```go
SelectedCellStyle = CellStyle.Background(lipgloss.Color("236"))  // dark gray
```
Every cell in the row gets the background. No prefix marker. The background alone = selection.

**lazygit:**
- Focused panel cursor: `selectedLineBgColor` (blue, full-width)
- Unfocused panel cursor: just bold text, no background

**k9s:**
```go
tcell.StyleDefault.
    Foreground(CursorFgColor).
    Background(CursorBgColor).
    Attributes(tcell.AttrBold)
```

### Prefix Icon, No Background (superfile, huh)

The cursor is a prefix character. No background color on the row:

**superfile:**
```
  Documents/
  Downloads/
  file.txt           ← cursor: nerd font chevron prefix, cream color
  README.md
```

**huh:**
```
> Option A            ← "> " prefix in fuchsia, text in green
  Option B            ← spaces prefix, text in normal fg
```

### Recommendation

Full-width background fill is more visible and feels more polished. It's what the most-starred projects use. The prefix icon approach is subtler and works better when you need a separate multi-select indicator.

---

## Modals / Dialogs

### The Standard: Centered Box with lipgloss.Place (superfile, lazygit)

```
╭──── Create Loadout ───────────────────────╮
│                                           │
│  Name: my-loadout█                        │
│                                           │
│  Provider: Claude Code                    │
│                                           │
│   Confirm    Cancel                       │
│                                           │
╰───────────────────────────────────────────╯
```

Implementation:
```go
modalContent := renderModal(m)
return lipgloss.Place(
    m.width, m.height,
    lipgloss.Center, lipgloss.Center,
    modalContent,
    lipgloss.WithWhitespaceBackground(lipgloss.Color("235")),  // dim bg
)
```

`WithWhitespaceBackground` fills surrounding whitespace with a dark color, creating a scrim/dim effect. Without it, the background is transparent (terminal default).

**superfile's modals** skip the dimmed background — the modal just floats over the panels with the same dark background as everything else. It's simpler and still reads fine.

**lazygit's popups** are centered at computed coordinates: `width/2 - panelWidth/2`. Max width = `min(4*width/7, 90)`, max height = `3/4 * screenHeight`.

**k9s's dialogs** use tview's `ModalForm` — proper centered overlay with actual Tab-navigable button widgets.

### Modal Button Layout

Buttons in modals are always:
1. Horizontally joined at the bottom of the modal
2. Active button = accent background, inactive = muted background
3. `lipgloss.JoinHorizontal(lipgloss.Center, confirmBtn, cancelBtn)`

superfile includes the hotkey inline: `" (enter) Create "` / `" (ctrl+c) Cancel "`

---

## Breadcrumbs

### k9s Colored Pills (most visually distinct)

```
 pods   mynamespace   pod-name-xyz
```

Each segment is a background-filled block. Active = highlight color. Previous = muted color.

### lazygit Panel Titles (simplest)

Breadcrumbs ARE the panel titles. The focused panel's title turns green. No separate breadcrumb widget.

### superfile Embedded in Border

Panel titles are embedded in the top border using `├title┤` junction characters. Footer info embedded in bottom border the same way.

### Syllago's Current Approach

Breadcrumbs in the topbar: `syllago > Loadouts > claude-code > rules/my-rule`. This is fine — it's the standard web/file-manager pattern. The k9s pill style could make it more visually distinct if desired.

---

## Help / Status Bars

### gh-dash: Full-Width Footer Band

The entire bottom row has `SelectedBackground` fill. Contains:
- Left: view switcher buttons (`PRs │ Issues │ Notifications`) with active = bold
- Right: `? help` pill (inverted colors)

```go
FooterStyle = lipgloss.NewStyle().
    Background(theme.SelectedBackground).  // ANSI 236
    Height(1)
```

### lazygit: Keybinding Hints

Bottom bar shows context-sensitive keys: `<enter> confirm  <esc> cancel` in blue text. Changes based on what's focused.

### superfile: Info in Border

Footer info (sort mode, cursor position, selection count) is embedded in the panel's bottom border. No separate status bar.

### k9s: Flash Messages

A 1-row flash bar at the very bottom for status messages. Separate from the header hint bar.

---

## Layout Switching Based on Selection

### gh-dash: Side Panel Slides In

When you select a PR, a detail panel appears on the right:

```
BEFORE (no selection):
┌─────────────────────────────────────────┐
│ PR #123  Fix auth bug                   │
│ PR #124  Add caching                    │
│ PR #125  Update deps                    │
└─────────────────────────────────────────┘

AFTER (PR selected, sidebar opens):
┌──────────────────────┐│┌───────────────┐
│ PR #123  Fix auth bug│││ ## Fix auth   │
│ PR #124  Add caching ││|               │
│ PR #125  Update deps │││ Changes:      │
└──────────────────────┘│└───────────────┘
```

Implementation: `sidebar.IsOpen` bool. When open, the list shrinks to `MainContentWidth` and the sidebar gets `DynamicPreviewWidth`. Joined with `JoinHorizontal`. No animation — it snaps.

The sidebar has a single `│` left-border as separator:
```go
Sidebar.Root = lipgloss.NewStyle().
    BorderLeft(true).
    BorderStyle(lipgloss.Border{Left: "│"}).
    BorderForeground(theme.PrimaryBorder)
```

### superfile: Dynamic Panel Count

Users can open N file panels side-by-side. The layout recalculates widths when panels are added/removed. Each panel is an equal fraction of available width.

### k9s: Stack-Based View Replacement

Selecting a resource pushes a new view onto the stack. The content area completely replaces. Pressing Esc pops back. Breadcrumbs track depth.

---

## Pattern Recommendations for Syllago

Based on everything above, here's what I'd steal for each syllago TUI element:

### Topbar Navigation Dropdown

**Steal from: lazygit menu + huh Select styling**

- Trigger shows current section with `▾` indicator: `[Loadouts ▾]`
- When open: bordered list appears below trigger (not centered)
- Cursor: `> ` prefix + accent text color (no background on row)
- Selected (committed) item: text stays in the trigger area
- Close on selection or Esc
- Open state intercepts all keys (modal pattern)

### Buttons (Modal Actions)

**Steal from: huh/superfile — background-color blocks**

```go
ConfirmBtn = lipgloss.NewStyle().
    Foreground(lipgloss.Color("0")).     // dark text
    Background(lipgloss.Color("#89dceb")). // accent bg
    Padding(0, 2)

CancelBtn = lipgloss.NewStyle().
    Foreground(lipgloss.Color("252")).   // light text
    Background(lipgloss.Color("237")).   // muted gray bg
    Padding(0, 2)
```

Include hotkey hint inline: `" (enter) Install "` / `" (esc) Cancel "`

### Panel Focus

**Steal from: superfile — border color only**

- Unfocused: muted gray border
- Focused: bright accent color border
- Same border style (rounded) throughout
- Sidebar border = invisible when unfocused (border color = background color)

### Selected Item in Lists

**Steal from: gh-dash — full-width background fill**

```go
SelectedRow = lipgloss.NewStyle().
    Background(lipgloss.Color("236")).  // dark gray
    Bold(true)
```

No prefix marker needed. Background alone conveys selection.

### Modals

**Steal from: superfile — simple centered box**

- Rounded border, accent border color
- `lipgloss.Place(w, h, Center, Center, content)` for positioning
- Optional `WithWhitespaceBackground` for dimmed scrim (or skip it like superfile does)
- Buttons at bottom: `JoinHorizontal(Center, confirm, cancel)`

### Tab Bar / Section Navigation

**Steal from: gh-dash — bold + background fill for active tab**

```go
ActiveSection = lipgloss.NewStyle().
    Bold(true).
    Background(lipgloss.Color("236")).
    Padding(0, 2)

InactiveSection = lipgloss.NewStyle().
    Faint(true).
    Padding(0, 2)
```

Thick bottom border on the tab row to separate from content.

### Help Bar

**Steal from: lazygit — context-sensitive keybinding hints**

Bottom row shows available keys for the current context: `tab section  / search  enter select  ? help`

---

## Sources

### Projects Analyzed (Visual Patterns)
- [superfile](https://github.com/yorukot/superfile) — border color focus, prefix icon cursor, bg-fill modal buttons
- [gh-dash](https://github.com/dlvhdr/gh-dash) — bg-fill tabs, bg-fill row selection, side panel layout switch
- [lazygit](https://github.com/jesseduffield/lazygit) — centered floating menus, keybinding-hint-driven UI
- [k9s](https://github.com/derailed/k9s) — pill breadcrumbs, prompt insertion, tview modal buttons
- [huh](https://github.com/charmbracelet/huh) — Select field, Confirm buttons, theme system

### Cross-Framework References
- [Textual](https://textual.textualize.io/widget_gallery/) — half-block buttons, underline tabs, overlay dropdowns
- [Ink](https://github.com/vadimdemedes/ink) — inverse text buttons
- [Ratatui](https://ratatui.rs/examples/widgets/tabs/) — reversed-color tabs
- [fzf](https://github.com/junegunn/fzf) — inline fuzzy picker
- [Telescope.nvim](https://github.com/nvim-telescope/telescope.nvim) — dropdown/ivy/cursor layout modes
