# TUI Redesign: Sidebar Layout + Mouse Support + Modals + Nesco Palette

**Date:** 2026-02-17
**Status:** Design
**Feature:** tui-redesign

---

## 1. Problem & Solution

### Problem

The current Nesco TUI uses full-screen replacement navigation: selecting a category replaces the entire screen with the items list, selecting an item replaces it with the detail view, and pressing Esc goes back. This creates several issues:

- **Context loss** — navigating to a tool's detail view hides the category list and items list entirely, forcing users to mentally track where they are
- **Wasted screen real estate** — the category list (6-8 short items) takes the full terminal width, leaving most of the screen empty
- **Janky back-and-forth** — browsing multiple tools requires repeated Enter/Esc cycling through screens
- **No mouse support** — everything is keyboard-only, no click interaction
- **No confirmation dialogs** — destructive actions (install, uninstall) happen immediately without confirmation
- **Poor contrast** — 3 colors fail WCAG AA on light terminals (secondary cyan 2.8:1, success green 2.9:1, warning amber 2.3:1)

### Solution

Redesign the TUI with three changes:

1. **Sidebar + Content layout** (VS Code style) — persistent category sidebar on the left (~16 chars wide), content area on the right swaps between items list and detail view
2. **Modal overlay system** — centered confirmation dialogs for destructive/important actions using bubbletea-overlay
3. **Mouse support** — click-to-select via bubblezone for sidebar items, list items, action buttons, and tabs
4. **Nesco color palette** — brand-aligned mint green + lavender purple with all colors passing WCAG AA

---

## 2. Visual Layout

### Items List View

When browsing a category, the sidebar stays fixed and the content area shows the items table:

```
┌─ Nav ─────────┬─ Items: Skills (12) ────────────────────────────────────────────────────┐
│               │                                                                          │
│ ▸ Skills   12 │  Name               Description                          Provider       │
│   Agents    4 │  ─────────────────────────────────────────────────────────────────────── │
│   Prompts   8 │  cursor-rules       Cursor rules for AI coding           Claude         │
│   MCP       6 │ ▸ develop           Unified feature development          All provs      │
│   Apps      2 │   execute           Execute plans with subagent dispatch  Claude         │
│               │   brainstorm        Turn ideas into designs               Claude         │
│ ───────────── │   research          Two-phase web research workflow       Claude         │
│ ◆ My Tools  3 │   plan              Create detailed implementation plans  Claude         │
│ ◇ Import      │   inbox             Quick capture to Obsidian inbox       Claude         │
│ ◇ Update      │   code-review       Code review a pull request            Claude         │
│ ◇ Settings    │   tech-writing      Apply frameworks to documentation     Claude         │
│               │   demo-builder      Build end-to-end Aembit demos         Claude         │
│               │   art               Visual content with Excalidraw        Claude         │
│               │   agents            Dynamic agent composition             Claude         │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
├───────────────┴──────────────────────────────────────────────────────────────────────────┤
│  /search   Enter: view detail   ?: help   q: quit                        12 items       │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

**Sidebar layout:**
- Content type categories with item counts (▸ = selected)
- Horizontal separator
- Utility items: My Tools (◆ = has items, ◇ = empty), Import, Update, Settings
- Sidebar width: fixed ~16 characters

### Detail View (Overview Tab)

Selecting an item replaces the items list with the detail view. The detail view has **metadata above a horizontal separator** and **description/content below**:

```
┌─ Nav ─────────┬─ develop ── Overview  Files  Install ───────────────────────────────────┐
│               │                                                                          │
│   Skills   12 │  Type: Skill            Path: skills/develop/                            │
│   Agents    4 │  Providers: All                                                          │
│   Prompts   8 │                                                                          │
│   MCP       6 │  ─────────────────────────────────────────────────────────────────────── │
│   Apps      2 │                                                                          │
│               │  Unified feature development workflow. USE WHEN developing features      │
│ ───────────── │  OR brainstorm to execute OR full development cycle OR create             │
│   My Tools  3 │  implementation from idea.                                               │
│   Import      │                                                                          │
│   Update      │  Orchestrates brainstorm → plan → validate → beads → execute.            │
│   Settings    │                                                                          │
│               │  ## How it Works                                                         │
│               │                                                                          │
│               │  1. Brainstorm phase - collaborative dialogue to shape the idea           │
│               │  2. Plan phase - detailed implementation plan with bite-sized tasks       │
│               │  3. Validate phase - review plan for completeness                         │
│               │  4. Beads phase - create tracked issues for each task                     │
│               │  5. Execute phase - dispatch subagents to implement                       │
│               │                                                                          │
│               │  Installed: Claude ✓  Cursor ✗  Windsurf ✗                               │
│               │                                                                          │
│               │  [i]nstall  [u]ninstall  [c]opy  [s]ave                                  │
├───────────────┴──────────────────────────────────────────────────────────────────────────┤
│  Esc: back to list   Tab/1-3: switch tab   ?: help                Skills > develop       │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

**Detail view structure:**
- **Header bar:** item name + tab bar (`Overview  Files  Install`)
- **Metadata section** (above separator): Type, Path, Providers — compact, always visible
- **Horizontal separator line** — visual divider
- **Content section** (below separator): description, README content, scrollable
- **Install status + action keys** near bottom of content area
- **Footer:** context-sensitive help keys + breadcrumb (`Skills > develop`)

**Key layout principle:** Metadata is always visible at the top. The scrollable content area is below the separator only — metadata stays pinned.

### Modal Overlay

Confirmation dialogs appear as centered overlays on top of the current view:

```
┌─ Nav ─────────┬─ develop ── Overview  Files  Install ───────────────────────────────────┐
│               │                                                                          │
│   Skills   12 │  Type: Skill       ┌──────────────────────────────┐                      │
│   Agents    4 │  Providers: All    │                              │                      │
│   Prompts   8 │                    │   Install "develop"?         │                      │
│   MCP       6 │  ──────────────    │                              │                      │
│   Apps      2 │                    │   Providers:                 │                      │
│               │  Unified feature   │   [x] Claude                 │                      │
│ ───────────── │  OR brainstorm to  │   [ ] Cursor                 │                      │
│   My Tools  3 │  implementation    │   [ ] Windsurf               │                      │
│   Import      │                    │                              │                      │
│   Update      │  Orchestrates bra  │   [Enter] Install  [Esc] Cancel                    │
│   Settings    │                    │                              │                      │
│               │  ## How it Works   └──────────────────────────────┘                      │
│               │                                                                          │
│               │  1. Brainstorm phase - collaborative dialogue                            │
│               │  2. Plan phase - detailed implementation plan                             │
│               │  3. Validate phase - review plan for completeness                         │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
│               │                                                                          │
├───────────────┴──────────────────────────────────────────────────────────────────────────┤
│  Enter: confirm   Esc: cancel                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

**Modal behavior:**
- Centered on the content area (not the full terminal)
- Background content is dimmed/visible but not interactive
- All input routes to the modal until dismissed
- Border uses accent color (viola purple)

---

## 3. Architecture (Internal)

### Layout Structure

```
┌─────────────────────────────────────────────────────┐
│  App Model (root)                                   │
│  ┌──────────┬──────────────────────────────────────┐│
│  │ Sidebar  │  Content Area                        ││
│  │ Model    │  ┌──────────────────────────────────┐││
│  │          │  │ Items Model  OR  Detail Model    │││
│  │ category │  │                                  │││
│  │ list     │  │ (swapped based on navigation)    │││
│  │          │  │                                  │││
│  │          │  └──────────────────────────────────┘││
│  └──────────┴──────────────────────────────────────┘│
│  ┌──────────────────────────────────────────────────┐│
│  │ Footer: breadcrumb + help keys                   ││
│  └──────────────────────────────────────────────────┘│
│                                                     │
│  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐                   │
│  │  Modal Overlay (when active) │  ← bubbletea-overlay
│  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘                   │
│                                                     │
│  Mouse zones managed by bubblezone                  │
└─────────────────────────────────────────────────────┘
```

### Component Hierarchy

```
AppModel (root)
├── SidebarModel          — category list, always visible
│   └── focused: bool     — receives input when focused
├── ContentModel          — routes to active sub-view
│   ├── ItemsModel        — tool list for selected category
│   ├── DetailModel       — tool detail + actions
│   │   ├── DetailEnvModel
│   │   ├── DetailFileViewerModel
│   │   └── DetailProvCheckModel
│   ├── ImportModel       — import flow
│   ├── SettingsModel     — settings screen
│   └── UpdateModel       — update screen
├── ModalModel            — overlay manager (bubbletea-overlay)
│   ├── ConfirmModal      — install/uninstall/promote
│   ├── SavePromptModal   — save prompt dialog
│   ├── EnvSetupModal     — multi-step env wizard
│   └── AppScriptModal    — app script confirmation
├── SearchModel           — search overlay (stays as-is)
├── HelpOverlayModel      — help overlay (stays as-is)
└── FooterModel           — breadcrumb + context-sensitive help keys
```

### Focus Management

```
focusTarget enum: focusSidebar | focusContent | focusModal

Update routing:
1. If modal is active → all input to ModalModel
2. Else if search is active → all input to SearchModel
3. Else → route to focused panel (sidebar or content)

Tab / Shift+Tab → toggle focus between sidebar and content
Mouse click in zone → set focus to that panel
```

### Sidebar ↔ Content Interaction

```
Sidebar selects category → sends CategoryChangedMsg
Content receives msg → loads items for that category
Content selects item → sends ItemSelectedMsg
Content receives msg → switches to detail view
Esc in detail → back to items list
Esc in items → focus moves to sidebar
```

### Panel Rendering (lipgloss composition)

```go
// Pseudo-code for the View method
sidebar := m.sidebar.View()  // fixed width ~16 chars
content := m.content.View()  // fills remaining width

// Compose panels
panels := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

// Add footer
body := lipgloss.JoinVertical(lipgloss.Left, panels, footer)

// Wrap with bubblezone for mouse tracking
return zone.Scan(body)

// If modal active, bubbletea-overlay renders it centered on top
```

---

## 4. Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Layout | Sidebar + Content (Design 2) | Eliminates back-and-forth navigation, keeps category context visible, validated by research |
| Sidebar width | Fixed ~16 chars | Category names are short, no need for dynamic sizing |
| Content switching | Swap models in content area | Items list and detail view replace each other (not stacked) |
| File navigation | Single-pane | Enter views file content, Esc goes back. Simple, matches current behavior |
| Confirmations | Centered modal overlays | Industry standard for destructive actions, clear visual hierarchy |
| Modal library | bubbletea-overlay v0.6.5 | Purpose-built for Bubble Tea, lipgloss-compatible, handles positioning |
| Mouse support | bubblezone | Zone-based click detection, wraps existing rendering, minimal code changes |
| Color scheme | Nesco palette (mint + viola) | Brand-aligned, all colors pass WCAG AA on both light and dark terminals |
| Env setup flow | Multi-step modal wizard | Groups related steps, doesn't leave the detail view context |
| Search/Help | Keep current overlay approach | Already works well, no need to change |

---

## 5. User Flows

### Flow 1: Browse and Install

```
1. App opens → sidebar focused, first category selected
2. → or Enter → focus moves to content (items list)
3. j/k or ↑↓ → navigate items
4. Enter → detail view replaces items list in content area
5. i → Install confirmation modal appears (centered)
6. y/Enter → install proceeds, modal shows progress
7. Modal closes → back to detail view with "Installed ✓" status
8. Esc → back to items list
```

### Flow 2: Save Prompt

```
1. In detail view → s to save prompt
2. Save prompt modal appears with text input
3. User enters filename → Enter to confirm
4. Modal closes → success message inline in detail view
```

### Flow 3: Uninstall

```
1. In detail view → x to uninstall
2. Confirmation modal: "Uninstall {tool}? This will remove..."
3. y/Enter → uninstall proceeds
4. n/Esc → cancel, modal closes
```

### Flow 4: Environment Setup

```
1. In detail view → e to set up environment
2. Multi-step modal wizard appears
3. Step 1: Select environment type
4. Step 2: Configure paths/settings
5. Step 3: Confirm and apply
6. Modal closes → detail view shows updated env status
```

### Flow 5: File Browsing

```
1. In detail view → switch to Files tab
2. Content area shows file/directory listing
3. j/k to navigate, Enter to open file
4. File content replaces file list in content area
5. Esc → back to file list
6. Esc again → back to detail tabs
```

### Flow 6: Mouse Interaction

```
1. Click category in sidebar → selects category, loads items
2. Click item in list → opens detail view
3. Click action button in detail → triggers that action (modal if needed)
4. Click tab in detail → switches tab
5. Scroll → scrolls active panel content
```

### Action → Modal/Inline Mapping

| Action | Type | Notes |
|--------|------|-------|
| Install tool | Modal | Confirmation required, shows progress |
| Uninstall tool | Modal | Destructive action, confirmation required |
| Save prompt | Modal | Text input for filename |
| Promote tool | Modal | Confirmation required |
| Run app script | Modal | Confirmation + output display |
| Copy prompt | Inline | Instant action, brief success message |
| Search | Overlay | Existing behavior, keep as-is |
| Help | Overlay | Existing behavior, keep as-is |
| Environment setup | Modal wizard | Multi-step flow |

---

## 6. Color Palette (Nesco)

### Semantic Colors

All colors use `lipgloss.AdaptiveColor{Light, Dark}` where:
- `Light` = color used on **light** terminal backgrounds
- `Dark` = color used on **dark** terminal backgrounds

```
PRIMARY (Mint)    : Light #047857  Dark #6EE7B7
ACCENT  (Viola)   : Light #6D28D9  Dark #C4B5FD
MUTED   (Stone)   : Light #57534E  Dark #A8A29E
SUCCESS (Green)   : Light #15803D  Dark #4ADE80
DANGER  (Red)     : Light #B91C1C  Dark #FCA5A5
WARNING (Amber)   : Light #B45309  Dark #FCD34D
```

### Panel/Layout Colors

```
BORDER            : Light #D4D4D8  Dark #3F3F46
SELECTED BG       : Light #D1FAE5  Dark #1A3A2A
MODAL OVERLAY BG  : Light #F4F4F5  Dark #27272A
MODAL BORDER      : Light #6D28D9  Dark #C4B5FD
```

### Contrast Ratios

All colors pass WCAG AA (4.5:1 minimum for normal text):

| Color | Dark terminal | Light terminal |
|-------|--------------|----------------|
| Primary (Mint) | 11.3:1 | 5.9:1 |
| Accent (Viola) | 8.2:1 | 6.7:1 |
| Muted (Stone) | 5.2:1 | 5.4:1 |
| Success (Green) | 10.5:1 | 5.1:1 |
| Danger (Red) | 7.4:1 | 5.5:1 |
| Warning (Amber) | 12.8:1 | 5.2:1 |
| Selected (Mint on BG) | 8.4:1 | 5.3:1 |

---

## 7. New Dependencies

| Package | Version | Purpose | Stars |
|---------|---------|---------|-------|
| `github.com/lrstanley/bubblezone` | latest | Mouse click region tracking | 818 |
| `github.com/erikgeiser/bubbletea-overlay` | v0.6.5 | Modal/overlay dialog windows | 112 |

### Existing Dependencies (no changes)

- `github.com/charmbracelet/bubbletea` v1.3.10
- `github.com/charmbracelet/bubbles` v1.0.0
- `github.com/charmbracelet/lipgloss` v1.1.1
- `github.com/charmbracelet/glamour` v0.10.0
- `github.com/muesli/termenv` v0.16.0

---

## 8. Success Criteria

1. **Sidebar always visible** — category list never disappears during navigation
2. **No full-screen replacements** — content swaps happen within the content panel only
3. **All confirmations use modals** — install, uninstall, save, promote, app script
4. **Mouse clickable** — sidebar items, list items, action buttons, tabs respond to clicks
5. **WCAG AA contrast** — all text passes 4.5:1 ratio on both light and dark terminals
6. **No regression** — all existing keyboard shortcuts continue to work
7. **Responsive** — layout adapts to terminal width (sidebar collapses below ~60 cols)
8. **Brand-aligned** — Nesco mint green + viola purple palette throughout

---

## 9. Scope Boundaries

### In Scope

- Sidebar + content panel layout
- Modal overlay system for confirmations
- Mouse click support (bubblezone)
- Nesco color palette
- Breadcrumb footer
- Focus management (Tab to switch panels)

### Out of Scope

- Split-pane file viewer (using single-pane)
- Drag-and-drop
- Resizable panels
- Custom themes / theme switching
- Undo/redo for actions
- Animation / transitions
