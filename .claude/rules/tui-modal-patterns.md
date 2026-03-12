---
paths:
  - "cli/internal/tui/**"
---

# Modal Construction Rules

All modals follow consistent structure for visual and behavioral uniformity.

## Structural Requirements

Every modal type MUST:
1. Have an `active bool` field and guard methods with `if !m.active { return }`
2. Implement `View() string` using `modalBorderColor`, `modalBgColor`, `lipgloss.RoundedBorder()`, `Padding(1, 2)`
3. Implement `overlayView(background string) string` using `overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)`
4. Implement `Update(tea.Msg) (T, tea.Cmd)` handling keyboard AND mouse input

## Dimensions

- **All modals use `modalWidth = 56`** — one standard size, no exceptions
- Use fixed height when buttons are pinned to bottom (prevents jitter between steps)
- Inner height = modalHeight - 2 (top + bottom padding)
- Maximum modal height: terminal height - 2 (must not overflow at 60x20 minimum)

## Buttons

ALL modals with confirm/cancel actions use `renderButtons()` — no inline help text like `"[Enter] Save [Esc] Cancel"`:

```go
// Right — use renderButtons for all action pairs
buttons := renderButtons("Cancel", "Confirm", m.buttonCursor, innerWidth)

// Wrong — inline help text instead of styled buttons
content += helpStyle.Render("[Enter] Save   [Esc] Cancel")
```

Pin buttons to bottom using spacer calculation:
```go
contentLines := strings.Count(content, "\n")
spacer := innerHeight - contentLines - 1
```

Default cursor: 1 (Cancel) for destructive actions, 0 (Confirm) for safe actions.

## Mouse Support

- Both buttons wrapped in `zone.Mark()` for click support:
  ```go
  zone.Mark("modal-btn-confirm", buttonStyle.Render("Confirm"))
  zone.Mark("modal-btn-cancel", buttonDisabledStyle.Render("Cancel"))
  ```
- Click outside `modal-zone` dismisses the modal
- Clickable options (radio items in Install/Env modals) respond to click
- **Form fields:** Any modal with multiple Tab-navigable inputs MUST wrap each input in `zone.Mark("modal-field-{name}", ...)`. Clicking a field focuses it — the same result as Tab-navigating to it. This is required, not optional.
  ```go
  zone.Mark("modal-field-url", inputStyle.Render(m.urlInput.View()))
  zone.Mark("modal-field-name", inputStyle.Render(m.nameInput.View()))
  ```

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
