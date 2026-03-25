# TUI Component Rules

This directory is the BubbleTea terminal UI for syllago (v3 clean-slate rewrite). All components follow strict conventions to maintain visual and behavioral consistency.

**Skill reference:** `.claude/skills/tui-builder/SKILL.md` — auto-loads when editing files in this directory. Contains the full design system, component patterns, and phase log.

## Before You Edit

1. **Read `styles.go`** — all colors (Flexoki palette) and named styles are defined there. Logo colors are separate from theme colors.
2. **Check `keys.go`** — all key bindings are defined. Use named bindings, not hardcoded strings.
3. **Run golden tests after visual changes** — `go test ./internal/tui/ -update-golden`, then review the diff.
4. **Test at multiple sizes** — verify at 60x20, 80x30, and 120x40.

## Architecture

Root model is `App` (app.go). Sub-models own their state and are composed into App. Messages flow up to App, which dispatches back down.

**File organization:**
- One model per file (topbar.go, helpbar.go, etc.)
- All styles in `styles.go` — never define colors or styles inline
- All key bindings in `keys.go`

## Navigation — Two-Tier Tabs

The topbar uses a **two-tier tab bar** inside a bordered frame (no dropdowns — those are a GUI pattern):

```
╭──syllago──────────────────────────────────────────╮
│     [1] Collections  [2] Content  [3] Config      │
├───────────────────────────────────────────────────┤
│  Library  Registries  Loadouts   [a] Add [n] Create│
╰───────────────────────────────────────────────────╯
```

- **1/2/3** switch groups, **h/l** cycle sub-tabs within a group
- **a** = Add, **n** = Create (context-sensitive to current group+tab)
- Collections is `[1]` (default), Content is `[2]`, Library is the landing page
- Group tabs are button-styled (backgrounds), sub-tabs are text-only (cyan active, faint inactive)

## Color and Styling — Flexoki

All theme colors come from the [Flexoki](https://stephango.com/flexoki) palette. Logo uses separate syllago brand colors.

| Role | Variable | Usage |
|------|----------|-------|
| Primary (cyan) | `primaryColor` | Active tabs, headings, section titles |
| Accent (purple) | `accentColor` | Focus borders, buttons, active button BG |
| Muted | `mutedColor` | Help text, inactive elements, separators |
| Success (green) | `successColor` | Installed status, success toasts |
| Danger (red) | `dangerColor` | Error messages, error borders |
| Warning (orange) | `warningColor` | Warnings, update badge |
| Logo mint | `logoMint` | `syl` in logo ONLY |
| Logo viola | `logoViola` | `lago` in logo ONLY |

**Rules:**
- Never use raw hex — define named variables in `styles.go`
- New colors MUST come from the Flexoki extended palette
- No emojis — use colored text symbols (checkmark, X, warning)

## Hotkey Labels — Brackets Standard

All keyboard shortcuts displayed in the UI use **square brackets**: `[1]`, `[a]`, `[n]`, `[esc]`. Never parentheses or other formats.

## Keyboard Handling

Key bindings are defined in `keys.go`. For the topbar, key routing is handled directly in `app.go` Update via `msg.String()` comparisons (the topbar doesn't own its own key handling — the app dispatches to it).

**Global keys (always active):**
| Key | Action |
|-----|--------|
| `ctrl+c` | Quit |
| `q` | Quit |
| `1` / `2` / `3` | Switch group |
| `h` / `l` / left / right | Cycle sub-tabs |
| `a` | Add action |
| `n` | Create action |
| `?` | Help overlay (future) |

## Mouse Handling

Every interactive element supports mouse via `lrstanley/bubblezone`:
- Zone IDs: `group-N`, `tab-G-N`, `btn-add`, `btn-create`
- Root View wraps output in `zone.Scan()`
- Click detection in topbar's `Update()` method

## Message Passing

- `tabChangedMsg{group, tab, tabLabel}` — group or sub-tab changed
- `actionPressedMsg{action, group, tab}` — action button activated
- Sub-models return `tea.Cmd` that produce typed messages
- App.Update() receives all messages and routes to handlers
- Never send messages between sibling components directly

## Layout Rules

- Topbar height is always 5 (bordered frame with 2 content rows)
- Content height = terminal height - topbar height - helpbar height (1)
- `lipgloss.Width()` for rendered strings, never `len()`
- Set `Width()` explicitly on lipgloss styles — it won't auto-wrap

## Library Table + Metadata Bar

The Library tab uses a full-width sortable table (`tableModel` in `table.go`) wrapped by `libraryModel` in `library.go`. The table shows all content items with columns: Name, Type, Scope, Files, Installed, Description (wide only).

See **Metadata Panel** section below for the current metadata implementation (replaced the original bottom-of-table metadata bar).

**Sorting:** `s` cycles columns, `S` reverses. Click column headers (zone-marked: `col-name`, `col-type`, etc.) to sort/reverse. Active column shows ▲/▼.

**Search:** `/` activates search with background-tinted input field (`inputActiveBG`/`inputInactiveBG` from styles.go). Live filters across name, display name, description, type.

**Drill-in:** `Enter` → `libraryDetail` mode with file tree + preview split. `Esc` returns to browse.

## Metadata Panel (Reusable Component)

The metadata panel (`metapanel.go`) is a shared component used across Library and Content views. It renders 3 lines of item metadata inside a unified bordered frame.

**Architecture:** `renderMetaPanel(item, data, width)` takes a `ContentItem` + pre-computed `metaPanelData` (installed status, type detail) and returns 3 padded lines. The caller wraps these in a manually-constructed border frame.

**Fields (line 1):** Name (40 fixed), Type (14), Files (9), Origin (19), Installed (greedy), Registry (greedy)
**Fields (line 2):** Scope (15 fixed), Registry (30 fixed), Path (middle-truncated, greedy)
**Fields (line 3):** Type-specific detail + [r] Rename button (right-aligned)

Type-specific details:
- Hooks: Event, Matcher, Handler (pre-computed in `computeHookDetail`)
- MCP: Server, Command (pre-computed in `computeMCPDetail`)
- Loadouts: Target, item counts (pre-computed in `computeLoadoutDetail`)
- Others: blank line

**Unified frame pattern:** Both library and explorer build frames manually:
```
╭─────────────────────────╮   Browse mode
│ metadata line 1          │
│ metadata line 2          │
│ metadata line 3  [r]     │
├─────────────────────────┤
│ table/items content      │
╰─────────────────────────╯

╭─────────────────────────╮   Detail/split mode
│ metadata line 1          │
│ metadata line 2          │
│ metadata line 3  [r]     │
├────────┬────────────────┤
│ tree   │ preview         │
╰────────┴────────────────╯
```

## Text Input Modal

`textInputModal` in `modal.go` provides centered overlay for single-field input (rename, etc.):
- Overlay via `overlayModal()` in app.go — background content visible above/below modal rows
- Background-tinted input field: dim cyan when active (`inputActiveBG`), dim grey when inactive (`inputInactiveBG`)
- Buttons use background+padding (no borders) for consistent height
- `Tab` cycles focus: input → save → cancel
- `Enter` submits, `Esc` cancels
- `modalSavedMsg` / `modalCancelledMsg` for message passing

## Navigation — q Backs Out

`q` only quits from the landing page (Collections > Library browse). Elsewhere:
- Library detail view → Library browse
- Any Content/Config tab → Collections > Library
- `Ctrl+C` always quits immediately

`R` (shift+r) re-scans catalog from disk without restarting. `rescanCatalog()` in app.go.

## MCP Scanner

The scanner in the catalog package handles provider-specific MCP directory layouts. When scanning `mcp/<provider>/`, if a directory has no `config.json` but contains subdirectories that do, it's treated as a provider grouping directory and recurses into each subdirectory.

## Testing

**Golden files** in `testdata/`:
- Naming: `{component}-{variant}-{width}x{height}.golden`
- ANSI stripped before storing (human-readable diffs)
- Test at 60x20, 80x30, 120x40

**After any visual change:**
```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/   # review every change
```

**Test helpers** in `testhelpers_test.go`:
- `testApp(t)` — empty catalog, 80x30
- `testAppSize(t, w, h)` — custom dimensions
- `keyRune(r)`, `keyPress(k)`, `pressN(m, key, n)`
- `assertContains(t, view, substr)`, `assertNotContains(t, view, substr)`
- `requireGolden(t, name, snapshot)`, `snapshotApp(t, app)`
