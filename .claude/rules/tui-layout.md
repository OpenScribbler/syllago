---
paths:
  - "cli/internal/tui/**"
---

# Layout Arithmetic Rules

## 1. Border Subtraction (Always Apply)

Every bordered panel costs 2 chars height (top+bottom) and 2 chars width (left+right).
```go
// WRONG — content overflows the border
contentHeight = panelHeight
contentWidth = panelWidth

// RIGHT
contentHeight = panelHeight - 2  // subtract top+bottom border
contentWidth = panelWidth - 2    // subtract left+right border
```

## 2. Text Truncation Formula

Never rely on terminal auto-wrap. Truncate ALL strings before rendering.
```go
// WRONG — long text wraps, breaks height
style.Render(longTitle)

// RIGHT — explicit truncation
maxTextWidth := panelWidth - 4  // -2 borders, -2 padding
title = truncateString(title, maxTextWidth)
```

## 3. Never Use lipgloss.Width() for Sizing

`Width()` word-wraps (creates multi-line output that breaks height). `MaxWidth()` truncates.
```go
// WRONG — creates multi-line overflow that breaks height
style.Width(w).Render(content)

// RIGHT — truncates without wrapping
style.MaxWidth(w).Render(content)
```

For bordered panels, use `borderedPanel()` from `styles.go` which sets both
Width+MaxWidth and Height+MaxHeight.

## 4. Dynamic Dimensions

Never hardcode height/width offsets. Use rendered string dimensions.
```go
// WRONG — breaks when topbar height changes
contentHeight = termHeight - 5

// RIGHT — adapts to actual rendered size
topbarRendered := a.topbar.View()
contentHeight = termHeight - lipgloss.Height(topbarRendered) - helpbarHeight
```

## 5. Parent Owns Child Sizes

Children never calculate their own size. Parent calls `SetSize()` during `WindowSizeMsg`.
```go
// WRONG — child computes its own dimensions
func (m child) View() string {
    w := termWidth - 20  // where did termWidth come from?

// RIGHT — parent tells child its size
case tea.WindowSizeMsg:
    m.child.SetSize(msg.Width-sidebarWidth, contentHeight)
```
