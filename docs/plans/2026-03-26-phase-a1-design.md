# Phase A.1 Design Specification: Action Buttons, Remove Flow, Modal UX

**Date:** 2026-03-26
**Status:** Design specification
**Parent:** Phase A (Foundation)
**Epic bead:** syllago-wxtkw

## Overview

Phase A delivered keyboard shortcuts for Remove (`d`) and Uninstall (`x`), but user
testing revealed three UX gaps:

1. **No visible action buttons** — actions only discoverable via keyboard shortcuts
2. **Remove flow is confusing** — locked checkboxes, unclear what's being deleted, no
   way to selectively uninstall before removing
3. **Modal navigation** — left/right doesn't work between buttons, title wraps past border

This spec covers all three fixes.

---

## 1. Action Buttons on Sub-Tab Row

### Current Layout

```
╭──syllago────────────────────────────────────[?]──╮
│          [1] Collections  [2] Content  [3] Config │  ← Row 1: groups
├──────────────────────────────────────────────────┤
│  Library  Registries  Loadouts          [a] Add   │  ← Row 2: tabs + buttons
│                                                    │  ← Row 3: breadcrumbs (usually blank)
╰──────────────────────────────────────────────────╯
```

### New Layout

Move action buttons from Row 2 to Row 3 (the breadcrumb/action row). Buttons are
right-aligned, breadcrumbs are left-aligned. When breadcrumbs are active, they share
the row with buttons.

```
╭──syllago────────────────────────────────────[?]──╮
│          [1] Collections  [2] Content  [3] Config │  ← Row 1: groups
├──────────────────────────────────────────────────┤
│  Library  Registries  Loadouts                    │  ← Row 2: tabs only
│                          [a] Add  [d] Remove  [x] Uninstall │  ← Row 3: buttons
╰──────────────────────────────────────────────────╯
```

With breadcrumbs active (gallery drill-in):
```
│  team-starter > hooks                [d] Remove   │  ← Row 3: crumbs + buttons
```

### Context-Sensitive Buttons

Buttons change per tab, matching the design doc's action map (only showing
actions that are implemented):

| Tab | Buttons |
|-----|---------|
| Library | `[a] Add` `[d] Remove` `[x] Uninstall` |
| Registries | `[a] Add` `[d] Remove` |
| Loadouts | `[d] Remove` |
| Content tabs (Skills, Agents, etc.) | `[a] Add` `[d] Remove` `[x] Uninstall` |
| Config tabs | *(none)* |

Future phases will add `[i] Install`, `[s] Share` to the relevant tabs.

### Button Rendering

Same `activeButtonStyle` as current `[a] Add`. Each button gets a zone ID for
mouse clicks: `btn-add`, `btn-remove`, `btn-uninstall`.

When the row is too narrow for all buttons, rightmost buttons are dropped first
(graceful degradation — keyboard shortcuts still work).

---

## 2. Multi-Step Remove Modal

### Current Flow (broken)

Single confirm with locked "Delete from library" checkbox and pre-checked provider
checkboxes. Confusing because: can't interact with locked checkbox, not clear what's
happening, no selective uninstall.

### New Flow: 3-Step Overlay Modal

The Remove action becomes a multi-step overlay modal. Same visual size as current
confirm modal but content changes per step. The modal grows taller when needed
(many providers) but stays the same width.

**Step 1 — Confirm Removal:**

```
+-- Remove "my-hook"? -------------------------+
|                                               |
|  This will remove "my-hook" from your         |
|  library.                                     |
|                                               |
|  This action cannot be undone.                |  ← red text
|                                               |
|  This content is installed in 2 providers.    |  ← only if installed
|  Uninstall from them too?                     |
|                                               |
|     [Cancel]    [Remove Only]    [Yes]        |
+-----------------------------------------------+
```

**Button behavior:**
- **Cancel** — close modal, no changes
- **Remove Only** — skip to Step 3 review (will NOT uninstall from providers)
- **Yes** — go to Step 2 (provider selection)

**When not installed anywhere:**
- The "installed in N providers" section is absent
- Buttons: `[Cancel]  [Remove]` (two buttons, no third option needed)

**Step 2 — Provider Selection (conditional):**

```
+-- Uninstall from providers -------------------+
|                                               |
|  Select providers to uninstall from:          |
|                                               |
|  [ ] Claude Code                              |
|  [ ] Cursor                                   |
|  [ ] Windsurf                                 |
|                                               |
|          [Back]    [Done]                     |
+-----------------------------------------------+
```

- All checkboxes **unchecked by default** (opt-in)
- Space toggles, j/k or up/down navigates
- Tab cycles: checkboxes → Back → Done → checkboxes
- Back returns to Step 1
- Done goes to Step 3 with selections
- If no checkboxes checked, Done goes to Step 3 showing "no providers selected"

**Step 3 — Review:**

```
+-- Review ------------------------------------+
|                                               |
|  Will remove "my-hook" from library.          |
|                                               |
|  Will uninstall from:                         |  ← only if providers selected
|    Claude Code                                |
|    Cursor                                     |
|                                               |
|  Still installed in:                          |  ← only if skipped/unchecked providers
|    Windsurf                                   |
|                                               |
|      [Cancel]    [Back]    [Remove]           |
+-----------------------------------------------+
```

- **Cancel** — close modal, no changes
- **Back** — return to previous step (Step 2 if providers were selected, Step 1 otherwise)
- **Remove** — execute the removal

**Section visibility rules:**
- "Will uninstall from" — shown only if user selected providers in Step 2
- "Still installed in" — shown only if the item is installed somewhere AND the user
  didn't select all providers. This is the key UX improvement: users always know
  what they're leaving in place.
- If item was never installed anywhere, neither section appears (just "Will remove
  from library")

### Step Transitions

```
Not installed:  Step 1 (Cancel/Remove) ──→ execute
                  │
Installed:      Step 1 (Cancel/Remove Only/Yes)
                  │           │
                  │     Step 2 (Back/Done) ──→ Step 3 (Cancel/Back/Remove)
                  │                                      │
                  └──────── Step 3 (Cancel/Back/Remove) ─┘──→ execute
```

### Modal Sizing

- Width: `min(54, terminal_width - 10)`, minimum 34. Slightly wider than current
  (50) to prevent title wrapping on typical item names.
- Height: grows with content. Provider lists of 11-12 entries will make Step 2 tall
  — that's fine, the overlay function handles vertical centering.
- Title is constrained to `usableW` with `MaxWidth` to prevent wrapping past border.

---

## 3. Modal Button Navigation: Left/Right

### Current Behavior

Only Tab/Shift+Tab and Up/Down move between buttons. Left/Right is not handled
in the confirm modal (it does nothing).

### New Behavior

When focus is on any button:
- **Left** → move to previous button (wrap from first to last)
- **Right** → move to next button (wrap from last to first)

When focus is on a checkbox:
- Left/Right do nothing (consistent with checkbox interaction — Space toggles)

This matches the editModal pattern where Left/Right moves between Cancel and Save
when on button focus (modal.go lines 179-187).

---

## 4. Title Wrapping Fix

The title (`"Remove \"very-long-item-name\"?"`) is rendered without width constraint.
On narrow terminals or long names, it wraps past the modal border.

**Fix:** Apply `MaxWidth(usableW)` to the title text, same as body lines already do.
If the name is too long, it gets truncated with ellipsis rather than wrapping.

---

## Testing Requirements

### Action Buttons

- Buttons render on Row 3 (not Row 2)
- Context-sensitive: Library shows [a][d][x], Loadouts shows [d], Config shows none
- Mouse clicks on button zones fire correct actions
- Narrow terminal: buttons degrade gracefully (rightmost dropped)
- Golden files updated for all sizes

### Multi-Step Remove Modal

**Step 1 tests:**
- Not installed: shows Cancel/Remove only (no provider section, no "Yes" button)
- Installed: shows provider count, Cancel/Remove Only/Yes buttons
- Cancel → modal closes
- Remove Only → jumps to Step 3 review
- Yes → goes to Step 2

**Step 2 tests:**
- All checkboxes unchecked by default
- Space toggles checkbox
- Back → returns to Step 1
- Done with selections → Step 3 shows selected providers
- Done with no selections → Step 3 shows "no providers selected to uninstall"

**Step 3 tests:**
- Shows "Will remove from library"
- Shows "Will uninstall from: [list]" when providers selected
- Shows "Still installed in: [list]" when providers exist but not all selected
- Cancel → closes modal
- Back → returns to previous step
- Remove → fires remove command with correct provider list

**Step transition tests:**
- Not installed: Step 1 → Remove → execute (skip Steps 2-3)
- Installed, Remove Only: Step 1 → Step 3 → Remove → execute (skip Step 2)
- Installed, Yes: Step 1 → Step 2 → Step 3 → Remove → execute
- Back navigation preserves checkbox state across steps

### Button Navigation

- Left/Right moves between buttons when focused on buttons
- Left/Right wraps (first ← last)
- Left/Right is no-op when focused on checkboxes
- Tab still cycles through all focusable elements

### Title Wrapping

- Long title truncated, does not wrap past border
- Golden file with long item name verifies no wrapping

---

## Non-Goals

- Full-screen wizard for Remove (decided: overlay modal is sufficient)
- Cross-reference detection (deferred: syllago-91l3q)
- Buttons for unimplemented actions ([i] Install, [s] Share — Phase B/E)
