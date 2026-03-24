---
paths:
  - "cli/internal/tui_v1/**"
---

# Action Button Pattern

Action buttons are chip-style clickable buttons that appear below the breadcrumb on content pages. They provide context-specific actions like "Create", "Remove", or "Sync".

## Struct Definition

```go
type ActionButton struct {
    Hotkey string         // single character, e.g. "a"
    Label  string         // display text after hotkey, e.g. "Create Loadout"
    ZoneID string         // zone.Mark ID for click handling, e.g. "action-a"
    Style  lipgloss.Style // semantic background color
}
```

## Rendering

```go
func renderActionButtons(buttons ...ActionButton) string
```

Renders a single-line row of buttons formatted as `"[key] Label"` with semantic background colors. Buttons are separated by single spaces and surrounded by blank lines.

**Returns:** Empty string if no buttons are provided. Otherwise, a newline-prefixed, newline-suffixed button row.

## Available Semantic Styles

Define action buttons using these semantic styles from `styles.go`:

| Style | Use Case | Color |
|-------|----------|-------|
| `actionBtnAddStyle` | Add/create operations | Green |
| `actionBtnRemoveStyle` | Remove/delete operations | Red |
| `actionBtnUninstallStyle` | Uninstall operations | Orange |
| `actionBtnSyncStyle` | Sync/refresh operations | Viola/Purple |
| `actionBtnDefaultStyle` | Neutral operations | Gray |

All styles include horizontal padding (0, 1) for chip appearance.

## Zone ID Convention

Zone IDs follow the pattern `"action-{hotkey}"` for consistent click handling:

```go
buttons := []ActionButton{
    {
        Hotkey: "a",
        Label:  "Add Loadout",
        ZoneID: "action-a",
        Style:  actionBtnAddStyle,
    },
}
```

## Page Integration

Action buttons appear:
1. After the breadcrumb trail
2. Before the main content area
3. On a single line with blank spacing above and below

## Example Usage

```go
func (m detailModel) View() string {
    s := renderBreadcrumb(
        BreadcrumbSegment{"Home", "crumb-home"},
        BreadcrumbSegment{"Skills", ""},
    )
    s += "\n"

    // Add action buttons
    buttons := []ActionButton{
        {
            Hotkey: "i",
            Label:  "Install",
            ZoneID: "action-i",
            Style:  actionBtnAddStyle,
        },
        {
            Hotkey: "c",
            Label:  "Copy to clipboard",
            ZoneID: "action-c",
            Style:  actionBtnDefaultStyle,
        },
    }
    s += renderActionButtons(buttons...)

    s += "Content goes here...\n"
    return s
}
```

## Mouse and Keyboard

- **Keyboard:** Pressing the hotkey (e.g. `i`, `a`, `r`) triggers the action. Register these via `key.Binding` in `keys.go`.
- **Mouse:** Click any button to trigger its action. Zone ID must be handled in the component's `Update()` method when `tea.MouseMsg` is received.

## Notes

- Hotkey must be a single lowercase character
- All buttons must fit on one line at the minimum terminal width (60 chars)
- Use `actionBtnDefaultStyle` for operations that don't fit semantic categories (neutral, miscellaneous)
- Never inline button styling — always use semantic styles from `styles.go`
