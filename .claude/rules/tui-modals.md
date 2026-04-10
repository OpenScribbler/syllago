---
paths:
  - "cli/internal/tui/**"
---

# Modal Patterns

## 1. Standard Modal Structure

- Fixed width: `modalWidth = 56`
- Rounded border in accent color
- Background: `modalBgColor`
- Padding: `Padding(1, 2)`
- Composited via `overlayModal()` — background visible on both sides

## 2. Zone-Safe Borders

Build modal borders manually (`╭─╮│╰─╯`) instead of `lipgloss.Border()`.
Lipgloss dimension styling mangles bubblezone's invisible markers.

```go
// WRONG — lipgloss border breaks zone markers
style.Border(lipgloss.RoundedBorder())

// RIGHT — manual box-drawing characters
top := "╭" + strings.Repeat("─", innerW) + "╮"
row := "│" + content + "│"
bot := "╰" + strings.Repeat("─", innerW) + "╯"
```

## 3. Keyboard Priority

When modal is active, it consumes ALL input except Ctrl+C.
- Tab: cycle focus targets
- Enter: confirm / advance field
- Esc: cancel / go back
- Ctrl+S: save from any field

## 4. Button Rendering

Use `renderButtons(left, right, cursor, contentWidth)` for all two-button footers.
Active: white on accent. Inactive: muted on gray. Buttons pinned to bottom via spacer.

```go
// WRONG — custom button rendering per modal
cancelBtn := style1.Render("Cancel")
saveBtn := style2.Render("Save")

// RIGHT — use shared renderButtons/renderModalButtons
renderModalButtons(focusIdx, usableW, pad, accentLabels, buttons...)
```

## 5. Click-Away Dismissal

Clicks outside `modal-zone` dismiss. Clicks inside do not propagate to background.
