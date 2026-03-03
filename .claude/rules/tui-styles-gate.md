---
paths:
  - "cli/internal/tui/styles.go"
---

# Style Definition Gate

This file is the single source of truth for all TUI colors and styles.

## Rules

- Every color MUST use `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}` for light/dark theme support
- New colors go in the color variable block at the top (lines 6-19), grouped with related colors
- New styles go in the style variable block below colors, using the named color variables
- Never use raw hex colors in style definitions — always reference a named color variable
- If an existing color covers your need, reuse it. Check the palette before adding:
  - primaryColor (mint) — titles, labels
  - accentColor (viola) — selection, active elements, buttons
  - mutedColor (stone) — help text, inactive
  - successColor (green) — installed, success
  - dangerColor (red) — errors
  - warningColor (amber) — warnings, global
- Style names use camelCase with a `Style` suffix (e.g., `buttonStyle`, `labelStyle`)
- No emojis — use colored symbols: ✓ ✗ ▸ > ─ ← ⚠
