# Research: CLI Accessibility (2024-2026)

**Date:** 2026-03-11
**Scope:** Screen reader compatibility, NO_COLOR, keyboard nav, color-independent meaning, cognitive accessibility for command-line tools

---

## Key Principles

### 1. Keyboard Operability
- **WCAG 2.1.1 (Keyboard):** All functionality operable via keyboard without timing requirements
- **WCAG 2.4.7 (Focus Visible):** Keyboard focus must always be visible
- CLIs are inherently keyboard-native (natural strength over GUIs)
- **Common failure:** Keyboard traps where users cannot escape components

### 2. Color and Contrast
- **WCAG 1.4.3 (Contrast Minimum):**
  - Standard text: 4.5:1 ratio minimum (Level AA)
  - Large text (18pt+): 3:1 minimum
  - No rounding: 4.499:1 does NOT meet 4.5:1
- **WCAG 1.4.1 (Use of Color):** Color must never be the ONLY means of conveying information
  - Supplement with text labels, icons, patterns, or contrast
  - Color blindness affects ~8% of males, ~0.5% of females

### 3. NO_COLOR Standard
- When `NO_COLOR` env var is present (regardless of value), disable ANSI color output
- Supports: color vision deficiency, low vision, piped output, screen readers
- Check during initialization; allow config/flags to override

### 4. Semantic Structure
- **WCAG 1.3.1 (Info and Relationships):** Information through presentation must be available in text
- Visual structure (indentation, spacing) must be meaningful to sequential readers
- Consistent formatting, clear headings, proper hierarchy

### 5. Error Communication
- Errors perceivable WITHOUT color alone
- Display near source of problem
- Use high-contrast styling + icons + text (never color only)
- Describe specifically what went wrong in plain language
- Offer constructive solutions
- Preserve user input for editing
- Use prefixes (ERROR:, WARNING:) in addition to color

### 6. Cognitive Accessibility
- **Memory:** Keep messages concise; don't overwhelm
- **Reading:** Use literal language (avoid idioms); ~15-20% experience language difficulties
- **Attention:** Minimize distractions; clear focus
- **Problem-Solving:** Make solutions immediately obvious

---

## Concrete Patterns

### Text-Based Status Indicators
```
checkmark  Succeeded
warning    Warning
X          Error
arrow      Navigation/flow
[ ]        Checkbox unchecked
[x]        Checkbox checked
```
Work across terminals, respect NO_COLOR, perceivable to screen readers.

### Error Message Structure
```
ERROR: [specific problem]
       [constructive solution]

Example:
ERROR: Port 8080 already in use
       Run with --port=8081 or stop the process on 8080
```

### Keyboard Navigation (Huh Library Pattern)
- Arrow keys or j/k: Move between fields
- Tab/Shift+Tab: Navigate between sections
- Enter/Space: Toggle/submit
- Escape: Cancel/exit (always available)
- All documented in help text

### Accessible Mode Fallback
```go
form.WithAccessible(true)  // Replaces styled TUI with standard prompts
```
Users opt-in via config or environment variable.

### Validation Feedback Timing
- Severe errors: show immediately
- Minor errors: delay until blur (field exit)
- Empty fields: validate only on submit
- Once error appears, validate immediately as user corrects

---

## Common Accessibility Failures

1. **Color-Only Status:** Red text for errors without text prefix
2. **No NO_COLOR Support:** Hardcoded ANSI colors
3. **Inadequate Contrast:** Dark gray on black, light gray on white
4. **Insufficient Keyboard Nav:** Mouse-only interactions, no focus indicator, keyboard traps
5. **Unclear Errors:** "Invalid input" without explanation or solution
6. **Format-Only Hierarchy:** Indentation alone to show nesting
7. **Unstructured Output:** Long blocks without separators or headings
8. **Distracting Animations:** Spinners without static fallback
9. **Color-Only Required Fields:** Red asterisk without text marker
10. **Hidden Help:** -h flag not discoverable

---

## Recommendations for Go/BubbleTea

### Terminal Capability Detection
- Detect color support; provide graceful degradation
- Support 16, 256, and 24-bit color via Lip Gloss
- Check `NO_COLOR` on startup

### Keyboard-First Architecture
- Keyboard primary; mouse optional
- Full tab order navigation
- Visible focus indicator on all focusable elements
- Document all shortcuts in help
- Support arrow keys AND vim-style (hjkl)
- Escape always exits modals

### Error and Status Pattern
```go
"SUCCESS: Setup complete"     // Text prefix + symbol
"WARNING: Disk space low"     // Clear prefix
"ERROR: Port 8080 in use\n   Try --port=8081"  // Specific + solution
```

### Information Structure
- Semantic spacing (blank lines) between sections
- Descriptive section headings
- Consistent indentation for nesting
- Wrap at reasonable lengths (60-80 chars)

### Alternative Text Modes
- `--plain` or `--accessible` flags
- Text-only mode when piped: `if !term.IsTerminal(os.Stdout.Fd())`
- Document accessibility features in README

---

## The Three Accessibility Layers

**Layer 1: Perceivable**
- Not relying on color alone (symbols, text prefixes)
- Adequate contrast (4.5:1 minimum)
- Clear, structured text output
- NO_COLOR support

**Layer 2: Operable**
- Full keyboard navigation (no mouse required)
- Visible focus indicator
- No keyboard traps
- Discoverable shortcuts (--help, ?)

**Layer 3: Understandable**
- Clear, plain language
- Predictable behavior
- Descriptive headings and labels
- Errors that explain what happened and how to fix

---

## Sources
1. W3C WAI -- WCAG 2.1 Success Criteria (Keyboard, Contrast, Use of Color, Focus Visible, etc.)
2. no-color.org -- NO_COLOR Standard
3. Nielsen Norman Group -- Keyboard Accessibility, Error Message Guidelines
4. WebAIM -- Visual/Motor/Cognitive Disabilities guides
5. Charmbracelet -- BubbleTea, Huh, Lip Gloss documentation
6. Baymard Institute -- Form validation UX research
7. W3C WAI Methodology -- WCAG-EM
