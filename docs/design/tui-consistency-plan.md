# TUI Consistency Implementation Plan

**Goal:** Bring all TUI pages and components into full compliance with the design spec (`docs/design/tui-spec.md`) and enforced rules (`.claude/rules/tui-*.md`).

**Approach:** Phased, incremental. Each task is self-contained, testable, and committable independently. Phases are ordered so that foundational changes (shared infrastructure) land before page-specific work that depends on them.

**Total issues:** 40 (25 High, 12 Medium, 3 Low)

---

## Phase 1: Card Grid Foundation

**Why first:** Cards are the most visibly broken component. Registry cards are tiny and misshapen, Library/Loadout cards lack breadcrumbs, and the Homepage cards can't be keyboard-navigated. Fixing this requires shared infrastructure (consistent styles, sizing logic) that later phases build on.

### Task 1.1: Standardize card styles across all pages

**What:** Replace all inline card style construction with `cardNormalStyle`/`cardSelectedStyle` from styles.go. Currently, Homepage (app.go:2315-2322, 2383-2390), Library (app.go:2553-2559), and Loadout (app.go:2621-2627) cards build styles inline with `lipgloss.NewStyle()`. Only Registries uses the shared styles.

**Why:** The spec mandates a single source of truth for card appearance. Inline styles drift — the Homepage config cards already have a different min width (16 vs 18) and breakpoint (56 vs 42) from other card pages.

**Changes:**
- `app.go`: In `renderWelcomeCards()`, replace inline `cardStyle` construction (lines 2315-2322) with `cardNormalStyle.Width(cardW).Height(3)` (two-col) or `cardNormalStyle.Width(cardW)` (single-col). Same for config section (lines 2383-2390).
- `app.go`: In `renderLibraryCards()`, replace `cardBase` construction (lines 2553-2559) with `cardNormalStyle`/`cardSelectedStyle`.
- `app.go`: In `renderLoadoutCards()`, replace `cardBase` construction (lines 2621-2627) with `cardNormalStyle`/`cardSelectedStyle`.
- `app.go`: Fix config card breakpoint from `contentW < 56` to `contentW < 42` (line 2375).
- `app.go`: Fix config card min width from `16` to `18` (line 2380).

**Success criteria:**
- All card pages render cards using `cardNormalStyle`/`cardSelectedStyle` from styles.go
- No `lipgloss.NewStyle().Border(lipgloss.RoundedBorder())` card construction remains outside styles.go
- Config cards use same breakpoint (42) and min width (18) as all other cards
- Visual appearance unchanged (borders, colors, padding identical)

**Testing:**
- `make test` passes
- Update golden files: `go test ./cli/internal/tui/ -update-golden`
- Review golden diffs to confirm visual output is identical or improved
- Test at 60x20, 80x30, 120x40, 160x50

---

### Task 1.2: Fix registry card sizing

**What:** Replace hardcoded `cardWidth := 36` in registries.go with the standard dynamic sizing formula. Also fix the column breakpoint from `m.width < 80` to `contentW < 42`, and the column gap from 2 spaces to 1.

**Why:** Registry cards are visibly broken — they're tiny fixed-width boxes that don't fill the content area, making the page look completely different from Library/Loadout cards.

**Changes:**
- `registries.go`: Replace `cardWidth := 36` (line 76) with dynamic calculation:
  ```go
  contentW := m.width
  singleCol := contentW < 42
  cardW := (contentW - 5) / 2
  if singleCol { cardW = contentW - 2 }
  if cardW < 18 { cardW = 18 }
  ```
- `registries.go`: Replace `if m.width < 80 { cols = 1 }` (line 78) with the `singleCol` variable.
- `registries.go`: Fix column gap from `"  "` to `" "` and use `lipgloss.JoinHorizontal` for layout (lines 90-94).
- `registries.go`: Pass `cardW` to `renderRegistryCard()` instead of hardcoded width.

**Success criteria:**
- Registry cards fill the content area at the same width as Library/Loadout cards
- Single-column triggers at `contentW < 42`, not terminal width 80
- Column gap is 1 character, matching other card pages
- Cards scale correctly at all 4 golden sizes

**Testing:**
- `make test` passes
- Update golden files
- Visually compare registry page at 60x20, 80x30, 120x40, 160x50
- Compare side-by-side with Library cards at same size — should look identical in layout

---

### Task 1.3: Add breadcrumbs to Library and Loadout card pages

**What:** Add `renderBreadcrumb()` calls to `renderLibraryCards()` and `renderLoadoutCards()`, replacing the inline title/subtitle.

**Why:** Every page except Homepage should have a breadcrumb. Library and Loadout card pages are the only ones missing them.

**Changes:**
- `app.go`: In `renderLibraryCards()` (line 2541), replace:
  ```go
  s += "\n" + titleStyle.Render("  Library") + "\n"
  s += helpStyle.Render("  Browse library items by content type") + "\n\n"
  ```
  with:
  ```go
  s += renderBreadcrumb(
      BreadcrumbSegment{"Home", "crumb-home"},
      BreadcrumbSegment{"Library", ""},
  ) + "\n\n"
  ```
- `app.go`: Same pattern for `renderLoadoutCards()` (line 2609) with `"Loadouts"`.

**Success criteria:**
- Library page shows `Home > Library` breadcrumb at top
- Loadout page shows `Home > Loadouts` breadcrumb at top
- Clicking "Home" in the breadcrumb navigates back to homepage
- Breadcrumb style matches other pages (Items, Registries, Settings, etc.)

**Testing:**
- `make test` passes
- Update golden files
- Click-test breadcrumb navigation in both directions

---

### Task 1.4: Fix Update page breadcrumb to use shared helper

**What:** Replace the custom inline breadcrumb in update.go with `renderBreadcrumb()`.

**Why:** Consistency. The Update page builds its breadcrumb manually instead of using the shared helper. If breadcrumb styling changes, this page would be out of sync.

**Changes:**
- `update.go`: Replace custom breadcrumb (line 295) with `renderBreadcrumb(BreadcrumbSegment{"Home", "crumb-home"}, BreadcrumbSegment{"Update", ""})`.

**Success criteria:**
- Update page breadcrumb looks identical but uses shared helper
- Clicking "Home" navigates back

**Testing:**
- `make test` passes
- Update golden files if Update page has golden tests

---

## Phase 2: Keyboard and Focus

**Why second:** With cards visually consistent, we can add the keyboard navigation and focus behavior that makes them interactive. These changes are in app.go's Update() method and don't affect rendering.

### Task 2.1: Enable Tab focus and Search on Registries page

**What:** Remove `screenRegistries` from the Tab toggle and Search exclusion lists.

**Why:** The spec says Tab and Search work on all card pages. Registries is currently excluded from both, making it the only card page where you can't Tab to content or search.

**Changes:**
- `app.go` line 1415: Remove `&& a.screen != screenRegistries` from the Tab condition.
- `app.go` line 1323: Remove `&& a.screen != screenRegistries` from the Search condition.

**Success criteria:**
- On Registries page, pressing Tab toggles between sidebar and card content
- On Registries page, pressing `/` opens search bar
- Search filters registry cards by name/URL/description
- Tab still works correctly on all other card pages (no regressions)

**Testing:**
- Add test: Tab on Registries toggles focus
- Add test: `/` on Registries activates search
- `make test` passes
- No golden file changes expected (these are behavioral, not visual)

---

### Task 2.2: Add keyboard navigation for Homepage welcome cards

**What:** When `focus == focusContent` on `screenCategory`, enable arrow-key navigation through welcome cards with Enter to drill in.

**Why:** Homepage cards are currently click-only. Every other card page supports keyboard navigation. Users expect Tab to focus the cards and arrows to navigate them.

**Changes:**
- `app.go`: In the `screenCategory` case (line 1430), add a `focusContent` branch:
  - Track `cardCursor` position across all three card sections (Content, Collections, Configuration) as a flat list
  - Up/Down move by column count (2 or 1 depending on layout)
  - Left/Right move by 1
  - Enter drills into the selected card (same action as click)
  - Home/End jump to first/last card
- `app.go`: In `renderWelcomeCards()`, add selection styling — apply `cardSelectedStyle` when card index matches `cardCursor`

**Success criteria:**
- Tab from sidebar focuses the card grid, showing a selection highlight
- Arrow keys move the selection through all cards across all three sections
- Enter on a Content card navigates to that category's items
- Enter on a Collection card navigates to Library/Loadouts/Registries
- Enter on a Configuration card navigates to Add/Update/Settings/Sandbox
- Selection wraps correctly at section boundaries

**Testing:**
- Add test: Tab focuses content, arrows navigate cards, Enter drills in
- Add test: cursor clamps at bounds (can't go past first/last card)
- Update golden files (selection highlight will change appearance)
- Test at 60x20 (single-col) and 80x30 (two-col) to verify column-aware movement

---

### Task 2.3: Complete help overlay for all screens

**What:** Add help overlay sections for the 5 missing screen types: Library Cards, Loadout Cards, Registries, Update, Sandbox.

**Why:** The spec says every screen type must have a help overlay section. Currently 5 screens show no context-sensitive shortcuts when you press `?`.

**Changes:**
- `help_overlay.go`: Add case blocks for `screenLibraryCards`, `screenLoadoutCards`, `screenRegistries`, `screenUpdate`, `screenSandbox` with their relevant keyboard shortcuts.

**Success criteria:**
- Pressing `?` on any screen shows context-sensitive shortcuts for that screen
- No screen type falls through to "no shortcuts available"
- Shortcuts listed match the actual bindings for each screen

**Testing:**
- Add test: help overlay content for each new screen type
- `make test` passes

---

## Phase 3: Modal Consistency

**Why third:** Modals are self-contained overlays. Fixing them doesn't affect page layout. This phase standardizes width, buttons, and adds mouse support to all modals.

### Task 3.1: Standardize modal widths to 56

**What:** Change confirmModal (40), saveModal (40), and loadoutCreateModal (64) to use width 56.

**Why:** The spec standardizes on one modal width. Having 3 different widths means modals visually jump when switching between them, and text wrapping behaves differently in each.

**Changes:**
- `modal.go`: confirmModal — change `modalWidth = 40` to `56`, `modalHeight = 10` stays (or adjust if content needs it), update `renderButtons` contentWidth from `36` to `52`.
- `modal.go`: saveModal — change `Width(40)` to `Width(56)`, update input width.
- `loadout_create.go`: Change `createLoadoutModalWidth = 64` to `56`, adjust `createLoadoutModalHeight` if 24 is too tall for 60x20 minimum.

**Success criteria:**
- All modals render at 56 characters wide
- No modal exceeds terminal height at 60x20 minimum
- Button alignment looks correct at new width
- Text inputs fit within the wider/narrower modal

**Testing:**
- `make test` passes
- Update golden files if modals have golden tests
- Visual check at 60x20 to ensure modals fit

---

### Task 3.2: Replace inline help text with renderButtons() in all modals

**What:** saveModal and loadoutCreateModal use inline help text (`"[Enter] Save [Esc] Cancel"`) instead of styled buttons. envSetupModal uses inline help on some steps. Replace all with `renderButtons()`.

**Why:** Consistent visual language. Users should see the same styled button pair everywhere, not sometimes buttons and sometimes text hints.

**Changes:**
- `modal.go`: saveModal View() — replace `helpStyle.Render("[Enter] Save   [Esc] Cancel")` with `renderButtons("Save", "Cancel", m.btnCursor, 52)`. Add `btnCursor` field and Left/Right handling in Update().
- `loadout_create.go`: Replace inline help on steps 2-4 with `renderButtons()`. Add button cursor tracking per step.
- `modal.go`: envSetupModal — evaluate each step's help text and replace with `renderButtons()` where a confirm/cancel pair applies.

**Success criteria:**
- Every modal with confirm/cancel actions shows styled button pair
- No `helpStyle.Render("[Enter]...")` patterns remain in modal View() methods
- Left/Right switches buttons, Enter activates current button
- Default cursor position follows spec (Cancel for destructive, Confirm for safe)

**Testing:**
- Add/update tests for button navigation in each modified modal
- `make test` passes
- Update golden files

---

### Task 3.3: Add mouse support to all modal buttons

**What:** Wrap modal buttons in `zone.Mark()` and add `tea.MouseMsg` handling in each modal's Update() method.

**Why:** The spec requires every interactive element to support both keyboard and mouse. Currently NO modal supports mouse clicks on buttons.

**Changes:**
- `modal.go`: Update `renderButtons()` to wrap each button in `zone.Mark("modal-btn-{left}", ...)` and `zone.Mark("modal-btn-{right}", ...)`.
- `modal.go`: Add `tea.MouseMsg` case to confirmModal, saveModal, installModal, envSetupModal, registryAddModal Update() methods. Check `zone.Get("modal-btn-confirm").InBounds(msg)` etc.
- `loadout_create.go`: Same for loadoutCreateModal.
- Also add zone marks for clickable option lists in installModal and envSetupModal.

**Success criteria:**
- Clicking a modal button activates it (same as pressing Enter with cursor on it)
- Clicking outside modal-zone dismisses it (already works via App-level handler)
- Clickable options (radio items) respond to mouse click
- All existing keyboard behavior preserved

**Testing:**
- Add mouse click tests for each modal
- `make test` passes

---

## Phase 4: Scroll Support

**Why fourth:** Scroll requires the card grid and keyboard infrastructure from Phases 1-2 to be in place. Adding scroll to card pages builds on the cursor navigation added in Phase 2.

### Task 4.1: Add scroll support to card grid pages

**What:** Implement scroll for Homepage, Library, Loadout, and Registry card grids so cards that overflow the viewport are accessible.

**Why:** On small terminals or with many items, cards disappear off the bottom with no indication. Users can't access content they can't see.

**Changes:**
- `app.go`: Add `cardScrollOffset int` field to App.
- `app.go`: In card rendering functions, calculate visible card range based on `cardScrollOffset` and viewport height. Render `renderScrollUp()` / `renderScrollDown()` indicators.
- `app.go`: In card keyboard handling, auto-scroll to keep cursor visible when it moves past viewport edge.
- `app.go`: Reset `cardScrollOffset` to 0 when navigating to a different page.
- `registries.go`: Same pattern — add scroll state, render indicators.

**Success criteria:**
- Card pages with more cards than fit on screen show scroll indicators
- Cursor movement auto-scrolls to keep selection visible
- PgUp/PgDown jump by viewport height
- Home/End jump to first/last card
- Scroll resets when navigating away

**Testing:**
- Add golden tests with large datasets on card pages
- Test at 60x20 with many items to verify scroll indicators appear
- Test cursor clamp at bounds with scroll
- `make test` passes
- Update golden files

---

### Task 4.2: Add scroll support to help overlay

**What:** Add scroll to the help overlay so it works on small terminals where the shortcut list exceeds viewport height.

**Why:** At 60x20, the help overlay can easily exceed the visible area, cutting off shortcuts with no way to see them.

**Changes:**
- `help_overlay.go`: Add `scrollOffset int` field. Calculate visible lines based on available height. Add `renderScrollUp()` / `renderScrollDown()` indicators. Handle Up/Down keys for scrolling.

**Success criteria:**
- Help overlay scrolls when content exceeds viewport height
- Scroll indicators appear when content is clipped
- Up/Down scroll the overlay content
- Esc still dismisses

**Testing:**
- Test at 60x20 with full help content
- `make test` passes

---

### Task 4.3: Add mouse wheel scroll to card pages, items, and sidebar

**What:** Add `tea.MouseMsg` wheel handling in App.Update() for card pages, the items list, and the sidebar.

**Why:** The spec requires mouse wheel scroll on all scrollable areas. Currently only Detail and Import handle wheel events.

**Changes:**
- `app.go`: In the `tea.MouseMsg` handler (lines 728-760), add cases for `screenCategory`, `screenLibraryCards`, `screenLoadoutCards`, `screenRegistries`, `screenItems` to handle `tea.MouseWheelUp` / `tea.MouseWheelDown`.
- `app.go`: Add sidebar wheel handling when mouse position is within sidebar bounds.

**Success criteria:**
- Mouse wheel scrolls card grids on all 4 card pages
- Mouse wheel scrolls the items list
- Mouse wheel scrolls the sidebar when items overflow
- Scroll direction: wheel up = scroll up, wheel down = scroll down

**Testing:**
- Add mouse wheel tests for each scrollable area
- `make test` passes

---

## Phase 5: Text Handling Cleanup

**Why last:** Text handling issues are lower severity — they cause visual glitches on edge cases (very long URLs, MCP commands) but don't break core functionality.

### Task 5.1: Standardize text truncation across all files

**What:** Replace all manual string slicing with the `truncate()` helper. Fix the possible truncation bug in registries.go. Add truncation to MCP Command and URL fields in detail_render.go.

**Why:** Manual slicing is error-prone (the `width-7` vs `width-4` discrepancy in registries.go is likely a bug) and inconsistent.

**Changes:**
- `registries.go`: Replace manual URL slicing (line 119) and description slicing (line 124) with `truncate()`. Fix the `width-7` to `width-4`.
- `detail_render.go`: Replace manual hook command slicing (line 197) with `truncate()`.
- `detail_render.go`: Add truncation for MCP Command field (line 390) and MCP URL field (line 393).
- Verify `truncate()` is accessible from all files that need it (may need to move from items.go to a shared location if not already).

**Success criteria:**
- All text truncation uses the `truncate()` helper — no manual `[:n-3] + "..."` patterns
- MCP Command and URL fields truncate to content width instead of wrapping
- No visual regressions on normal-length text

**Testing:**
- Add test with very long registry URL (200+ chars)
- Add test with very long MCP command + args
- `make test` passes
- Update golden files if truncation changes visible output

---

## Implementation Order

```
Phase 1: Card Grid Foundation
  1.1  Standardize card styles ──────────────────────── (app.go)
  1.2  Fix registry card sizing ─────────────────────── (registries.go)
  1.3  Add breadcrumbs to Library/Loadout ────────────── (app.go)
  1.4  Fix Update page breadcrumb ───────────────────── (update.go)

Phase 2: Keyboard and Focus
  2.1  Enable Tab/Search on Registries ──────────────── (app.go)
  2.2  Add keyboard nav for Homepage cards ──────────── (app.go)
  2.3  Complete help overlay for all screens ─────────── (help_overlay.go)

Phase 3: Modal Consistency
  3.1  Standardize modal widths to 56 ───────────────── (modal.go, loadout_create.go)
  3.2  Replace inline help with renderButtons() ─────── (modal.go, loadout_create.go)
  3.3  Add mouse support to modal buttons ───────────── (modal.go, loadout_create.go)

Phase 4: Scroll Support
  4.1  Add scroll to card grid pages ────────────────── (app.go, registries.go)
  4.2  Add scroll to help overlay ───────────────────── (help_overlay.go)
  4.3  Add mouse wheel to cards/items/sidebar ────────── (app.go)

Phase 5: Text Handling
  5.1  Standardize text truncation ──────────────────── (registries.go, detail_render.go)
```

Each task is independently committable. Run `make test` and update golden files after every task.
