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

**Critical: Modal mouse handling lives in `app.go`, NOT in the modal's own `Update()` method.**

Inner `zone.Mark()` calls inside a modal's `View()` do NOT survive `overlay.Composite()` — only the outer `zone.Mark("modal-zone", m.View())` wrapper survives. This means `zone.Get("modal-opt-0").InBounds(msg)` will always fail for modal content. Instead, app.go uses coordinate-based hit testing:

```go
z := zone.Get("modal-zone")
if z.InBounds(msg) {
    relX, relY := z.Pos(msg)
    // Hit test using relative coordinates within the modal
    // Options start at relY=4: border(1)+padding(1)+title(1)+blank(1)
    // Button row: modalH - 3 (bottom padding(1)+border(1)+row itself)
}
```

**Rendering side (modal's `View()`):** Still use `zone.Mark()` for semantic clarity and potential future use, but they are NOT used for hit testing:
- Buttons: `zone.Mark("modal-btn-left", ...)` / `zone.Mark("modal-btn-right", ...)` via `renderButtons()`
- Options: `zone.Mark(fmt.Sprintf("modal-opt-%d", i), row)`
- Form fields: `zone.Mark("modal-field-{name}", input.View())`

**Hit testing side (app.go mouse handler):** Coordinate math within `"modal-zone"` bounds:
- Click outside `modal-zone` → dismiss the modal
- Button row → `relY == modalH - 3`, left/right half determines which button
- Option rows → `relY` mapped to option index (2 rows per option for name+desc)
- Text inputs → `relY` mapped to specific field, then `.Focus()` the input

**Form fields:** Any modal with multiple Tab-navigable inputs MUST support click-to-focus. Add coordinate-based hit testing in app.go's mouse handler alongside the zone marks in View():
```go
// In app.go mouse handler for the modal:
if relY == 4 { // URL field row
    m.focusedField = 0
    m.urlInput.Focus()
    m.nameInput.Blur()
}
if relY == 5 { // Name field row
    m.focusedField = 1
    m.urlInput.Blur()
    m.nameInput.Focus()
}
```

**When adding a new modal:** Add its mouse handling block to app.go's `tea.MouseMsg` handler (around line 1118), following the existing pattern for the other modals. Do NOT add `case tea.MouseMsg:` to the modal's own `Update()` — it won't work due to the overlay constraint.

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
