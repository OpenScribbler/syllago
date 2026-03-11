---
paths:
  - "cli/internal/tui/**"
---

# Text Handling Rules

Never rely on terminal auto-wrap. All text must be explicitly truncated or word-wrapped to fit its container.

## When to Truncate

Use truncation for single-line labels in constrained spaces:
- Card titles: truncate to `cardWidth - 4` with `"..."` suffix
- Card descriptions: truncate to `cardWidth - 4` with `"..."`
- List item names: truncate to available column width
- URLs: truncate with `"..."` to fit available width
- Breadcrumb segments: truncate long item names

Use `truncate(text, maxWidth)` helper or manual slice + `"..."`.

## When to Word-Wrap

Use word-wrap for multi-line prose in reading contexts:
- Detail view content (overview tab)
- Modal body text
- Toast messages
- Error messages

Use `wordwrap.String(text, maxWidth)` from `muesli/reflow/wordwrap`.

## Width Calculations

Always subtract border and padding from container width:

```go
// Card inner text width
maxText := cardWidth - 4  // -2 border, -2 padding

// Modal inner text width
maxText := 56 - 6  // -2 border, -4 padding (Padding(1,2) = 2 chars each side)
// = 50 characters

// Content pane width
contentW := a.width - sidebarWidth - 1  // -1 for sidebar border
```

## Truncation Rules

- Suffix: `"..."` (3 characters)
- Minimum display: at least 10 characters before truncating
- Never truncate and word-wrap the same text — pick one strategy per context
