---
paths:
  - "cli/internal/tui_v1/**"
---

# Card Grid Pattern

Cards are used on Homepage, Library, Loadouts, and Registries pages. All card grids follow the same layout, style, and interaction rules.

**Note:** Homepage is structurally different — it has multiple card sections (Content, Collections, Configuration) with section headers between them, rather than one continuous grid. This affects cursor navigation and scroll behavior. The rules below apply to each section individually.

## Layout

- Two columns when `contentW >= 42`, single column below
- Card width: `(contentW - 5) / 2` (two-col) or `contentW - 2` (single-col)
- Minimum card width: 18
- **Fixed height in two-column mode** (set via `cardStyle.Height()`), dynamic in single-column. Fixed height is required so that bubblezone `zone.Mark()` click regions align correctly — variable-height cards in the same row cause click targets to shift or overlap.
- Gap: 1 character between columns (via `lipgloss.JoinHorizontal` with `" "` spacer)

## Styles

Use `cardNormalStyle` / `cardSelectedStyle` from styles.go for ALL card rendering:

```go
// Right — use shared styles
style := cardNormalStyle.Width(cardW)
if selected {
    style = cardSelectedStyle.Width(cardW)
}

// Wrong — don't build card styles inline
style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor)
```

Card text:
- Titles: `labelStyle`, truncated to `cardWidth - 4`
- Descriptions: `helpStyle`, truncated to `cardWidth - 4`
- Counts: `countStyle` in parentheses, e.g. `"(5)"`
- Status indicators: `installedStyle` / `helpStyle` for present/missing

## Keyboard (when content is focused via Tab)

- Up/Down: move cursor by column count (2 in two-col, 1 in single-col)
- Left/Right: move cursor by 1
- Enter: drill into the selected card
- Home/End: jump to first/last card
- Cursor clamps to `[0, len(cards)-1]` on every movement and resize

## Mouse

- Every card wrapped in `zone.Mark(id, card)` for click handling
- Click selects and drills into the card

## Breadcrumb

- Every card page except Homepage renders `renderBreadcrumb()` at top
- Pattern: `Home > [Page Name]` (e.g., `Home > Library`, `Home > Registries`)

## Tab Focus

- Tab toggles between sidebar and card content on ALL card pages — no exceptions
- When content is focused, card cursor is visible and arrow-navigable

## Scroll

- Card pages with more cards than fit on screen MUST scroll
- Track `scrollOffset` and show `renderScrollUp()` / `renderScrollDown()` indicators
- Support PgUp/PgDown for page jumps, Home/End for bounds
- Mouse wheel scrolls card grid
