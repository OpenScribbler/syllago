---
paths:
  - "cli/internal/tui/modal.go"
---

# Modal Construction Rules

All modals in this file must follow consistent structure for visual and behavioral uniformity.

## Structural Requirements

Every modal type MUST:
1. Have an `active bool` field and guard all methods with `if !m.active { return }`
2. Implement `View() string` with a `lipgloss.NewStyle()` using `modalBorderColor`, `modalBgColor`, `lipgloss.RoundedBorder()`, and `Padding(1, 2)`
3. Implement `overlayView(background string) string` using `overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)`
4. Implement `Update(tea.Msg) (T, tea.Cmd)` handling at minimum: Enter (confirm), Esc (cancel/back), arrow keys (navigation)

## Dimensions

- Simple dialogs (confirm, save): `modalWidth = 40`
- Complex wizards (install, env setup): `modalWidth = 56`
- Use fixed height when modal has buttons pinned to bottom (prevents jitter between steps)
- Inner height = modalHeight - 2 (accounts for top + bottom padding)

## Buttons

- Use `renderButtons(left, right, cursor, contentWidth)` for ALL two-button footers
- Pin buttons to bottom using spacer calculation:
  ```go
  contentLines := strings.Count(content, "\n")
  spacer := innerHeight - contentLines - 1
  ```
- Active button gets `buttonStyle` (white on viola) with `▸ ` prefix
- Inactive button gets `buttonDisabledStyle` (muted on gray) with `  ` prefix
- Default cursor position: 1 (Cancel) for destructive actions, 0 (Confirm) for safe actions

## Keyboard Behavior

- `Enter` acts on current button cursor position
- `Esc` cancels or goes back one step (never dismisses from middle of wizard)
- `Left/Right` switch between buttons
- `Up/Down` navigate options within modal content
- Confirm modal also accepts `y/Y` (confirm) and `n/N` (cancel)

## Multi-Step Wizards

- Step tracking via typed enum (e.g., `installStep`, `envSetupStep`)
- Show progress: `"(N of M)"` when iterating through items
- Same fixed dimensions across all steps
- Esc on first step dismisses; Esc on later steps goes back
