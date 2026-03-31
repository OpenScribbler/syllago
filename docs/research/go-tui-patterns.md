# Go TUI Patterns Research

Deep research into how popular Go CLI applications build complex TUIs with panels, dropdowns, navigation, modals, and focus management. Conducted 2026-03-24.

## Table of Contents

- [Project Survey](#project-survey)
- [Architecture Patterns](#architecture-patterns)
- [Layout Composition](#layout-composition)
- [Focus Management](#focus-management)
- [Dropdown / Navigation Menus](#dropdown--navigation-menus)
- [Modal / Overlay Patterns](#modal--overlay-patterns)
- [Keyboard Routing](#keyboard-routing)
- [State Management](#state-management)
- [Key Libraries](#key-libraries)
- [Recommendations for Syllago](#recommendations-for-syllago)

---

## Project Survey

### Bubble Tea Projects (Elm Architecture)

| Project | Stars | What It Does | Key TUI Features |
|---------|-------|-------------|-----------------|
| [superfile](https://github.com/yorukot/superfile) | ~17k | Terminal file manager | N-panel split views, sidebar, file preview, modals, toast notifications |
| [gh-dash](https://github.com/dlvhdr/gh-dash) | ~11k | GitHub dashboard | Section tabs, split list+preview, markdown rendering, configurable layout |
| [soft-serve](https://github.com/charmbracelet/soft-serve) | ~6.7k | SSH Git server TUI | Tabs with `TabComponent` interface, two-pane layout, `Common` struct pattern |
| [glow](https://github.com/charmbracelet/glow) | - | Markdown reader | Two-state full-screen swap, shared `commonModel` context, section tabs |
| [go-musicfox](https://github.com/go-musicfox/go-musicfox) | ~2.3k | Music player | Sidebar + main list + player bar, search panel, login modal |
| [tuios](https://github.com/Gaurav-Gosain/tuios) | ~2.5k | Terminal multiplexer | 9 workspaces, BSP tiling, modal modes, 10k-line scrollback |
| [jjui](https://github.com/idursun/jjui) | ~1.7k | Jujutsu VCS TUI | Revision tree, details panel, preview, ace-jump overlay, modals |
| [pug](https://github.com/leg100/pug) | ~667 | Terraform TUI | Three-pane layout, pane focus cycling, resizable panes, nav history stack |
| [wander](https://github.com/robinovitch61/wander) | ~466 | Nomad TUI | Hierarchical drill-down stack, live log streaming, filter |
| [ktea](https://github.com/jonas-grgt/ktea) | ~417 | Kafka TUI | Tab-based via termkit/skeleton, table views, in-app config panels |

### Non-Bubble-Tea Projects (for comparison)

| Project | Stars | Library | Architecture Style |
|---------|-------|---------|--------------------|
| [lazygit](https://github.com/jesseduffield/lazygit) | ~60k | gocui (custom fork) | Imperative, context-stack, coordinate-based layout |
| [k9s](https://github.com/derailed/k9s) | ~30k | tview (custom fork) | Object-graph, stack-based page navigation, observer callbacks |

---

## Architecture Patterns

Six distinct patterns emerged across the surveyed projects, ordered from simplest to most sophisticated:

### 1. Flat Model with State Enum

**Used by:** glow, slides, smaller apps

A single model struct with a `currentView` field. `Update()` switches on the view enum to dispatch.

```go
type state int
const (
    stateList state = iota
    stateDetail
    stateSearch
)

type model struct {
    state   state
    list    list.Model
    detail  viewport.Model
    search  textinput.Model
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.state {
    case stateList:
        m.list, cmd = m.list.Update(msg)
    case stateDetail:
        m.detail, cmd = m.detail.Update(msg)
    }
    return m, cmd
}
```

**Scales to:** 2-4 views. Becomes unwieldy beyond that.

**Why it works:** Dead simple. No indirection. Easy to test. Good for apps where only one view is active at a time (full-screen swap).

### 2. Top-Down Tree (Standard Bubble Tea)

**Used by:** superfile, gh-dash, go-musicfox

Root model owns child models as struct fields. `Update()` routes to active children. `View()` composes children's rendered strings.

```go
type appModel struct {
    sidebar   sidebarModel
    content   contentModel
    modal     *modalModel  // nil when not shown
    focused   focusZone
}
```

**Scales to:** Medium complexity. Root model grows linearly with child count.

**Why it works:** Explicit ownership. Each component has clear boundaries. The root is both router and compositor.

### 3. Tab Container (Framework-Assisted)

**Used by:** ktea, gama (via termkit/skeleton)

A framework wraps Bubble Tea and provides `AddPage(id, label, model)`. Each tab is an independent `tea.Model`. The parent handles tab navigation.

```go
s := skeleton.New()
s.AddPage("topics", "Topics", topicsModel)
s.AddPage("groups", "Groups", groupsModel)
s.AddPage("schemas", "Schemas", schemasModel)
```

**Scales to:** Many tabs of similar complexity. Less flexible for non-tab layouts.

### 4. Screen Switcher with Init

**Used by:** shi.foo pattern (blog post)

A `rootScreenModel` holds `model tea.Model` (the current screen). A `SwitchScreen(model)` method replaces the current model and calls `Init()` on the new one — ensuring animations and initialization hooks run on every screen transition.

**Why this matters:** Directly assigning `m.current = newModel` skips `Init()`, which is a common source of bugs (no initial data fetch, no spinner start).

### 5. Stack Navigation

**Used by:** wander, jjui, k9s (via tview Pages)

A stack holds previously visited models. Pushing creates a new child model. Popping restores the previous. The top of stack is "current" and receives all input.

```go
type navStack struct {
    stack []tea.Model
}
func (s *navStack) Push(m tea.Model) { s.stack = append(s.stack, m) }
func (s *navStack) Pop() tea.Model   { /* pop and return previous */ }
func (s *navStack) Current() tea.Model { return s.stack[len(s.stack)-1] }
```

**Scales to:** Deep drill-down hierarchies (jobs -> allocations -> tasks).

**Key insight from lazygit:** The stack can be context-kind-aware. Pushing a `SIDE_CONTEXT` clears other side contexts. Pushing a `TEMPORARY_POPUP` overlays on top of everything.

### 6. Root-as-Router with Pane Manager (Most Sophisticated)

**Used by:** pug (the canonical example, author wrote the "Tips for building Bubble Tea programs" blog post)

The root model is a thin coordinator. A `PaneManager` owns named pane slots, handles focus, sizing, history stacks, and message routing. Child models implement a `ChildModel` interface. Models are instantiated by a `Maker` factory and cached.

```go
// ChildModel interface — note Update returns tea.Cmd, not (tea.Model, tea.Cmd)
type ChildModel interface {
    Init() tea.Cmd
    Update(tea.Msg) tea.Cmd  // mutates in place, returns only cmd
    View() string
}

// PaneManager owns three named slots
type PaneManager struct {
    panes   map[PaneID]*Pane
    focused PaneID
}

// Each Pane has a history stack and cached models
type Pane struct {
    current ChildModel
    history []Page  // Page = Kind + ID reference
    maker   Maker   // factory to instantiate models on demand
}
```

**Key patterns:**
- Root is "merely a message router and screen compositor"
- Models cached after first creation (not re-created on navigation)
- Pub/sub system for cross-model state propagation
- `Page` is a lightweight reference; `Maker` factory instantiates on demand

**Scales to:** Arbitrary complexity. pug has 30+ TUI files.

---

## Layout Composition

### lipgloss v1 (Current — what syllago uses)

All surveyed Bubble Tea apps use the same primitives:

```go
// Sidebar + content
body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

// Full layout with header/footer
full := lipgloss.JoinVertical(lipgloss.Left, topbar, body, statusbar)
```

**Critical rules every project follows:**

1. **Subtract border widths from content dimensions.** A border adds 2 to both width and height. Content width = panel width - `style.GetHorizontalFrameSize()`.

2. **Set `Width()` on styles explicitly.** Lipgloss won't auto-wrap — it'll overflow.

3. **Propagate `tea.WindowSizeMsg` to all children.** Children that never receive a size message render at zero size until the next resize.

4. **Use `lipgloss.Width()` not `len()`.** ANSI sequences add invisible bytes; Unicode wide chars count as 2 cells.

**Superfile's approach (N-panel strip):**
```go
// filemodel.Render() — join N panels + optional preview horizontally
panels := make([]string, len(m.filePanels))
for i, p := range m.filePanels {
    panels[i] = p.Render()
}
strip := lipgloss.JoinHorizontal(lipgloss.Top, panels...)
return lipgloss.JoinHorizontal(lipgloss.Top, strip, previewPanel)
```

**Soft Serve's approach (tabs + content):**
```go
// Tabs bar renders tab labels; active content pane renders below
tabBar := m.tabs.View()
content := m.panes[m.activeTab].View()
return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
```

### lipgloss v2 — Canvas/Layer Compositor (New)

Merged May 2025. Provides proper z-index layering:

```go
bg := lipgloss.NewLayer(mainContent)
fg := lipgloss.NewLayer(modalContent).X(10).Y(5).Z(1)
canvas := lipgloss.NewCanvas(bg, fg)
output := canvas.Render()
```

Supports `canvas.Hit(x, y)` for mouse event routing by layer. This replaces the string-splicing overlay hacks needed in v1.

### Coordinate-Based Layout (lazygit via gocui)

For comparison: lazygit computes absolute `(x0, y0, x1, y1)` coordinates for every view. No flex layout or constraint solver — everything recomputed from scratch on each render cycle.

```go
// Pseudocode for lazygit's layout
dims := gui.getWindowDimensions(termWidth, termHeight)
for name, d := range dims {
    gui.g.SetView(name, d.X0, d.Y0, d.X1, d.Y1)
}
```

---

## Focus Management

### The Universal Pattern

Every surveyed project uses the same core approach:

```go
type focusZone int
const (
    focusSidebar focusZone = iota
    focusContent
    focusSearch
)

type model struct {
    focused focusZone
    // ... child models
}
```

On Tab: increment `focused` (mod N). On Shift+Tab: decrement. Route key events only to the focused component.

### Focus/Blur on bubbles Components

Components like `textinput` and `textarea` have explicit `.Focus()` and `.Blur()` methods that control cursor visibility and input acceptance:

```go
m.search.Focus()   // shows cursor, accepts input
m.sidebar.Blur()   // dims or deactivates
```

### Mouse-Based Focus

**Simple approach (fixed layout):**
```go
case tea.MouseMsg:
    if msg.X < sidebarWidth {
        m.focused = focusSidebar
    } else {
        m.focused = focusContent
    }
```

**Robust approach (bubblezone library):**
```go
// In child View():
return zone.Mark("sidebar", sidebarContent)

// In root View():
return zone.Scan(lipgloss.JoinHorizontal(lipgloss.Top, ...))

// In root Update():
if zone.Get("sidebar").InBounds(msg) {
    m.focused = focusSidebar
}
```

Zone markers are zero-width ANSI sequences that don't affect `lipgloss.Width()` calculations.

### Soft Serve's TabComponent Pattern

Tabs always receive all messages (so tab switching always works). The active content pane only gets messages after tabs have processed:

```go
func (s *Selection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 1. Tabs always get the message first
    s.tabs, cmd = s.tabs.Update(msg)
    cmds = append(cmds, cmd)

    // 2. Handle tab-change events
    switch msg := msg.(type) {
    case tabs.ActiveTabMsg:
        s.activePane = pane(msg)
    }

    // 3. Only active pane gets remaining messages
    switch s.activePane {
    case selectorPane:
        s.selector, cmd = s.selector.Update(msg)
    case readmePane:
        s.readme, cmd = s.readme.Update(msg)
    }
    return s, tea.Batch(cmds...)
}
```

### Lazygit's Context Stack

Focus is a LIFO stack with context-kind-aware push semantics:

| Push | Effect |
|------|--------|
| `SIDE_CONTEXT` | Clear other side contexts, push |
| `MAIN_CONTEXT` | Clear other main, keep side |
| `TEMPORARY_POPUP` | Push on top (modal overlay) |
| `PERSISTENT_POPUP` | Push on top, Escape returns to it |

This is more sophisticated than a simple `focused` field — it handles the case where a modal is showing on top of a panel that's showing on top of another panel.

---

## Dropdown / Navigation Menus

**Key finding: there is no first-party dropdown component in the Bubble Tea ecosystem.** This is a confirmed gap (bubbletea issue #309).

### Approach A: Full-Panel List Replacement (Most Common)

When the user activates the "dropdown," swap the content area for a `bubbles/list` component. On selection, swap back. No compositing needed.

**Used by:** Most Charm-native apps (glow's section tabs, gh-dash's section navigation).

### Approach B: huh Select Field

`charmbracelet/huh` provides a `Select[T]` component that is the closest thing to a production dropdown:

- Renders options in a `viewport.Model` (viewport handles scroll)
- Press `/` to enter filter mode (fuzzy search via `textinput.Model`)
- Inline mode (`s.inline = true`): renders as `< option >` on a single line
- Viewport scroll follows selected item via `cursorOffset`/`cursorHeight`

```go
form := huh.NewForm(
    huh.NewGroup(
        huh.NewSelect[string]().
            Title("Choose provider").
            Options(huh.NewOption("Claude", "claude"), ...),
    ),
).WithSubmitCmd(func() tea.Msg { return formDoneMsg{} })
```

**Key insight:** huh's Select is viewport-backed. Set content, let viewport handle scroll — cleaner than manually tracking `renderIndex`.

### Approach C: Custom Overlay Dropdown

Build a custom model with `isOpen bool` and `cursor int`. When open, render the options list using an overlay compositor positioned relative to the trigger element.

**Requires:** Knowing the trigger's absolute screen position (bubblezone can provide this).

### Approach D: Soft Serve's Tab Message Pattern

For syllago's dropdown nav specifically, Soft Serve's approach is the cleanest reference:

```go
// Tab component fires ActiveTabMsg on selection
type SelectTabMsg int
type ActiveTabMsg int

// Parent catches ActiveTabMsg to update active pane
case tabs.ActiveTabMsg:
    r.activeTab = int(msg)
```

The tabs component uses `bubblezone` to mark each tab label with a zone ID for mouse support. Keyboard: `tab`/`shift+tab` cycles. This pattern works for a dropdown too — the list `isOpen` state is a bool on the component model, and `ActiveTabMsg` equivalent fires on selection.

---

## Modal / Overlay Patterns

### Approach 1: lipgloss.Place (v1, simple)

Used by superfile and most v1 apps:

```go
func (m model) View() string {
    main := m.renderMainLayout()

    if m.modal != nil {
        modalContent := m.modal.View()
        return lipgloss.Place(
            m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            modalContent,
            lipgloss.WithWhitespaceBackground(dimBGColor),
        )
    }
    return main
}
```

The modal replaces the entire frame. `WithWhitespaceBackground` fills surrounding space to create a dimmed effect. Simple but effective — no actual layering needed because the terminal redraws everything each frame.

### Approach 2: String-Splicing Compositor (v1, complex)

For overlays that don't replace the entire frame (e.g., a dropdown floating over content):

1. Split both background and foreground into lines
2. For each background line in the overlay's Y range, splice in foreground at correct X offset
3. Must walk strings character-by-character to handle ANSI sequences correctly

**Libraries:** `rmhubbert/bubbletea-overlay`, `jsdoublel/bubbletea-overlay`

```go
overlayModel := overlay.New(
    fgModel,           // the modal
    bgModel,           // background
    overlay.Center,    // horizontal
    overlay.Center,    // vertical
    0, 0,              // offset
)
```

**Important:** Don't add margins to the foreground model — use the overlay's positioning parameters.

### Approach 3: Canvas Compositor (v2, native)

```go
bg := lipgloss.NewLayer(mainContent)
fg := lipgloss.NewLayer(modalContent).X(x).Y(y).Z(10)
canvas := lipgloss.NewCanvas(bg, fg)
output := canvas.Render()
```

Proper z-index compositing. Supports hit-testing for mouse routing by layer.

### Modal State Management

Superfile's pattern (representative of most apps):

```go
type model struct {
    // Modal state — simple bool + struct
    typingModal   typingModalState   // owned on root model
    sortModal     sortModel          // sub-package model
    confirmModal  confirmModalState

    // When any modal is open, divert ALL input to it
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if m.typingModal.open {
        return m.handleTypingModal(msg)
    }
    if m.sortModal.IsOpen() {
        return m.handleSortModal(msg)
    }
    // ... normal panel routing
}
```

---

## Keyboard Routing

### Bubble Tea Pattern (Message Dispatch)

Route in `Update()` based on focused component:

```go
case tea.KeyMsg:
    // 1. Global keys first (quit, help)
    switch msg.String() {
    case "ctrl+c": return m, tea.Quit
    case "?":      return m.showHelp()
    }

    // 2. Modal intercepts all remaining keys
    if m.modal != nil {
        m.modal, cmd = m.modal.Update(msg)
        return m, cmd
    }

    // 3. Route to focused panel
    switch m.focused {
    case focusSidebar:
        m.sidebar, cmd = m.sidebar.Update(msg)
    case focusContent:
        m.content, cmd = m.content.Update(msg)
    }
```

### Lazygit Pattern (Registration Table)

gocui registers keybindings per-view at startup:

```go
// Registration
gui.g.SetKeybinding("branches", 'd', gocui.ModNone, deleteBranch)
gui.g.SetKeybinding("",         'q', gocui.ModNone, quit)  // "" = global

// gocui dispatches automatically to the handler for the focused view
```

### k9s Pattern (Per-View InputCapture)

Each view calls `SetInputCapture(v.keyboard)` on its tview primitive. tview fires the capture handler only for the currently focused primitive — key isolation is automatic.

### Key Insight: Priority Order

Every project follows the same priority:
1. **Global keys** (quit, help) — always handled
2. **Modal keys** (confirm, cancel) — when modal is active, consume everything
3. **Panel keys** (scroll, select) — only for focused panel

---

## State Management

### Shared Context Struct (Soft Serve's `Common`)

Every component embeds `common.Common`:

```go
type Common struct {
    Width, Height int          // current dimensions
    Styles        *styles.Styles // shared stylesheet
    KeyMap        *keymap.KeyMap // shared keybindings
    Zone          *zone.Manager  // mouse hit-testing
    Logger        *log.Logger    // structured logger
}
```

No component ever needs to be told its size via message — parent calls `SetSize(w, h)` directly. This avoids threading dimensions through the message bus.

### Model Data vs Navigation Context (Syllago-Relevant)

Syllago's `itemsContext` pattern (preserving breadcrumbs, filters across refreshes) maps to a common challenge. The clean solution used by multiple projects:

```go
// Separate data state from navigation state
type itemsModel struct {
    // Data — changes on refresh
    items     []Item
    providers []Provider

    // Navigation — survives refreshes
    ctx itemsContext
}

// Rebuild preserves navigation context
func (a *app) rebuildItems() {
    savedCtx := a.items.ctx
    a.items = newItemsModel(freshData...)
    a.items.ctx = savedCtx
}
```

This is exactly what syllago already does with `rebuildItems()` — it's the right pattern.

### Lazygit's Per-Repo State Map

```go
type Gui struct {
    RepoStateMap map[Repo]*GuiRepoState
}
```

When switching repos/worktrees, the current state is stored and the new repo's state is loaded. This restores exact cursor positions and panel state. Relevant if syllago ever needs to switch between multiple catalog roots.

---

## Key Libraries

| Library | Stars | Purpose | Used By |
|---------|-------|---------|---------|
| [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | - | Official components (list, viewport, textinput, table, etc.) | Everyone |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | - | Styling and layout composition | Everyone |
| [charmbracelet/huh](https://github.com/charmbracelet/huh) | - | Forms, Select/MultiSelect fields (closest to dropdown) | Embedded in larger apps |
| [lrstanley/bubblezone](https://github.com/lrstanley/bubblezone) | ~845 | Mouse region tracking and hit-testing | soft-serve, others |
| [termkit/skeleton](https://github.com/termkit/skeleton) | ~62 | Multi-tab framework with parent-child refs | ktea, gama |
| [evertras/bubble-table](https://github.com/evertras/bubble-table) | - | Feature-rich table component | Multiple projects |
| [rmhubbert/bubbletea-overlay](https://pkg.go.dev/github.com/rmhubbert/bubbletea-overlay) | - | Foreground/background compositor for modals | jjui, others |

---

## Recommendations for Syllago

Based on the research, here's what maps to syllago's TUI needs:

### Architecture

Syllago's current approach (top-down tree with root model owning child models) is the right pattern for its complexity level. It matches Pattern 2 (superfile, gh-dash). If complexity grows significantly, Pattern 6 (pug's PaneManager) is the upgrade path — but that's premature now.

### Dropdown Navigation

Since no first-party dropdown exists, the recommended approach is:

1. **Soft Serve's message pattern** (`SelectTabMsg`/`ActiveTabMsg`) for the tab/dropdown component API
2. **`isOpen bool` + `cursor int`** on the dropdown model
3. **Render the open dropdown over the topbar area** using `lipgloss.Place` (simple overlay, not full compositor)
4. **When open, intercept all key events** (same as modal pattern — dropdown is effectively a temporary modal)
5. **bubblezone markers** on each option for mouse support

### Multi-Panel Layout

Current lipgloss v1 approach is correct:
```go
body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
full := lipgloss.JoinVertical(lipgloss.Left, topbar, body, helpbar)
```

Key improvement from superfile: **parent calculates column widths in `WindowSizeMsg` handler and calls `SetSize(w, h)` on each child** — don't let children calculate their own size.

### Modals

`lipgloss.Place(fullW, fullH, Center, Center, content)` is the standard v1 approach. Works for syllago's install/confirm modals. No need for the string-splicing compositor unless modals need to float at non-center positions.

### Focus

Syllago's existing `focusZone` enum pattern is standard. No changes needed.

### Future: Bubble Tea v2

The v2 migration (`charm.land/bubbletea/v2`) brings:
- Native Canvas/Layer compositor (replaces overlay hacks)
- Granular mouse events (`MouseClickMsg`, `MouseWheelMsg`)
- Kitty Keyboard Protocol (`ctrl+m` distinct from Enter, `shift+enter`)
- `View()` returns `tea.View` struct instead of `string`

This is a bounded refactor worth planning for but not urgent. v2.0.0 final was imminent as of March 2026.

---

## Sources

### Projects
- superfile: https://github.com/yorukot/superfile
- gh-dash: https://github.com/dlvhdr/gh-dash
- soft-serve: https://github.com/charmbracelet/soft-serve
- glow: https://github.com/charmbracelet/glow
- pug: https://github.com/leg100/pug
- lazygit: https://github.com/jesseduffield/lazygit
- k9s: https://github.com/derailed/k9s
- jjui: https://github.com/idursun/jjui
- wander: https://github.com/robinovitch61/wander
- ktea: https://github.com/jonas-grgt/ktea
- tuios: https://github.com/Gaurav-Gosain/tuios

### Articles & Talks
- Tips for building Bubble Tea programs (leg100): https://leg100.github.io/en/posts/building-bubbletea-programs/
- Multi-view interfaces in Bubble Tea (shi.foo): https://shi.foo/weblog/multi-view-interfaces-in-bubble-tea
- Overlay composition using Bubble Tea (Leon Mika): https://lmika.org/2022/09/24/overlay-composition-using.html
- Bubbletea state machine pattern (Zack Proser): https://zackproser.com/blog/bubbletea-state-machine
- Managing nested models (donderom): https://donderom.com/posts/managing-nested-models-with-bubble-tea/
- Lazygit turns 5 (Jesse Duffield): https://jesseduffield.com/Lazygit-5-Years-On/
- Bubble Tea v2 discussion: https://github.com/charmbracelet/bubbletea/discussions/1374

### Libraries
- bubbles: https://github.com/charmbracelet/bubbles
- lipgloss: https://github.com/charmbracelet/lipgloss
- huh: https://github.com/charmbracelet/huh
- bubblezone: https://github.com/lrstanley/bubblezone
- bubbletea-overlay: https://github.com/rmhubbert/bubbletea-overlay
- termkit/skeleton: https://github.com/termkit/skeleton
- charm-in-the-wild: https://github.com/charm-and-friends/charm-in-the-wild
- awesome-tuis: https://github.com/rothgar/awesome-tuis
