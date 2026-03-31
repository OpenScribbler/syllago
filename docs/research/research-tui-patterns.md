# Research: TUI Patterns & Best Practices (2024-2026)

**Date:** 2026-03-11
**Scope:** BubbleTea/Charm ecosystem, modal patterns, error recovery, status/notification patterns, card grids, async UX, navigation

---

## 1. Modal Patterns & Best Practices

### 1.1 Modal Architecture (BubbleTea + Overlay Pattern)
Modals are centered overlays rendered on top of background content using `bubbletea-overlay` + `bubblezone` for click handling.

**Structure:**
```
Modal = Border + Padding + Fixed Content + Button Bar

- active bool          (guards all methods)
- Update(msg)          (handles keyboard/mouse)
- View()               (renders modal box)
- overlayView(bg)      (composites on background)
```

**Key Libraries:**
- `github.com/charmbracelet/bubbletea` -- core framework
- `github.com/rmhubbert/bubbletea-overlay` -- overlay positioning (Center, Center)
- `github.com/lrstanley/bubblezone` -- clickable regions with `zone.Mark(id, content)`
- `github.com/charmbracelet/lipgloss` -- border + styling

### 1.2 Confirmation Dialog (Simple Modal)
- Width: 40 characters (fixed)
- Height: 10 characters (fixed to prevent jitter)
- Padding: 1 top/bottom, 2 left/right
- Keyboard: Enter (acts on cursor), Esc (cancel), Left/Right (switch buttons), y/Y (confirm), n/N (cancel)
- Buttons: Active gets `buttonStyle` with prefix, inactive gets `buttonDisabledStyle`

### 1.3 Multi-Step Wizards
- Track step with typed enum; same fixed dimensions across all steps
- Progress: "(N of M)" on title line
- Esc goes back one step (dismiss only on first step)
- Each step uses same fixed modal dimensions

### 1.4 Smart Modal Redirects
When a user action leads somewhere unexpected, redirect instead of erroring:

1. Detect condition requiring different modal (e.g., missing env vars during install)
2. Open appropriate modal with context pre-filled
3. Modal's final action triggers the original operation
4. If user cancels, original operation is abandoned gracefully

### 1.5 Multi-Field Input Modal
- Use `textinput.Model` from bubbles
- Tab/Shift+Tab to switch fields
- Focus/Blur to control which field captures keystrokes
- Inline validation on submit, clear errors on next keypress

---

## 2. Error Recovery & Graceful Degradation

### 2.1 Status Message Pattern (Transient Feedback)
- `message string + messageIsErr bool` on model
- Rendered outside scrollable area (always visible)
- Cleared on next keypress (not timer-based)
- Why not timer-based: can be missed by fast users; keypress-based is responsive

### 2.2 Operation-in-Progress States
- Set loading flag when async starts; disable input; show spinner; re-enable on complete
- Use `tea.Batch()` for spinner tick + async command
- Always show adjacent text explaining what's happening

### 2.3 Error Recovery: "Did You Mean?" Patterns
When operation fails, suggest alternatives before asking user to re-enter:
- Invalid path: offer "Create directory" or "Choose different location"
- Auth failed: offer re-enter URL, verify in browser, try SSH
- Permission denied: offer global install location, show permissions

**Implementation:**
```go
// Don't just set error message and return
// Instead, open recovery modal with alternatives
if isPermissionError(err) {
    return m, func() tea.Msg {
        return openPermissionRecoveryModal{}
    }
}
```

### 2.4 Graceful Degradation
- README rendering fails: show raw markdown
- Release notes fetch fails: show git log
- MCP config parsing fails: show error in detail, allow viewing other metadata

---

## 3. Status/Notification Display Patterns

### 3.1 Global Status Bar (App-Level)
- Single status message bar shared across all screens
- Use for: operation completions, cross-screen feedback, async results
- Non-blocking; doesn't require user acknowledgment

### 3.2 Inline Component Messages
- Sub-component messages stay within component bounds
- Cleared on next keypress within that component
- App-level messages override these

### 3.3 Color-Coded Status (No Emoji)
- checkmark -- success, installed (green)
- X mark -- failed, error (red)
- right-pointing triangle -- current selection
- warning sign -- warning, not ready (amber)
- horizontal line -- separator

---

## 4. Card Grid & Layout Patterns

### 4.1 Card Grid Navigation
- Up/Down: move between rows
- Left/Right: move within row
- Page Up/Down: jump multiple rows
- Home/End: first/last card
- Calculate cards per row responsively: `cardsPerRow = (width - padding) / (cardWidth + spacing)`

### 4.2 Text Truncation
Never rely on terminal auto-wrap. Truncate explicitly with ellipsis.

---

## 5. Async Operation UX

### 5.1 Command-Based Pattern
- Commands are pure functions; I/O happens inside
- Messages carry results back to Update()
- `tea.Batch()` for concurrent, `tea.Sequence()` for sequential
- Spinner animation continues during wait

---

## 6. Multi-Screen Navigation

### 6.1 Screen Types
- Track current screen with typed enum
- Sidebar always visible; content changes with screen type
- Tab/Shift+Tab cycle, number keys jump directly

### 6.2 Breadcrumb Navigation
- Clickable with `zone.Mark()`
- Esc goes back one level

### 6.3 Screen State Preservation
- Cache detail model when navigating away
- Restore on return (scroll position, etc.)

---

## 7. Input Validation & Feedback

### 7.1 Timing
- Real-time: visual feedback during typing (character count)
- On-submit: validation errors on confirm attempt
- Stable layout: spacer lines prevent jitter as errors appear/disappear

### 7.2 "Wrong Input Type" Handling
When user provides wrong type, suggest correction:
- Not a number: "Please enter a port number (1-65535)"
- Out of range: show valid range
- Offer common defaults: "Did you mean 8080?"

---

## 8. Patterns from Popular Apps

### lazygit
- Single-key mnemonics for actions
- Confirmation dialog on destructive ops
- Errors shown in toast at bottom; Enter dismisses

### k9s
- Breadcrumb navigation showing hierarchy
- `/` opens regex filter with real-time matching
- `?` shows all shortcuts by context
- `[`/`]` for back/forward history

### Huh (Charm forms library)
- `form.WithAccessible(true)` for screen reader mode
- Dynamic forms with conditional fields
- Multi-select with search filtering

---

## Sources
- BubbleTea: https://github.com/charmbracelet/bubbletea
- Bubbles: https://github.com/charmbracelet/bubbles
- Lipgloss: https://github.com/charmbracelet/lipgloss
- Huh: https://github.com/charmbracelet/huh
- Gum: https://github.com/charmbracelet/gum
- Lazygit: https://github.com/jesseduffield/lazygit
- K9s: https://github.com/derailed/k9s
