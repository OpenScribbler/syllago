---
paths:
  - "cli/internal/tui/**"
---

# Mouse Support Requirements

Every interactive element in the TUI must support both keyboard AND mouse interaction. No keyboard-only or mouse-only elements.

## Zone Marking

All clickable elements use `zone.Mark(id, content)` from bubblezone:

```go
// Cards
zone.Mark(fmt.Sprintf("library-card-%s", ct), cardStyle.Render(inner))

// List items
zone.Mark(fmt.Sprintf("item-%d", i), row)

// Breadcrumb segments
zone.Mark("crumb-home", helpStyle.Render("Home"))

// Tabs
zone.Mark(fmt.Sprintf("tab-%d", i), tabContent)

// Modal buttons
zone.Mark("modal-btn-confirm", buttonStyle.Render("Confirm"))
zone.Mark("modal-btn-cancel", buttonDisabledStyle.Render("Cancel"))
```

## Zone ID Conventions

| Element | Pattern | Example |
|---------|---------|---------|
| Cards | `"{page}-card-{id}"` | `"library-card-Skills"`, `"registry-card-0"` |
| List items | `"item-{index}"` | `"item-0"`, `"item-5"` |
| Tabs | `"tab-{index}"` | `"tab-0"`, `"tab-1"` |
| Breadcrumbs | `"crumb-{name}"` | `"crumb-home"`, `"crumb-category"` |
| Modal buttons | `"modal-btn-{action}"` | `"modal-btn-confirm"`, `"modal-btn-cancel"` |
| Sidebar items | `"sidebar-{index}"` | `"sidebar-0"` |
| Welcome items | `"welcome-{id}"` | `"welcome-library"`, `"welcome-0"` |
| Provider checks | `"prov-check-{index}"` | `"prov-check-0"` |
| Action buttons | `"detail-btn-{action}"` | `"detail-btn-install"` |

## Click Behaviors

- **Cards:** Click selects and drills into the card (same as Enter)
- **List items:** Click selects; can also drill into on click
- **Breadcrumbs:** Click navigates to that level
- **Tabs:** Click switches to that tab
- **Modal buttons:** Click activates the button action
- **Modal background (outside modal-zone):** Click dismisses the modal
- **Checkboxes:** Click toggles the checkbox
- **Sidebar items:** Click navigates to that section

## Scroll Wheel

- Mouse wheel scrolls the focused component
- Works on: item lists, detail content, card grids, sidebar
- Scroll direction: wheel up = scroll up, wheel down = scroll down
