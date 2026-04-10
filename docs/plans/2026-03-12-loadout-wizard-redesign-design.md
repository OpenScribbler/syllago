# Loadout Wizard Redesign - Design Document

**Goal:** Move the loadout creation wizard from a modal overlay to a full-screen content pane with split-view previews, add clickable action buttons across all TUI pages, and update docs/rules to codify the new patterns.

**Decision Date:** 2026-03-12

---

## Problem Statement

The current loadout creation wizard is a 56x24 modal overlay. This causes several problems:

1. **Text wrapping breaks layout** — the modal is too narrow, causing text to wrap past checkboxes and corrupt clickable zone areas.
2. **No preview** — users select items blind with no way to read their content before including them. This is especially problematic for security-sensitive types (MCP configs, hooks) that execute code.
3. **No mouse support** inside the modal's scrollable lists — violates the mouse parity principle.
4. **Vertical jitter** — fixed modal height with variable content causes layout instability when scrolling.
5. **Action discoverability** — keyboard shortcuts like `a` (add), `r` (remove), `s` (sync) are invisible to mouse users across all pages.

## Proposed Solution

### Part 1: Wizard as a Screen

Replace the `createLoadoutModal` with a `createLoadoutScreen` that renders in the content pane (sidebar visible). Every wizard step renders in the left pane of a persistent split-view. The right pane shows context when available (file preview on selection/review steps, empty or help text on form steps).

### Part 2: Action Buttons

Add visible, clickable action buttons with hotkey hints (`[a] + Create Loadout`) below the breadcrumb on every page that has keyboard-only actions today. Buttons use semantic background colors (green for create/add, red for remove, orange for uninstall, purple for sync, muted for utility actions).

### Part 3: Doc/Rules Updates

After implementation, update `docs/design/tui-spec.md`, `.claude/rules/tui-*.md`, and `styles.go` documentation to codify both new patterns.

## Architecture

### New Screen

- **Screen enum:** `screenCreateLoadout`
- **Model:** `createLoadoutScreen` (replaces `createLoadoutModal` in `loadout_create.go`)
- **Rendering:** Content pane with sidebar visible (peer of `screenImport`, `screenDetail`)
- **Focus:** `focusContent` — no `focusModal` involvement

### Unified Split-View Layout

Every step renders in the left pane of a persistent split-view. The right pane shows context when available.

| Step | Left pane | Right pane |
|------|-----------|------------|
| Provider picker | Cursor list of providers | Empty or "Select a provider to begin" |
| Type selection | Checkbox list with `!!` badges | Empty or brief description of selected type |
| Per-type items | Checkbox list with cursor + search | Primary file content of cursor item |
| Name/description | Text inputs | Empty or "Name your loadout" |
| Destination | Radio-style list | Empty or destination path preview |
| Review | Summary + navigable item list | Primary file content of cursor item |

### Step Flow

Same step sequence as existing wizard, just rendered differently:

1. **Provider picker** — skipped when provider is pre-filled
2. **Type selection** — checkboxes for content types. `!!` warning badge (warningStyle) on Hooks and MCP Config headers
3. **Per-type item selection** (one step per selected type) — checkbox list with search via `/`, split-view preview of cursor item's primary file
4. **Name/description** — two text inputs, Tab switches between them
5. **Destination** — radio-style list (Project, Library, optionally Registry)
6. **Review** — summary with navigable item list. Dangerous items (Hooks, MCP) marked with `!!` and cursor-navigable. Right pane shows preview of cursor item. Back/Create buttons pinned to bottom.

### Security Indicators

- **Type selection step:** Hooks and MCP Config type headers show `!!` badge in warningStyle
- **Review step:** Hooks and MCP items marked with `!!`, navigable with cursor to preview their content in the right pane. Two chances to review: during selection (preview) and during review (preview again).

### Data Flow

1. **Entry:** `a` key (or click action button) on loadout items/cards -> creates `createLoadoutScreen`, sets `a.screen = screenCreateLoadout`
2. **During:** Step transitions are internal to the model. App forwards key/mouse events.
3. **Exit (success):** Model sets `confirmed = true`, App calls `doCreateLoadout()` cmd, navigates to new loadout's detail view
4. **Exit (cancel):** Esc on first step -> App navigates back to previous screen

**State the model needs from App:**
- `providers []provider.Provider`
- `catalog *catalog.Catalog`
- `scopeRegistry string`
- `prefilledProvider string`
- `width, height int`
- `repoRoot string`

**Messages:**
- `splitViewCursorMsg` — reused from existing split-view pattern for preview loading
- `doCreateLoadoutMsg` — already exists with provider field

### Action Button Pattern

**Position:** Below breadcrumb, blank line above and below. Always visible.

**Format:** `[hotkey] Action Label` — each button is a `zone.Mark("action-{key}", ...)` clickable region with background color.

**Color mapping:**

| Action category | Background | Examples |
|----------------|------------|----------|
| Create/Add/Install | Green | `[a] + Create Loadout`, `[a] + Add Content`, `[i] Install` |
| Remove | Red | `[r] Remove` |
| Uninstall | Orange | `[u] Uninstall` |
| Sync | Purple | `[s] Sync` |
| Utility | Muted/dim gray | `[c] Copy`, `[s] Save`, `[e] Env Setup`, `[p] Share` |

**New styles in `styles.go`:**
- `actionBtnAddStyle` — green-tinted background, light text
- `actionBtnRemoveStyle` — red-tinted background, light text
- `actionBtnUninstallStyle` — orange-tinted background, light text
- `actionBtnSyncStyle` — purple-tinted background, light text
- `actionBtnDefaultStyle` — dim gray background, light text

Hotkey portion uses same background with bold text. Buttons have horizontal padding for chip look. Multiple buttons separated by space.

**Pages that get action buttons:**

| Page | Buttons |
|------|---------|
| Loadout cards | `[a] Create Loadout` |
| Loadout items list | `[a] Create Loadout` |
| Library cards | `[a] Add Content` |
| Items list (library source) | `[a] Add {Type}`, `[r] Remove` |
| Items list (other source) | `[a] Add {Type}` |
| Registries | `[a] Add Registry`, `[s] Sync`, `[r] Remove` |
| Detail view | Context-dependent: `[i] Install`, `[u] Uninstall`, `[c] Copy`, `[s] Save`, `[e] Env Setup`, `[p] Share` |
| Sandbox | Sandbox action buttons |

**Keyboard-only (no button):** `H` Toggle Hidden, `?` Help, `/` Search

### Mouse Parity Checklist

Every interactive element in the wizard with both input methods:

| Element | Keyboard | Mouse |
|---------|----------|-------|
| Provider list items | Up/Down cursor, Enter select | Click selects and advances |
| Type checkboxes | Up/Down cursor, Space toggle, `a` toggle all | Click toggles checkbox |
| Item checkboxes (per-type) | Up/Down cursor, Space toggle, `a` toggle all | Click toggles checkbox |
| Item list scrolling | Up/Down, PgUp/PgDown, Home/End | Scroll wheel |
| Search activation | `/` to activate, Esc to dismiss | Click search zone |
| Split-view pane switch | `l`/Right to preview, `h`/Left to list | Click in pane to focus |
| Preview pane scrolling | Up/Down when focused | Scroll wheel in pane |
| Text inputs (name/desc) | Tab switches fields | Click to focus field |
| Destination radio items | Up/Down cursor, Enter select | Click selects |
| Review item navigation | Up/Down through items | Click item to preview |
| Back/Create buttons | Left/Right switch, Enter activates | Click button |
| Breadcrumb segments | N/A | Click navigates back |
| Action buttons (all pages) | Keyboard shortcut | Click button |

### Old Modal Cleanup

Delete entirely:
- `createLoadoutModal` struct and all methods
- `overlayView()` method
- `focusModal` routing in `app.go` for create loadout
- Modal-zone click-away logic for create loadout
- `createLoadoutModalWidth` / `createLoadoutModalHeight` constants

### Standards Alignment

The new screen follows every convention in `.claude/rules/tui-*.md`:

| Convention | Implementation |
|------------|---------------|
| Breadcrumbs | `renderBreadcrumb()` — `Home > Loadouts > Create`, appends type name on per-type steps |
| `helpText()` | Context-sensitive per step |
| Mouse zones | `zone.Mark()` on every clickable element |
| Scroll support | `scrollOffset` + `clampScroll()` + indicators on item lists |
| Keyboard bindings | All via `keys.go`, no hardcoded strings |
| Styles | All from `styles.go` |
| Cursor pattern | `cursorPrefix()` helper |
| Toast integration | `message`/`messageIsErr` fields, promoted by App |
| Resize handling | `tea.WindowSizeMsg` updates width/height, clamps cursors |
| Golden tests | All 4 sizes (60x20, 80x30, 120x40, 160x50) |
| Search | `/` activates on item selection steps |

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Wizard location | Content pane (sidebar visible) | Consistent with other screens, room for split-view |
| Step layout | All steps in left pane of split-view | Uniform layout, no mode switching, no jank |
| Item selection | Per-type steps (not unified list) | Scales to large catalogs (1000+ rules, 500+ skills) |
| Preview content | Primary file content | Users can read actual content before including it |
| Security indicators | `!!` on type headers only | Clean items, review step handles detailed preview |
| Review preview | Split-view with navigable dangerous items | Two chances to review executable content |
| Form steps | In left pane, right pane empty/help | Consistent layout, no centered card switching |
| Action buttons | Clickable with hotkey hint + semantic bg color | Mouse parity across all pages, discoverable actions |
| Button position | Below breadcrumb with blank lines | Always visible, consistent placement |
| Button colors | Green=add, Red=remove, Orange=uninstall, Purple=sync, Gray=utility | Semantic meaning at a glance |

## Success Criteria

1. Wizard renders in content pane with sidebar visible throughout
2. Split-view on all steps — left pane content, right pane preview when applicable
3. Primary file preview loads on cursor movement in selection and review steps
4. `!!` danger badges on Hooks and MCP Config type headers
5. Review step shows navigable item list with inline preview of dangerous items
6. Every interactive element has keyboard AND mouse support
7. Action buttons appear on all pages with actions — clickable with semantic colors
8. Post-creation navigates to new loadout's detail view
9. Old modal code fully removed
10. Golden file tests at all 4 sizes for wizard steps
11. Action button goldens for every page
12. Docs/rules updated to codify new patterns

## Open Questions

None — all design decisions resolved during brainstorm.

---

## Implementation Phases

```
Phase 1: Action Button Pattern
  - New styles in styles.go (5 action button styles)
  - renderActionButtons() helper
  - Add buttons to all pages (loadout cards, library cards, items, registries, detail, sandbox)
  - Golden tests for all affected pages at all sizes

Phase 2: Wizard Screen (replace modal)
  - New screenCreateLoadout enum + createLoadoutScreen model
  - Unified split-view layout for all steps
  - Primary file preview on selection and review steps
  - Security indicators (!! badges)
  - Mouse support for all interactive elements
  - Wire into App routing (screen transitions, doCreateLoadout, navigation)
  - Delete old modal code
  - Golden tests at all sizes

Phase 3: Doc/Rules Updates
  - Update docs/design/tui-spec.md with Create Loadout screen spec
  - Update .claude/rules/tui-page-pattern.md page inventory
  - Update .claude/rules/tui-keyboard-bindings.md active key bindings
  - Add action button pattern to tui-card-grid.md or new rule file
  - Update styles.go documentation/comments
```

## Next Steps

Ready for implementation planning with `Plan` skill.
