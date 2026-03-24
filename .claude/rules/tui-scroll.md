---
paths:
  - "cli/internal/tui_v1/**"
---

# Scroll Requirements

Any area where content can exceed the visible viewport MUST implement scrolling. Never let content silently disappear off the bottom of the screen.

## Scrollable Areas

These areas require scroll support:
- Item lists (items.go) — implemented
- Detail content (detail.go) — implemented
- Card grids (Homepage, Library, Loadouts, Registries) — all must scroll when cards overflow
- Sidebar (sidebar.go) — not yet implemented
- Help overlay (help_overlay.go) — must scroll on small terminals
- Settings (settings.go) — must scroll if fields exceed viewport
- Toast messages — implemented (5 visible lines). Both error toasts and long success toasts (e.g., bulk operations with warnings) scroll with Up/Down.

## Implementation Checklist

Every scrollable component MUST:

1. **Track state:** `scrollOffset int` field on the model
2. **Clamp bounds:** Implement `clampScroll()` to prevent scrolling past content
3. **Show indicators:** Use `renderScrollUp(count, isContentView)` / `renderScrollDown(count, isContentView)` from pagehelpers.go
4. **Keyboard support:**
   - Up/Down: line-by-line
   - PgUp/PgDown: page jump (viewport height minus overlap)
   - Home/End: first/last item
5. **Mouse support:** Scroll wheel moves viewport
6. **Reset on navigation:** Set `scrollOffset = 0` when navigating away, switching tabs, or changing context
7. **Cursor visibility:** When cursor moves, auto-scroll to keep it in view

## Scroll Indicator Helpers

```go
// For lists (items, sidebar, cards)
renderScrollUp(hiddenAbove, false)   // "(N more above)"
renderScrollDown(hiddenBelow, false) // "(N more below)"

// For content views (detail text, toast)
renderScrollUp(linesAbove, true)     // "(N lines above)"
renderScrollDown(linesBelow, true)   // "(N lines below)"
```
