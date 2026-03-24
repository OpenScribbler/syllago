package tui

import "strings"

// dropdownMenu is a generic dropdown list with cursor navigation.
type dropdownMenu struct {
	items  []string // display labels
	cursor int      // currently highlighted item
	open   bool
}

// newDropdownMenu creates a dropdown with the given items.
func newDropdownMenu(items []string) dropdownMenu {
	return dropdownMenu{
		items:  items,
		cursor: 0,
	}
}

// toggle flips the dropdown open/closed state.
func (d *dropdownMenu) toggle() {
	d.open = !d.open
}

// close closes the dropdown.
func (d *dropdownMenu) close() {
	d.open = false
}

// moveUp moves the cursor up, clamping at 0.
func (d *dropdownMenu) moveUp() {
	if d.cursor > 0 {
		d.cursor--
	}
}

// moveDown moves the cursor down, clamping at the last item.
func (d *dropdownMenu) moveDown() {
	if d.cursor < len(d.items)-1 {
		d.cursor++
	}
}

// selected returns the currently highlighted item label.
func (d *dropdownMenu) selected() string {
	if len(d.items) == 0 {
		return ""
	}
	return d.items[d.cursor]
}

// View renders the open dropdown as a bordered box with cursor indicator.
// Returns empty string when closed.
func (d dropdownMenu) View() string {
	if !d.open || len(d.items) == 0 {
		return ""
	}

	var lines []string
	for i, item := range d.items {
		if i == d.cursor {
			lines = append(lines, dropdownSelectedStyle.Render("> "+item))
		} else {
			lines = append(lines, dropdownItemStyle.Render("  "+item))
		}
	}

	content := strings.Join(lines, "\n")
	return dropdownBorderStyle.Render(content)
}
