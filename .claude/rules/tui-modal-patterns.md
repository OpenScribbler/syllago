---
paths:
  - "cli/internal/tui/modal.go"
---

# Modal Construction Rules

All modals follow consistent structure for visual and behavioral uniformity.

## Structural Requirements

Every modal type should:
1. Have an `active bool` field and guard methods with `if !m.active { return }`
2. Implement `View() string` using `modalBorderColor`, `modalBgColor`, `lipgloss.RoundedBorder()`, `Padding(1, 2)`
3. Implement `overlayView(background string) string` using `overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)`
4. Implement `Update(tea.Msg) (T, tea.Cmd)` handling Enter (confirm), Esc (cancel/back), arrow keys

## Dimensions

- Simple dialogs (confirm, save): `modalWidth = 40`
- Complex wizards (install, env setup): `modalWidth = 56`
- Use fixed height when buttons are pinned to bottom (prevents jitter between steps)
- Inner height = modalHeight - 2 (top + bottom padding)

## Buttons

Use `renderButtons(left, right, cursor, contentWidth)` for all two-button footers:

```go
// Right way — use the shared helper
buttons := renderButtons("Cancel", "Confirm", m.buttonCursor, innerWidth)

// Wrong way — don't build button rendering inline
cancelBtn := "  Cancel"
confirmBtn := "▸ Confirm"  // Don't do this
```

Pin buttons to bottom using spacer calculation:
```go
contentLines := strings.Count(content, "\n")
spacer := innerHeight - contentLines - 1
```

Default cursor: 1 (Cancel) for destructive actions, 0 (Confirm) for safe actions.

## Keyboard Behavior

- `Enter` acts on current button cursor position
- `Esc` cancels or goes back one step (not dismiss from middle of wizard)
- `Left/Right` switch buttons, `Up/Down` navigate options
- Confirm modal: `key.Matches(msg, keys.ConfirmYes)` / `keys.ConfirmNo` (y/Y and n/N)

## Multi-Step Wizards

- Step tracking via typed enum (e.g., `installStep`, `envSetupStep`)
- Show progress: `"(N of M)"` when iterating items
- Same fixed dimensions across all steps
- Esc on first step dismisses; Esc on later steps goes back
