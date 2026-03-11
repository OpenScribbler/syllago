---
paths:
  - "cli/internal/tui/styles.go"
---

# Style Definition Gate

This file is the single source of truth for all TUI colors and styles.

## Adding a New Color

Colors use `lipgloss.AdaptiveColor` for automatic light/dark theme support:

```go
// Add to the color variable block at the top, grouped with related colors
var myNewColor = lipgloss.AdaptiveColor{Light: "#hexLight", Dark: "#hexDark"}

// Then create a named style that uses it
var myNewStyle = lipgloss.NewStyle().Foreground(myNewColor)
```

## Existing Palette

Check these before adding a new color — reuse when possible:
- `primaryColor` (mint) — titles, labels
- `accentColor` (viola) — selection, active elements, buttons
- `mutedColor` (stone) — help text, inactive
- `successColor` (green) — installed, success
- `dangerColor` (red) — errors
- `warningColor` (amber) — warnings, global

## Conventions

- Never use raw hex colors in style definitions — always reference a named color variable
- Style names use camelCase with a `Style` suffix (e.g., `buttonStyle`, `labelStyle`)
- No emojis — use colored symbols (checkmark, X, >, dash, arrow, warning triangle)
