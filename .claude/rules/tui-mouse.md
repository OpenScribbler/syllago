---
paths:
  - "cli/internal/tui/**"
---

# Mouse Support Requirements

## Mouse Parity Principle

**Every interactive element must support both keyboard AND mouse interaction — no exceptions.**

The rule is simple: if you can reach it with the keyboard, you can click it with the mouse.

- If you can Tab to a form field, you can click to focus it
- If you can press Enter/Space to toggle something, you can click to toggle it
- If you can navigate a list with arrow keys, you can click an item to select it
- If you can move between modal options or steps, you can click between them

This applies to **all** interactive elements: cards, list items, tabs, breadcrumbs, modal buttons, **form input fields**, radio options, checkboxes, and sidebar items. Form fields are the most commonly missed — any modal with multiple Tab-navigable inputs must also support clicking to focus each field.

## Zone Marking

All clickable elements use `zone.Mark(id, content)` from bubblezone.

**Critical:** In two-column card grids, cards MUST use fixed height (`cardStyle.Height()`) so that `zone.Mark()` regions align correctly within each row. Variable-height cards cause click targets to shift because bubblezone calculates zones from rendered string positions. Single-column grids don't need this since cards stack vertically.

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
| Modal form fields | `"modal-field-{name}"` | `"modal-field-url"`, `"modal-field-name"` |
| Radio/option items | `"modal-opt-{index}"` | `"modal-opt-0"`, `"modal-opt-1"` |
| Sidebar items | `"sidebar-{index}"` | `"sidebar-0"` |
| Welcome items | `"welcome-{id}"` | `"welcome-library"`, `"welcome-0"` |
| Provider checks | `"prov-check-{index}"` | `"prov-check-0"` |
| Action buttons | `"detail-btn-{action}"` | `"detail-btn-install"` |

## Click Behaviors

- **Cards:** Click selects and drills into the card (same as Enter)
- **List items:** Click selects and drills in (same as Enter)
- **Breadcrumbs:** Click navigates to that level
- **Tabs:** Click switches to that tab
- **Modal buttons:** Click activates the button action
- **Modal form fields:** Click focuses that field (same as Tab to it) — required for any modal with multiple inputs
- **Radio/option items:** Click selects that option (same as Up/Down + Enter)
- **Modal background (outside modal-zone):** Click dismisses the modal
- **Checkboxes:** Click toggles the checkbox
- **Sidebar items:** Click navigates to that section

## Scroll Wheel

- Mouse wheel scrolls the focused component
- Works on: item lists, detail content, card grids, sidebar
- Scroll direction: wheel up = scroll up, wheel down = scroll down
