---
paths:
  - "cli/internal/tui/**"
---

# Responsive Layout Rules

The TUI must work correctly at all terminal sizes from 60x20 (minimum) to 160x50+ (large).

## Breakpoints

| Size | Dimensions | Behavior |
|------|-----------|----------|
| Below minimum | < 60x20 | Show "Terminal too small" warning, skip all rendering |
| Minimum | 60x20 | Single-column cards, compact list fallback, no ASCII art |
| Default | 80x30 | Two-column cards, standard layout |
| Medium | 120x40 | Full card layout, more visible content |
| Large | 160x50 | ASCII art title on homepage (h>=48, contentW>=75) |

## Card Layout Breakpoints

- Two-column: `contentW >= 42`
- Single-column: `contentW < 42`
- Card width: `(contentW - 5) / 2` (two-col) or `contentW - 2` (single-col)
- Min card width: 18
- Homepage cards vs list: cards when `height >= 35`, text list fallback below

## Modal Sizing

- Standard width: 56 for ALL modals
- Modal height must not cause overflow at minimum terminal size (60x20)
- Maximum modal height: `terminal height - 2`

## Text Handling by Size

- Always truncate or word-wrap — never let text auto-wrap via terminal line overflow
- Recalculate available widths on `tea.WindowSizeMsg`
- Card text truncates to `cardWidth - 4` (border + padding)
- Detail content word-wraps to content pane width

## Golden File Testing

Every visual component must be tested at these sizes:
- 60x20 (minimum terminal)
- 80x30 (default — primary golden tests)
- 120x40 (medium)
- 160x50 (large)

Also test with large datasets (85+ items) at each size to verify scroll and truncation behavior.

## Resize Handling

On `tea.WindowSizeMsg`:
- Recalculate all layout dimensions
- Clamp cursor positions to valid range (e.g., 2-col cursor may be invalid after switch to 1-col)
- Clamp scroll offsets to valid range
- Update sub-model width/height fields
