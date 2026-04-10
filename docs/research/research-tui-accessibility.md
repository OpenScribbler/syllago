# Research: TUI Accessibility (2024-2026)

**Date:** 2026-03-11
**Scope:** Focus management, modal patterns, status regions, form inputs, screen reader compatibility for rich terminal UIs

---

## The Fundamental Challenge

Unlike HTML with semantic elements, terminals are unstructured flat text. Screen readers cannot infer hierarchy, headings, or regions. Users must parse content linearly. You cannot add semantic structure directly (no ARIA equivalent). Must compensate through textual labeling, predictable layout, explicit announcements, and plain-text output modes.

**Source:** Sampath et al., "Accessibility of Command Line Interfaces," CHI 2021

---

## Screen Reader Reality

### Platform Support

| Platform | Terminal | Screen Reader | Support |
|----------|----------|---------------|---------|
| Windows | Windows Terminal | NVDA, Narrator, JAWS | Reliable (UI Automation APIs) |
| macOS | Terminal.app | VoiceOver | Good (NSAccessibility) |
| macOS | iTerm2 | VoiceOver | Partial |
| Linux GUI | GNOME Terminal | Orca | Full (ATK/AT-SPI) |
| Linux console | TTY | Speakup/Fenrir | Works for pure CLI |
| Any | Alacritty, XTerm | Any | Inaccessible (no toolkit accessibility) |

### What Screen Readers Can/Cannot Do

**CAN:** Read text linearly, navigate cursor position, announce bold/underline/color as text, process Unicode, follow arrow keys.

**CANNOT:** Detect visual highlighting/selection, understand spatial positioning, read decorative Unicode meaningfully, detect animations (spinners), understand color differentiation.

---

## Focus Management

### Visual vs. Screen-Reader-Perceivable Focus

These are completely decoupled in terminals. Screen reader users cannot tell which item is "selected" without reading every line.

### Best Practices

**Status Line Approach (Recommended):**
```
Skills > alpha-skill (1/5)
```
Single status line shows context + position. Clear for screen readers, helps all users.

**Text Prefix Approach:**
```
> Selected Item
  Unselected Item
```
Screen reader knows which is selected, but `>` or special characters create noise.

**Best Combination:** ASCII indicator + status line in footer.

### Screen Reader Key Conflicts

Screen readers intercept keystrokes at OS level. Single-letter vim-style shortcuts conflict:
- NVDA: Insert + anything, single letters in browse mode (h, d, t, l, g, f)
- JAWS: Insert + anything, many single-letter shortcuts in virtual cursor mode
- VoiceOver: Ctrl + Option + anything

**Mitigation:** Always support arrow keys + Enter as primary navigation. Single-letter shortcuts are at risk.

---

## Modal Accessibility

### Focus Trapping (ARIA Principles Applied)

1. Focus restoration: when closing, return focus to invoking element
2. Escape key: always closes modal
3. Ctrl+C: always works as escape hatch
4. Tab cycling: Tab/Shift+Tab cycle within modal, not leak outside

### Modal Announcement

Terminals have no mechanism to announce modals. Must include text announcement:
```
[DIALOG] Confirm: Exit without saving?
```

### Checklist
- Modal closable with Escape
- Ctrl+C always works
- Modal presence announced to screen readers (text marker)
- Tab doesn't escape modal
- Shift+Tab implemented for reverse navigation

---

## Status/Error Announcements

### Terminal Equivalent of ARIA Live Regions

Terminals have NO built-in mechanism for live announcements. Workarounds:

**Persistent Status Line (Most viable):**
```
Last action: Installed provider alpha (2 seconds ago)
```
Screen readers read it once per update. Survives terminal redraws.

**Append-Only Message History:**
Messages accumulate; screen reader user can scroll back.

**Blocking Modal for Critical Updates:**
```
[!] Error: Install failed
[Press Enter to acknowledge]
```
Forces user awareness. Used for critical errors/warnings.

---

## Form Input Accessibility

### Best Practices (W3C ARIA Forms + Huh library)

1. **Label-to-Input Association:** Label must be visible and adjacent. Never rely on placeholder as label.
2. **Validation Feedback:** Error below field. Text references the field (not just color).
3. **Required Field Indicators:** `*` or `[required]` prefix visible.
4. **Focus Indicators:** Cursor/border visible when focused.

### Huh Library Pattern
`form.WithAccessible(true)` drops TUI in favor of plain-text prompts. Better for screen readers. Activated via `ACCESSIBLE` environment variable.

---

## Color and Contrast

### WCAG 2.2 Applied to Terminals

| Text Type | Minimum (AA) | Enhanced (AAA) |
|-----------|-------------|----------------|
| Normal | 4.5:1 | 7:1 |
| Large/Bold | 3:1 | 4.5:1 |
| UI Components | 3:1 | -- |

**Terminal Complication:** Cannot guarantee user's background color.

**Mitigation:**
1. Specify both foreground AND background when needing guaranteed contrast
2. Prefer ANSI 4-bit colors (basic 16) -- users control in terminal settings
3. Use `lipgloss.AdaptiveColor` for light/dark theme detection
4. Test on real light and dark backgrounds

### Color Blindness
- 8% of men have red-green color blindness
- Rule: Never use color alone. Always add text labels, symbols, positional cues, bold/underline.

---

## Alternative Output Modes

### Essential Flags

| Flag | Purpose |
|------|---------|
| `NO_COLOR` env var | Disables ANSI colors (standard convention) |
| `--no-color` flag | Explicit override |
| `--json` | Machine-readable output |
| `--plain` | Non-interactive for screen readers |
| `TERM=dumb` detection | Signals minimal terminal |

---

## Motion and Animation

### Problems with Animated Spinners
- Screen readers loop and repeatedly read changing characters
- Vestibular disorders triggered by moving content
- Spinners convey no progress information

**Better alternatives:**
- Static text: `"Loading..."` instead of spinner
- Progress percentage: `"Installing... 45%"`
- Appending dots: `"Loading."` -> `"Loading.."` -> `"Loading..."`

---

## Unicode Handling

**Rules:**
1. Use ASCII or simple Unicode for core UI
2. If Unicode needed, stick to well-supported: circle, checkmark, X, triangle
3. Always pair symbols with text (never symbol alone)
4. Use `runewidth` library for width calculations
5. Avoid emoji in UI (inconsistent width, screen reader noise)

---

## What's Possible vs. Impossible

### Possible
- Text-based status communication
- Keyboard navigation (arrow keys + Tab)
- Plain-text output modes
- Color alternative modes (NO_COLOR)
- Light/dark theme detection (tea.BackgroundColorMsg)
- Focus indication via text (status line)
- Modal dialog patterns with focus trapping

### Partially Possible (Significant Effort)
- Secondary buffer for screen readers (BubbleTea #780, not yet implemented)
- Live region equivalents (append-only history works)
- Semantic structure (must implement entirely via text)

### Impossible (Terminal Limitations)
- OS-level accessibility API integration
- Visual focus detection by screen readers
- Screen reader mode detection
- Animated state communication
- Layout through spatial positioning

---

## Practical BubbleTea Patterns

### Respecting Color Profile
```go
if os.Getenv("NO_COLOR") != "" {
    opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
}
```

### Adaptive Colors
```go
primaryColor := lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"}
```

### Complete Adaptive Color (All Profiles)
```go
statusColor := lipgloss.CompleteAdaptiveColor{
    Light: lipgloss.CompleteColor{TrueColor: "#2E7D32", ANSI256: "34", ANSI: "2"},
    Dark:  lipgloss.CompleteColor{TrueColor: "#81C784", ANSI256: "114", ANSI: "10"},
}
```

---

## Sources
1. Sampath et al., "Accessibility of Command Line Interfaces," CHI 2021
2. WCAG 2.2 -- https://www.w3.org/WAI/WCAG22/
3. W3C ARIA Authoring Practices Guide -- https://www.w3.org/WAI/ARIA/apg/
4. NO_COLOR Convention -- https://no-color.org/
5. BubbleTea Issue #780 -- Screen Reader Accessibility
6. Huh Forms Library -- https://github.com/charmbracelet/huh
7. Lipgloss Documentation -- https://pkg.go.dev/github.com/charmbracelet/lipgloss
8. Seirdy, "Best practices for inclusive CLIs"
9. Blind Computing: State of Linux CLI Accessibility
10. Deque Screen Reader Survival Guide
11. Martin Krzywinski: Color Blindness Palettes
12. Existing syllago docs: docs/reviews/research-accessibility.md, docs/reviews/review-accessibility.md
