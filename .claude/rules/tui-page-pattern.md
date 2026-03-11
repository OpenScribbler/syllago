---
paths:
  - "cli/internal/tui/sidebar.go"
  - "cli/internal/tui/items.go"
  - "cli/internal/tui/detail.go"
  - "cli/internal/tui/detail_render.go"
  - "cli/internal/tui/registries.go"
  - "cli/internal/tui/settings.go"
  - "cli/internal/tui/sandbox_settings.go"
  - "cli/internal/tui/filebrowser.go"
  - "cli/internal/tui/loadout_create.go"
  - "cli/internal/tui/import.go"
---

# TUI Page Model Pattern

Every page model follows the same structure. Use shared helpers from `pagehelpers.go` instead of reimplementing.

## Canonical Structure

```go
type fooModel struct {
    cursor       int
    scrollOffset int
    message      string    // set by actions, promoted to toast by App
    messageIsErr bool
    width        int
    height       int
}

func (m fooModel) Update(msg tea.Msg) (fooModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        m.message = ""  // clear on keypress
        switch {
        case key.Matches(msg, keys.Up):
            if m.cursor > 0 { m.cursor-- }
        case key.Matches(msg, keys.Down):
            if m.cursor < m.itemCount()-1 { m.cursor++ }
        }
    }
    return m, nil
}

func (m fooModel) View() string {
    s := renderBreadcrumb(
        BreadcrumbSegment{"Home", "crumb-home"},
        BreadcrumbSegment{"Foo", ""},
    ) + "\n\n"

    for i, item := range m.items {
        prefix, style := cursorPrefix(i == m.cursor)
        s += fmt.Sprintf("  %s%s\n", prefix, style.Render(item.Name))
    }

    // Don't render messages inline — App handles toast rendering
    return s
}

func (m fooModel) helpText() string {
    return "up/down navigate • enter select • esc back"
}
```

## Key Points

- Use `cursorPrefix()` for selection indicators — produces "> " / "  "
- Use `renderBreadcrumb()` for navigation headers
- Use `renderScrollUp/Down()` for scroll indicators
- Messages go to toast via promotion — don't call `renderStatusMsg()` in View
- Provide `helpText()` for the App footer bar
