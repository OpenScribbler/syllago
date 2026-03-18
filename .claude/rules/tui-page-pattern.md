---
paths:
  - "cli/internal/tui/**"
---

# TUI Page Patterns

There are two page patterns: **list pages** and **card grid pages**. Every page uses shared helpers from `pagehelpers.go`.

## Page Inventory

| Page | Screen Enum | Pattern | File | Breadcrumb | Tab Focus | Search |
|------|-------------|---------|------|------------|-----------|--------|
| Homepage | `screenCategory` | Card grid | app.go | No (is home) | Yes | Yes |
| Items | `screenItems` | List | items.go | Yes | Yes | Yes |
| Detail | `screenDetail` | Tabbed | detail.go | Yes | Tabs only | Yes (not when text input active) |
| Library | `screenLibraryCards` | Card grid | app.go | Yes | Yes | Yes |
| Loadouts | `screenLoadoutCards` | Card grid | app.go | Yes | Yes | Yes |
| Registries | `screenRegistries` | Card grid | registries.go | Yes | Yes | Yes |
| Import | `screenImport` | Wizard | import.go | Custom | No | No |
| Create Loadout | `screenCreateLoadout` | Wizard | loadout_create.go | Yes | No | No |
| Update | `screenUpdate` | Simple | update.go | Yes | No | No |
| Settings | `screenSettings` | Form | settings.go | Yes | No | No |
| Sandbox | `screenSandbox` | Form | sandbox_settings.go | Yes | No | No |

## List Page Pattern

```go
type fooModel struct {
    cursor       int
    scrollOffset int
    message      string    // set by actions, promoted to toast by App
    messageIsErr bool
    width        int
    height       int
}

func (m fooModel) View() string {
    s := renderBreadcrumb(
        BreadcrumbSegment{"Home", "crumb-home"},
        BreadcrumbSegment{"Foo", ""},
    ) + "\n\n"

    for i, item := range m.visibleItems() {
        prefix, style := cursorPrefix(i == m.cursor)
        s += fmt.Sprintf("  %s%s\n", prefix, style.Render(item.Name))
    }
    return s
}

func (m fooModel) helpText() string {
    return "up/down navigate • enter select • esc back"
}
```

## Card Grid Page Pattern

See `tui-card-grid.md` for full card spec. Card pages use `cardNormalStyle`/`cardSelectedStyle`, dynamic sizing, and `renderBreadcrumb()` (except Homepage).

## Required Elements

Every page MUST have:
1. **Breadcrumb** — via `renderBreadcrumb()` (except Homepage)
2. **Keyboard navigation** — Up/Down minimum, Left/Right for grids
3. **Mouse support** — clickable elements via `zone.Mark()`
4. **helpText()** — context-sensitive help for the footer bar
5. **Scroll support** — when content can exceed viewport (see `tui-scroll.md`)
6. **Toast integration** — messages via `message`/`messageIsErr` fields, promoted by App
7. **Items rebuild** — when refreshing item data, use `a.rebuildItems()` not `newItemsModel()` (see `tui-items-rebuild.md`)
8. **Post-action navigation** — wizard/action completion messages must navigate to the result, not the homepage (see `tui/CLAUDE.md` Post-Action Navigation section)

## Shared Helpers (pagehelpers.go)

- `renderBreadcrumb(segments...)` — clickable navigation trail
- `cursorPrefix(selected bool)` — returns `"> "` / `"  "` with appropriate style
- `renderScrollUp(count, isContentView)` / `renderScrollDown(count, isContentView)`
- `renderDescriptionBox(text, width, maxLines)` — bordered context box
- `renderStatusMsg(msg, isErr)` — toast message styling (used by toast, not pages)
