# Dropdown Component Pattern

Based on Soft Serve's `tabs.go` adapted for dropdown behavior. This is the reference implementation pattern for syllago's topbar dropdowns.

## Soft Serve's Actual Code (Simplified)

```go
// Message types
type SelectMsg int   // command: "select item N" (silent, no callback)
type ActiveMsg int   // event: "item N is now active" (fired on key/mouse)

// Model
type Dropdown struct {
    common    Common
    items     []string
    active    int       // committed selection
    cursor    int       // navigation position when open
    isOpen    bool
}

// Update — key and mouse handling
func (d *Dropdown) Update(msg tea.Msg) (Component, tea.Cmd) {
    cmds := make([]tea.Cmd, 0)
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        if !d.isOpen {
            switch msg.String() {
            case "enter", " ":
                d.isOpen = true
                d.cursor = d.active
            }
        } else {
            switch msg.String() {
            case "j", "down":
                d.cursor = (d.cursor + 1) % len(d.items)
            case "k", "up":
                d.cursor = (d.cursor - 1 + len(d.items)) % len(d.items)
            case "enter":
                d.active = d.cursor
                d.isOpen = false
                cmds = append(cmds, d.activeCmd)
            case "esc":
                d.isOpen = false
            }
        }
    case tea.MouseClickMsg:
        if d.isOpen {
            for i, item := range d.items {
                if d.common.Zone.Get(d.zoneID(item)).InBounds(msg) {
                    d.active = i
                    d.isOpen = false
                    cmds = append(cmds, d.activeCmd)
                }
            }
        }
    case SelectMsg:
        if int(msg) >= 0 && int(msg) < len(d.items) {
            d.active = int(msg)
            // NOTE: SelectMsg does NOT fire activeCmd (Soft Serve pattern)
        }
    }
    return d, tea.Batch(cmds...)
}

// View — closed state
func (d *Dropdown) viewClosed() string {
    label := d.items[d.active] + " ▾"
    return d.common.Zone.Mark(d.triggerZone, labelStyle.Render(label))
}

// View — open state (rendered as bordered list)
func (d *Dropdown) viewOpen() string {
    var lines []string
    for i, item := range d.items {
        style := normalItemStyle
        prefix := "  "
        if i == d.cursor {
            style = selectedItemStyle  // full-width background fill
            prefix = "> "
        }
        line := d.common.Zone.Mark(d.zoneID(item), style.Render(prefix+item))
        lines = append(lines, line)
    }
    content := strings.Join(lines, "\n")
    return dropdownBorder.Render(content)
}
```

## Rendering the Open Dropdown

The dropdown list renders directly below the trigger in the topbar. Since the topbar is the first element in the vertical layout, the dropdown simply extends it:

```go
func (t *topBar) View() string {
    bar := t.renderBar()  // the closed topbar row
    if t.openDropdown != nil && t.openDropdown.isOpen {
        dropdown := t.openDropdown.viewOpen()
        return lipgloss.JoinVertical(lipgloss.Left, bar, dropdown)
    }
    return bar
}
```

The content area below adjusts because the topbar's rendered height grows. No overlay compositing needed.

## Key Design Decisions

1. **Dropdown extends the topbar height** — avoids the overlay/compositor problem entirely
2. **When open, dropdown intercepts ALL keys** — same as modal pattern
3. **Parent maintains its own `active` copy** — synced via ActiveMsg, not by reference
4. **Zone-mark each item** for mouse support — item label string as zone ID
5. **SelectMsg is silent** (no callback) — used for programmatic selection
6. **ActiveMsg fires only on user interaction** (key/mouse) — parent catches to update content
