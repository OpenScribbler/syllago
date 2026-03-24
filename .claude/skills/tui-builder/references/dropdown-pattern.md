# Two-Tier Tab Navigation Pattern

**NOTE:** Dropdowns were abandoned in Phase 2 — they're a GUI pattern that fights the terminal. This file documents the two-tier tab pattern that replaced them.

## Design

```
╭──syllago─────────────────────────────────────────────────────────────────────╮
│               [1] Collections      [2] Content      [3] Config               │
├──────────────────────────────────────────────────────────────────────────────┤
│   Library     Registries     Loadouts              [a] Add      [n] Create   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

## Key Decisions

1. **Two tiers, not dropdowns** — group tabs (button-style, row 1) + sub-tabs (text-only, row 2)
2. **Bordered frame** with `╭──syllago──╮` inline logo and `├────┤` separator
3. **Group tabs are button-styled** (backgrounds) to differentiate from text-only sub-tabs
4. **Collections first** (`[1]`), Content second (`[2]`) — Library is the default landing page
5. **Brackets for all hotkeys** — `[1]`, `[a]`, `[n]` — never parentheses
6. **Action buttons** are context-sensitive per group, right-aligned on row 2
7. **1/2/3 switch groups**, h/l cycle sub-tabs (wraps), a/n trigger actions
8. **Mouse** supported on all elements via bubblezone

## Implementation

- `topbar.go` — `topBarModel` with `groups []tabGroup`, `activeGroup`, `activeTab`
- `tabChangedMsg` — fired when group or sub-tab changes
- `actionPressedMsg` — fired when action button activated (carries group + tab context)
- App.go handles key routing (1/2/3/h/l/a/n) and dispatches to topbar methods
- Topbar handles mouse clicks in its own `Update()`

## Why Not Dropdowns

- Dropdowns require overlay rendering (compositor complexity)
- Positioning floating menus relative to triggers is fragile in terminals
- The "open/close" state adds interaction friction
- Two-tier tabs are immediately visible and navigable — no hidden state
