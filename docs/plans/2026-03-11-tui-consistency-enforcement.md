# TUI Consistency Enforcement Plan

**Goal:** Make TUI conventions self-enforcing so no session (human or AI) can introduce inconsistencies. Four-part approach: fix existing inconsistencies, extract shared utilities, strengthen documentation, and add enforcement hooks.

**Context:** Audit found 85% consistency across 15 TUI files. Seven files have inconsistencies (modal.go, sidebar.go, registries.go, filebrowser.go, import.go, loadout_create.go, detail_render.go). Current test suite covers happy paths with small datasets at multiple sizes but lacks boundary-condition scenarios.

---

## Phase 1: Extract Shared Page Utilities (convention-as-code)

**Why first:** This changes the code patterns that all other phases reference. Documentation and rules should describe the final state, not the current one.

### Deep Audit Results (exact locations)

Before implementing, here's what the code audit found across every page model:

**Breadcrumb duplication (5 files, same zone.Mark + helpStyle + arrow pattern):**
- `items.go:236-253` — 4 switch cases for registry/search/library/parent breadcrumbs (most complex)
- `detail_render.go:29-47` — handles parentLabel + badge suffixes ([BUILT-IN], [LIBRARY], etc.)
- `registries.go:65-66` — simple `Home > Registries`
- `settings.go:116` — simple `Home > Settings`
- `sandbox_settings.go:158-159` — simple `Home > Sandbox`
- `filebrowser.go:370` — no breadcrumb, shows filesystem path (intentionally different)

**Cursor symbol inconsistency (sidebar vs everything else):**
- `sidebar.go:104,127,141,155-162,189` — uses `▸` in `selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", ...))`
- All 7 other files — use `>` with `prefix := "> "` / `prefix := "  "` pattern

**Status message field naming (4 different patterns):**
- `detail.go` — `message string` + `messageIsErr bool` (canonical)
- `settings.go` — `message string` + `messageErr bool` (wrong bool name)
- `sandbox_settings.go` — `message string` + `messageErr bool` (wrong bool name)
- `registries.go` — passed as View() params `statusMsg, statusIsErr` (not on model)
- `filebrowser.go:36` — `errMsg string` only (no success path, wrong field name)
- `loadout_create.go:67-68` — `message string` + `messageIsErr bool` (correct)

**Status message rendering (3 prefix patterns):**
- `detail_render.go:650-656` — `"Error: " + msg` / `"Done: " + msg`
- `settings.go:135-139` — `"Error: " + msg` / `"Done: " + msg`
- `sandbox_settings.go:186-190` — `"Error: " + msg` / `"Done: " + msg`
- `registries.go:69-73` — no prefix, just styled msg
- `filebrowser.go:373` — no prefix, `errorMsgStyle.Render(fb.errMsg)`
- `loadout_create.go:430` — `errorMsgStyle.Render(m.message)` (no prefix)

**Help text method naming (3 different names):**
- `detail_render.go:661` — `renderHelp() string` (builds from parts array with `" • "`)
- `registries.go:60` — `helpText() string` (static string)
- `settings.go:172` — `helpText() string` (static string)
- `sandbox_settings.go:197` — `helpText() string` (conditional)
- `import.go` — `helpText() string` (per-step)
- `items.go:476-480` — inline in View(), no method
- `filebrowser.go` — no help text method at all
- `loadout_create.go:422,428,449` — inline per-step in View()
- `app.go:2622,2638-2640` — inline for items/libraryCards/loadoutCards screens

**Scroll indicator wording (intentional but worth documenting):**
- List scrolling: `"(%d more above)"` / `"(%d more below)"` — items.go, filebrowser.go
- Content scrolling: `"(%d lines above)"` / `"(%d lines below)"` — detail_render.go, filebrowser preview

### Task 1.1: Create shared rendering helpers

**File:** `cli/internal/tui/pagehelpers.go` (new)

```go
// BreadcrumbSegment represents one piece of a breadcrumb trail.
// If ZoneID is non-empty, the segment is wrapped in zone.Mark and rendered as clickable (helpStyle).
// If ZoneID is empty, the segment is rendered as the current location (titleStyle, not clickable).
type BreadcrumbSegment struct {
    Label  string
    ZoneID string // empty = final segment (titleStyle), non-empty = clickable (helpStyle)
}

// renderBreadcrumb renders a clickable breadcrumb trail with " > " separators.
// The last segment with an empty ZoneID is rendered in titleStyle (current location).
// All other segments are rendered in helpStyle and wrapped in zone.Mark for click handling.
//
// Usage:
//   renderBreadcrumb(
//       BreadcrumbSegment{"Home", "crumb-home"},
//       BreadcrumbSegment{"Skills", ""},          // final = titleStyle
//   )
//
// With parent:
//   renderBreadcrumb(
//       BreadcrumbSegment{"Home", "crumb-home"},
//       BreadcrumbSegment{"Registries", "crumb-registries"},
//       BreadcrumbSegment{"my-registry", ""},      // final = titleStyle
//   )
func renderBreadcrumb(segments ...BreadcrumbSegment) string

// renderStatusMsg renders a transient status message using success or error styling.
// Returns empty string if msg is empty. Always prefixes with "Done: " or "Error: "
// for consistent user feedback across all screens.
//
// Usage:
//   s += renderStatusMsg(m.message, m.messageIsErr)
func renderStatusMsg(msg string, isErr bool) string

// cursorPrefix returns the cursor prefix string and appropriate style for a list item.
// Selected items get "> " prefix with selectedItemStyle.
// Unselected items get "  " prefix with itemStyle.
//
// Usage:
//   prefix, style := cursorPrefix(i == m.cursor)
//   row := fmt.Sprintf("  %s%s", prefix, style.Render(label))
func cursorPrefix(selected bool) (string, lipgloss.Style)

// renderScrollUp returns a scroll indicator for items above the viewport.
// For list views, uses "more above"; for content views, uses "lines above".
//
// Usage:
//   if offset > 0 { s += renderScrollUp(offset, false) + "\n" }
func renderScrollUp(count int, isContentView bool) string

// renderScrollDown returns a scroll indicator for items below the viewport.
//
// Usage:
//   if end < total { s += renderScrollDown(total-end, false) + "\n" }
func renderScrollDown(count int, isContentView bool) string
```

**Why these helpers and not others:**
- `renderBreadcrumb`: Duplicated verbatim in 5 files. The zone.Mark + helpStyle + arrow pattern is mechanical and error-prone to get wrong.
- `renderStatusMsg`: 6 files with 4 different prefix patterns. Standardizing to always use "Done:"/"Error:" eliminates the inconsistency.
- `cursorPrefix`: 8 files use cursor rendering. Enforces `>` everywhere (including sidebar), eliminates the `▸` inconsistency at the source.
- `renderScrollUp/Down`: 5 files with scroll indicators. Standardizes wording and styling.

// renderDescriptionBox renders a separator-bounded context box for the currently
// highlighted item. Fixed height prevents layout jitter when switching between items.
// Renders at the bottom of the content pane, above the footer.
// maxLines controls the box height (3 for normal terminals, 1-2 for small).
//
// Usage:
//   s += renderDescriptionBox(settingsDescriptions[m.cursor], 45, 3)
func renderDescriptionBox(text string, width int, maxLines int) string
```

**Why these helpers and not others:**
- `renderBreadcrumb`: Duplicated verbatim in 5 files. The zone.Mark + helpStyle + arrow pattern is mechanical and error-prone to get wrong.
- `renderStatusMsg`: 6 files with 4 different prefix patterns. Standardizing to always use "Done:"/"Error:" eliminates the inconsistency.
- `cursorPrefix`: 8 files use cursor rendering. Enforces `>` everywhere (including sidebar), eliminates the `▸` inconsistency at the source.
- `renderScrollUp/Down`: 5 files with scroll indicators. Standardizes wording and styling.
- `renderDescriptionBox`: Currently only in settings.go (lines 143-156). Needs to be reusable because this pattern should exist on every screen where the selected item isn't self-explanatory.

**What's NOT extracted (and why):**
- Help text content — too varied across components (static strings, conditional, per-step, composed from parts). Standardize the method name (`helpText()`) but not the implementation.
- Empty states — every empty state has different messaging. Not worth a helper.
- Modal container style — only 2 uses with different dimensions. A helper saves one line.

### Task 1.2: Standardize field and method naming

**Canonical names (all page models must use these):**
- `message string` — status message text
- `messageIsErr bool` — true = error, false = success
- `helpText() string` — returns help bar content for the footer

**Files to change for field naming:**

Message fields are removed entirely per Task 1.8 (centralized toast). Components return `toastMsg` commands instead. The following files lose their message fields:

| File | Fields removed | Replaced with |
|------|---------------|---------------|
| `detail.go` | `message string`, `messageIsErr bool` | Return `toastMsg{...}` from Update |
| `settings.go` | `message string`, `messageErr bool` | Return `toastMsg{...}` from `save()` |
| `sandbox_settings.go` | `message string`, `messageErr bool` | Return `toastMsg{...}` from `save()` |
| `filebrowser.go` | `errMsg string` | Return `toastMsg{...}` from `loadDir()` errors |
| `loadout_create.go` | `message string`, `messageIsErr bool` | Return `toastMsg{...}` for validation errors |
| `registries.go` | View() params removed | `View() string` (cursor moved to model field) |
| `app.go` | `statusMessage string`, `statusIsErr bool` | Route registry results through toast |

**Files to change for method naming:**

| File | Current | Change to |
|------|---------|-----------|
| `detail_render.go:661` | `renderHelp()` | `helpText()` |
| `app.go:2620` | `a.detail.renderHelp()` | `a.detail.helpText()` |

### Task 1.3: Fix known inconsistencies

| File | Lines | Fix |
|------|-------|-----|
| `sidebar.go` | 105,127,141,155,162,189 | Change `▸` to `>` — use `cursorPrefix()` helper |
| `registries.go` | 26-31, 64 | Add `cursor int`, `message string`, `messageIsErr bool` fields to `registriesModel` |
| `filebrowser.go` | 36, 86-96, 260, 372-374 | Rename `errMsg` → `message` + add `messageIsErr bool`, use `renderStatusMsg()` |
| `settings.go` | 20, 102, 106, 135 | Rename `messageErr` → `messageIsErr` |
| `sandbox_settings.go` | 26, 149, 153, 186 | Rename `messageErr` → `messageIsErr` |

### Task 1.4: Adopt shared helpers in all page models

For each file, replace inline implementations with the shared helpers:

**Breadcrumb adoption:**
| File | Lines | Replace inline breadcrumb with `renderBreadcrumb()` |
|------|-------|------------------------------------------------------|
| `items.go` | 236-253 | Complex: 4 switch cases → 4 calls to `renderBreadcrumb()` with different segments |
| `detail_render.go` | 29-47 | `renderBreadcrumb()` + badge suffix logic stays inline (badges are detail-specific) |
| `registries.go` | 65-66 | `renderBreadcrumb(home, title)` |
| `settings.go` | 116 | `renderBreadcrumb(home, title)` |
| `sandbox_settings.go` | 158-159 | `renderBreadcrumb(home, title)` |

**Status message adoption:**
Superseded by Task 1.8 (centralized toast system). Individual components no longer render status messages at all — they return `toastMsg` commands and App handles rendering via the toast overlay. The `renderStatusMsg()` helper is used internally by the toast renderer only, not by components.

**Cursor adoption (sidebar only — others already use `>`):**
| File | Lines | Replace inline cursor with `cursorPrefix()` |
|------|-------|----------------------------------------------|
| `sidebar.go` | 104-108, 126-130, 140-144, 155-165, 188-193 | Use `cursorPrefix()` to get prefix and style |

**Scroll indicator adoption:**
| File | Lines | Replace inline scroll indicators with `renderScrollUp/Down()` |
|------|-------|----------------------------------------------------------------|
| `items.go` | 374-376, 472-474 | `renderScrollUp(offset, false)` / `renderScrollDown(remaining, false)` |
| `detail_render.go` | 638-640, 645-647 | `renderScrollUp(offset, true)` / `renderScrollDown(remaining, true)` |
| `detail_render.go` | 357-359, 366-368 | File content: `renderScrollUp/Down(count, true)` |
| `filebrowser.go` | 388-390, 436-438 | `renderScrollUp(fb.offset, false)` / `renderScrollDown(remaining, false)` |
| `filebrowser.go` | 476-478, 485-487 | Preview: `renderScrollUp/Down(count, true)` |

### Task 1.5: Consolidate confirm shortcuts into keys.go

**Problem:** Confirm modals use `y/Y` and `n/N` shortcuts that are checked inline with `msg.String() == "y"` rather than going through keys.go bindings. This is an exception to the "all keys in keys.go" rule that should be eliminated.

**Files to change:**
- `keys.go` — add `keys.ConfirmYes` and `keys.ConfirmNo` bindings (y/Y and n/N)
- `modal.go` — replace inline `msg.String() == "y"` / `msg.String() == "n"` with `key.Matches(msg, keys.ConfirmYes)` etc.

### Task 1.6: Remove redundant inline help from content views

**Problem:** `items.go` renders shortcut help inline at the bottom of the items list (line 476-480) AND the footer shows the same shortcuts via `contextHelpText()` (app.go:2622). This duplicates information and is inconsistent with other screens that only use the footer.

**Rule:** Keyboard shortcuts belong in the footer bar only. The content pane should not contain inline help shortcuts. The one exception is modals (loadout_create.go, modal.go) — they overlay the footer, so inline help is the only option.

**Files to change:**
| File | Lines | Change |
|------|-------|--------|
| `items.go` | 476-481 | Remove the inline footer construction. The `contextHelpText()` in app.go already provides the footer help for items. |
| `items.go` | 260 | Remove redundant `"esc back"` on empty items view — footer handles this |

**What stays:** Modal inline help in `loadout_create.go:422,428,449` — these overlay the footer, so inline is correct.

### Task 1.7: Add description boxes to screens with non-obvious options

**Problem:** Settings (lines 143-156) has a description box that explains the currently highlighted row. This is great UX but it's the only screen that does it. Other screens with non-obvious options don't tell the user what the highlighted item does.

**The pattern (from settings.go):**
```go
// Bottom detail area (fixed 3-line height to prevent jitter)
if m.cursor >= 0 && m.cursor < len(descriptions) {
    const detailLines = 3
    s += "\n " + helpStyle.Render(strings.Repeat("─", boxWidth)) + "\n"
    lines := strings.Split(descriptions[m.cursor], "\n")
    for i := 0; i < detailLines; i++ {
        if i < len(lines) {
            s += " " + helpStyle.Render(lines[i]) + "\n"
        } else {
            s += "\n"
        }
    }
    s += " " + helpStyle.Render(strings.Repeat("─", boxWidth)) + "\n"
}
```

**Extract into `renderDescriptionBox()`** and adopt in these screens:

| Screen | What to describe | Description source |
|--------|-----------------|-------------------|
| **Settings** | Current setting row | Already has `settingsDescriptions` — refactor to use `renderDescriptionBox()` |
| **Sandbox Settings** | Current sandbox option | Add `sandboxDescriptions` slice: "Domains the sandbox allows network access to", "Environment variables passed through to sandboxed processes", "Network ports the sandbox allows outbound connections on" |
| **Library Cards** | Selected content type card | Use `categoryDesc` map (already exists at app.go:2047) |
| **Loadout Cards** | Selected provider card | Generate from provider metadata or use static descriptions |
| **Sidebar (category)** | Selected content type | Use `categoryDesc` map — show in the content welcome area when sidebar has focus |

**Placement rule:** Description box renders at the bottom of the content pane, above the footer. Fixed height (3 lines) with `─` separators to prevent jitter at normal sizes (80x30+). At small terminals (60x20), use dynamic height: min 1 line, max 2 lines, to avoid eating too much of the content area. The `renderDescriptionBox()` helper takes a `maxLines int` parameter — callers pass 3 normally, but can pass a smaller value based on available height.

### Task 1.8: Centralized toast system (replaces per-component status messages)

**Problem:** Every component manages its own `message`/`messageIsErr` fields and renders status messages differently (different placement, different prefixes, different styling). This is the single biggest source of inconsistency across the TUI.

**Solution:** Move all status message rendering to `App`. Components fire a message; App renders the toast.

#### Toast architecture

**New types:**

```go
// toastMsg is sent by any component to trigger a toast notification.
// App.Update() catches this and sets the active toast state.
type toastMsg struct {
    text  string
    isErr bool
}

// toastModel holds the state for the active toast overlay.
type toastModel struct {
    active       bool
    text         string
    isErr        bool
    scrollOffset int // for long error messages
}
```

**Two distinct behaviors based on `isErr`:**

| | Success Toast | Error Toast |
|--|---------------|-------------|
| **Border** | Green (`successColor`) | Red (`dangerColor`) |
| **Position** | Bottom of content pane, above footer | Bottom of content pane, above footer |
| **Width** | Fixed: content pane width minus 4 (responsive via WindowSizeMsg) | Same |
| **Height** | 1-2 lines, fixed (no auto-size — bubbles convention) | Fixed 5 lines, scrolls internally if content overflows |
| **Dismiss** | Any keypress (don't consume — let the key also trigger its normal action) | `Esc` only |
| **Copy** | No | `c` copies sanitized error text to clipboard via `atotto/clipboard` |
| **Focus** | Does not capture focus | Semi-modal: intercepts `c` and `Esc`, passes all other keys through |
| **Prefix** | "Done: " | "Error: " |
| **Footer hint** | None (disappears too fast) | Shows `"c copy • esc dismiss"` inside the toast |
| **Text wrapping** | Word-wrap via `muesli/reflow` within toast width | Same, with scroll for overflow |

**Text handling inside the toast:**
- Use `reflow/wordwrap` for ANSI-aware word wrapping within the toast's inner width
- Re-wrap on every `WindowSizeMsg` — do NOT cache wrapped text at toast creation time (terminal may resize while toast is visible)
- Apply `lipgloss.MaxWidth()` on the toast container as a hard-wrap safety net
- For error toasts with scroll: split wrapped text into lines, apply `scrollOffset`, render visible window

**Key routing for semi-modal error toasts:**
- Error toast intercepts `c` (copy) and `Esc` (dismiss) — all other keys fall through to normal handling AND the toast stays visible
- Success toast dismisses on any keypress but does NOT consume the keypress — the key triggers its normal action too (key queueing: user pressing `j` during a success flash should navigate down, not just dismiss)
- When `focusModal` is active, modals take priority over toast key handling

**Error message sanitization (before clipboard copy):**
- Create `sanitizeForClipboard(msg string) string` in `toast.go`
- Strip absolute file paths (replace `/home/user/...` with relative or `<path>`)
- Strip git remote URLs (may contain tokens)
- Strip environment variable values that look like secrets (`*_KEY`, `*_SECRET`, `*_TOKEN`)
- The raw error is still displayed in the toast — sanitization only applies to what goes to the clipboard

**Clipboard dependency:**
- Add `github.com/atotto/clipboard` to go.mod
- On Linux/WSL, requires `xclip` or `xsel` (document in README)
- Graceful fallback: if clipboard write fails, show "Copy failed — clipboard tool not found" as a brief inline message inside the toast

#### Rendering (in App.View)

Toast renders AFTER panel composition but BEFORE modals, using `bubbletea-overlay`:

```go
// In App.View(), after composing sidebar + content + footer:
if a.toast.active {
    body = a.toast.overlayView(body, a.width, a.height)
}
// Then modals overlay on top of everything (including toast)
```

Position: bottom-center of the content pane (not full-width, not sidebar area).

#### Key routing (in App.Update)

```go
// When toast is active and is an error toast:
if a.toast.active && a.toast.isErr {
    switch {
    case key.Matches(msg, keys.Back): // Esc
        a.toast.active = false
        return a, nil
    case msg.String() == "c":
        // Copy a.toast.text to clipboard
        a.toast.active = false
        return a, nil
    }
    // All other keys fall through to normal handling — toast stays visible
}

// When toast is active and is a success toast:
if a.toast.active && !a.toast.isErr {
    a.toast.active = false
    // Don't consume the keypress — let it also be handled normally
}
```

#### Migration: remove per-component message fields

Every component that currently has `message`/`messageIsErr` fields changes to return a `toastMsg` command instead:

**Before (per-component pattern):**
```go
// In component Update():
m.message = "Installed to Claude Code"
m.messageIsErr = false

// In component View():
if m.message != "" {
    if m.messageIsErr {
        s += errorMsgStyle.Render("Error: " + m.message)
    } else {
        s += successMsgStyle.Render("Done: " + m.message)
    }
}
```

**After (centralized toast):**
```go
// In component Update():
return m, func() tea.Msg {
    return toastMsg{text: "Installed to Claude Code", isErr: false}
}

// In component View():
// Nothing — toast rendering is App's responsibility
```

**Files that lose message fields:**

| File | Fields to remove | Return toastMsg instead |
|------|-----------------|------------------------|
| `detail.go` | `message string`, `messageIsErr bool` | All places that set `m.message = ...` |
| `settings.go` | `message string`, `messageErr bool` | `save()` method |
| `sandbox_settings.go` | `message string`, `messageErr bool` | `save()` method |
| `loadout_create.go` | `message string`, `messageIsErr bool` | Validation errors ("Name is required") |
| `filebrowser.go` | `errMsg string` | `loadDir()` errors, file read errors |
| `registries.go` | (currently on App as `statusMessage`/`statusIsErr`) | Registry add/remove/sync results |
| `modal.go` (envSetupModal) | `message string`, `messageIsErr bool` | Env save results |

**Note on registries:** `App` currently has `statusMessage`/`statusIsErr` fields specifically for the registries screen. These also get replaced by the toast system.

#### New file

**`cli/internal/tui/toast.go`** — contains `toastModel`, `toastMsg`, and the `overlayView` rendering method. Follows the same overlay pattern as modals.

**Verification:** `make test` must pass. Golden files will change (status messages no longer inline in component views). Regenerate and verify diffs.

---

## Phase 2: Boundary-Condition Test Scenarios

**Why:** The recurring bug pattern is "what happens with lots of data?" Current test catalog has 8 items total. Test helpers don't create overflow scenarios.

### Task 2.1: Create large test catalog factory

**File:** `cli/internal/tui/testhelpers_test.go` (extend)

```go
// testCatalogLarge creates a catalog with 50+ items per content type
// to test scroll, truncation, and overflow behavior.
// Uses in-memory catalog items (no temp dir files) for speed — the
// large catalog only tests list rendering, not file content display.
func testCatalogLarge(t *testing.T) *catalog.Catalog

// testAppLarge creates a fully-wired App using testCatalogLarge.
func testAppLarge(t *testing.T) App
func testAppLargeSize(t *testing.T, width, height int) App
```

The large catalog should include:
- 50 skills with varying name lengths (some very long names that need truncation)
- 20 agents
- 15 MCP configs
- Items with very long descriptions (200+ chars)
- Items with empty descriptions
- Items with special characters in names

**Performance note:** Unlike `testCatalog()` which creates real temp dir files (needed for file viewer tests), the large catalog constructs `catalog.ContentItem` structs directly without filesystem I/O. This keeps the test fast even with 85+ items. Only set `Path` fields if a test specifically needs file content.

### Task 2.2: Add overflow golden tests

**File:** `cli/internal/tui/golden_overflow_test.go` (new)

Tests at 80x30 (default) with large dataset:
- `fullapp-items-overflow.golden` — items list with 50+ items (verifies scroll indicators, truncation)
- `fullapp-detail-overflow.golden` — detail for an item with very long description
- `component-sidebar-overflow.golden` — sidebar with many categories (unlikely to overflow, but verify)
- `component-items-overflow.golden` — items list alone with 50+ items

Tests at 60x20 (minimum) with large dataset:
- `fullapp-items-overflow-60x20.golden` — verifies nothing breaks at tiny terminal with lots of data

### Task 2.3: Add empty-state golden tests

**File:** `cli/internal/tui/golden_overflow_test.go` (same file)

```go
// testCatalogEmpty creates a catalog with no items (empty registry)
func testCatalogEmpty(t *testing.T) *catalog.Catalog
```

- `fullapp-items-empty.golden` — items list with 0 items (empty state message)
- `fullapp-detail-empty.golden` — what happens if we somehow navigate to detail with no item?

### Task 2.4: Add scroll behavior tests (non-golden, behavioral)

**File:** `cli/internal/tui/scroll_test.go` (new)

Programmatic tests (not golden files) that verify:
- After pressing Down 50 times with 50 items, cursor is at last item (not beyond)
- After pressing Up at cursor=0, cursor stays at 0
- Page Up/Down moves by visible rows
- Mouse wheel scrolls content (if testable without real terminal)
- Switching tabs resets scroll to 0
- Navigating away and back preserves or resets scroll as appropriate

**Verification:** `make test` passes with new tests.

---

## Phase 3: Strengthen Documentation

**Why:** Documentation is the first thing a session reads. It must be scannable, example-heavy, and positively framed.

### Task 3.1: Rewrite tui/CLAUDE.md with pre-flight and anti-patterns

Restructure the existing `cli/internal/tui/CLAUDE.md` (currently 197 lines) with:

**New structure:**
1. **Before You Edit** (top of file, first thing read)
   - 5-step numbered checklist: read styles.go palette, check keys.go for existing bindings, use shared helpers (renderBreadcrumb, renderStatusMsg, renderHelpBar), run golden tests after changes, test with large dataset
2. **Architecture** (existing, minor updates)
3. **Shared Helpers** (new section documenting the helpers from Phase 1)
4. **Common Mistakes** (replaces "NEVER" section)
   - Side-by-side wrong/right code examples
   - Positive framing: "Colors go in styles.go — here's how to add one" with code
   - "Status messages use message + messageIsErr fields — here's the pattern" with code
5. **Component Patterns** (existing sections, consolidated)
6. **Testing Requirements** (strengthened)
   - Must run golden tests after visual changes
   - Must verify at 60x20 minimum size
   - Must consider: what happens with 50+ items? Empty state? Long text?

**Accessibility note:** Add a brief section documenting that `NO_COLOR=1` is supported automatically via the Charm stack. All status indicators use text+symbol alongside color (e.g., "Done: ..." with green, "Error: ..." with red) so meaning is never color-only.

**Tone:** Conversational, positive framing, concrete examples. No ALL CAPS. No "NEVER" — instead show the wrong way and explain why the right way is better.

### Task 3.2: Update existing path-scoped rules

Update the 5 existing `.claude/rules/` files to:
- Reference the new shared helpers
- Use positive framing (remove MUST/NEVER caps where present)
- Add code examples where currently prose-only

Files to update:
- `tui-styles-gate.md` — add example of adding a new color properly
- `tui-modal-patterns.md` — reference renderButtons, add wrong/right examples
- `tui-keyboard-bindings.md` — clarify the msg.Type exception more clearly
- `tui-test-patterns.md` — add boundary-condition testing requirements
- `go-error-handling.md` — add reference to message/messageIsErr pattern

### Task 3.3: Add new path-scoped rules

Create new rule files in `.claude/rules/`:

**`tui-page-pattern.md`** (paths: `cli/internal/tui/*.go`, excludes test files)
```yaml
paths:
  - "cli/internal/tui/sidebar.go"
  - "cli/internal/tui/items.go"
  - "cli/internal/tui/detail.go"
  - "cli/internal/tui/detail_render.go"
  - "cli/internal/tui/registries.go"
  - "cli/internal/tui/settings.go"
  - "cli/internal/tui/sandbox_settings.go"
  - "cli/internal/tui/filebrowser.go"
  - "cli/internal/tui/loadout_create.go"
  - "cli/internal/tui/import.go"
```
Contents: Every page model follows the standard pattern — use renderBreadcrumb for navigation, renderStatusMsg for messages, renderHelpBar for footer, message+messageIsErr fields, cursor pattern with "> " prefix. Shows the canonical page structure as a code template.

**`tui-boundary-testing.md`** (paths: `cli/internal/tui/*_test.go`)
Contents: When adding or modifying a TUI component, add tests for boundary conditions: 50+ items (scroll/overflow), empty state (0 items), long text (200+ char names/descriptions), minimum terminal size (60x20). Reference testCatalogLarge helper.

---

## Phase 4: Enforcement Hook

**Why:** Documentation and rules are passive — they load into context but don't block. A hook actively prevents non-compliant edits.

### Task 4.1: Create TUI convention enforcement hook

**File:** `.claude/hooks/tui-convention-check.sh` (new)

A PreToolUse hook that fires on Edit and Write operations targeting `cli/internal/tui/*.go` files.

The hook should:
1. Check that the edit doesn't introduce inline color definitions (lipgloss.NewStyle() with hardcoded colors outside styles.go)
2. Check that the edit doesn't introduce inline key string comparisons (hardcoded key checks outside the allowed msg.Type exceptions)
3. Check that the edit doesn't add emoji characters to UI output
4. Validate JSON input from stdin (gracefully handle malformed or missing input instead of crashing)
5. Return a non-zero exit code with a clear message explaining the violation and pointing to the convention

**Hook configuration** (add to `.claude/settings.json`):
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "bash .claude/hooks/tui-convention-check.sh"
          }
        ]
      }
    ]
  }
}
```

The hook receives the tool input as JSON on stdin. It can inspect the `file_path` and `content`/`new_string` fields to check for violations.

### Task 4.2: Test the hook

Manually verify the hook:
- Edit styles.go with a new color → should pass
- Edit items.go with an inline lipgloss.NewStyle().Foreground(...) → should block
- Edit modal.go with a hardcoded `"i"` key check → should block
- Edit a non-TUI file → should not fire at all

---

## Execution Order and Dependencies

```
Phase 1 (code changes)
  ├── Task 1.1: Create pagehelpers.go
  ├── Task 1.2: Standardize method naming
  ├── Task 1.3: Fix known inconsistencies
  ├── Task 1.4: Adopt helpers in all pages
  ├── Task 1.5: Consolidate confirm shortcuts into keys.go
  ├── Task 1.6: Remove redundant inline help
  ├── Task 1.7: Add description boxes
  └── Task 1.8: Centralized toast system
         │
Phase 2 (testing) ←── depends on Phase 1
  ├── Task 2.1: Large test catalog factory
  ├── Task 2.2: Overflow golden tests
  ├── Task 2.3: Empty-state golden tests
  └── Task 2.4: Scroll behavior tests
         │
Phase 3 (documentation) ←── depends on Phase 1 (references final code patterns)
  ├── Task 3.1: Rewrite tui/CLAUDE.md
  ├── Task 3.2: Update existing rules
  └── Task 3.3: Add new rules
         │
Phase 4 (enforcement) ←── depends on Phase 3 (hook messages reference rules)
  ├── Task 4.1: Create enforcement hook
  └── Task 4.2: Test the hook
```

Phases 2 and 3 can run in parallel (both depend on Phase 1 only). Phase 4 depends on Phase 3.

---

## Success Criteria

- All TUI pages use shared helpers (renderBreadcrumb, renderStatusMsg, cursorPrefix, renderScrollUp/Down, renderDescriptionBox)
- All field names standardized: `message` + `messageIsErr` (not messageErr, errMsg, or View params)
- All help text methods named `helpText()` (not renderHelp())
- All confirm shortcuts (y/Y/n/N) defined in keys.go and checked via `key.Matches()`
- No inline keyboard shortcuts in content views (footer only; modals excepted)
- Description boxes on all screens with non-obvious highlighted options (settings, sandbox, library cards, loadout cards, sidebar)
- Description boxes adapt height at small terminal sizes (60x20)
- Toast system handles all status messages centrally; no per-component message rendering
- Error toast sanitizes clipboard content (strips paths, secrets, URLs)
- Toast text re-wraps on terminal resize
- All golden tests pass at all sizes (60x20, 80x30, 120x40, 160x50)
- New overflow/empty golden tests exist and pass
- tui/CLAUDE.md has a scannable pre-flight section at top
- Path-scoped rules load automatically when editing TUI files
- Hook blocks inline color/key/emoji violations with clear error messages
- Hook handles malformed JSON input gracefully
- `make test` passes clean

## Out of Scope (for this plan)

- Refactoring import.go's multi-step wizard structure (complex, low ROI right now)
- Custom `go/analysis` linter (premature — hook + rules cover the same ground with less maintenance; revisit if hook proves insufficient)
- Per-component README files (Go convention is inline docs, not separate files)
- Base page struct embedding (research says this fights Go's design philosophy)
- Exit code propagation from toast errors for non-interactive/CI use (requires broader CLI error handling refactor)
- `NO_COLOR` environment variable handling (already works via Charm stack's built-in support — just document in tui/CLAUDE.md)
- Splitting `cursorPrefix` into separate symbol/style return values (current tuple return is simple enough; overcomplicating for minimal gain)

## Review Provenance

This plan was reviewed by 5 expert agents before finalization:
1. **Go CLI best practices** — toast error propagation, hook JSON validation
2. **Go TUI best practices** — resize-aware wrapping, semi-modal key routing, dynamic description height
3. **Accessibility** — NO_COLOR docs, text+symbol indicators, confirm key consolidation, key queueing
4. **Security** — error message sanitization before clipboard, hook input validation
5. **General Go dev** — mock items for large catalog, exported BreadcrumbSegment, value receivers

Key changes incorporated: error sanitization for clipboard (security), re-wrap on resize (TUI), dynamic description box height (accessibility+TUI), confirm key consolidation (accessibility), hook JSON validation (security), mock items for large catalog (Go dev).
