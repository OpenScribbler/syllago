package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// dropdownActiveMsg is fired when the user selects an item via keyboard or mouse.
// The parent catches this to update content views.
type dropdownActiveMsg struct {
	id    string // which dropdown fired
	index int    // selected item index
	label string // selected item label
}

// dropdownModel is a reusable dropdown menu component.
type dropdownModel struct {
	id       string   // unique identifier (e.g., "content", "collection", "config")
	label    string   // display label (e.g., "Content", "Collection")
	items    []string // menu items
	active   int      // committed selection index
	cursor   int      // cursor position when open
	isOpen   bool
	disabled bool // grayed out (inactive dropdown in mutual exclusion)
}

func newDropdown(id, label string, items []string) dropdownModel {
	return dropdownModel{
		id:    id,
		label: label,
		items: items,
	}
}

// Update handles keyboard and mouse input when the dropdown is open.
// The parent is responsible for calling this only when appropriate.
func (d dropdownModel) Update(msg tea.Msg) (dropdownModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !d.isOpen {
			return d, nil
		}
		switch msg.String() {
		case "j", "down":
			d.cursor = (d.cursor + 1) % len(d.items)
		case "k", "up":
			d.cursor = (d.cursor - 1 + len(d.items)) % len(d.items)
		case "enter":
			d.active = d.cursor
			d.isOpen = false
			return d, d.activeCmd()
		case "esc":
			d.isOpen = false
		case "h", "left":
			// Parent handles switching between dropdowns
			d.isOpen = false
			return d, nil
		case "l", "right":
			d.isOpen = false
			return d, nil
		}

	case tea.MouseMsg:
		if !d.isOpen {
			return d, nil
		}
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return d, nil
		}
		for i := range d.items {
			if zone.Get(d.itemZoneID(i)).InBounds(msg) {
				d.active = i
				d.isOpen = false
				return d, d.activeCmd()
			}
		}
	}
	return d, nil
}

// Open opens the dropdown, setting cursor to current active item (or 0 if none).
func (d *dropdownModel) Open() {
	d.isOpen = true
	if d.active >= 0 && d.active < len(d.items) {
		d.cursor = d.active
	} else {
		d.cursor = 0
	}
}

// Close closes the dropdown without selecting.
func (d *dropdownModel) Close() {
	d.isOpen = false
}

// SetActive sets the active selection programmatically (no event fired).
func (d *dropdownModel) SetActive(index int) {
	if index >= 0 && index < len(d.items) {
		d.active = index
	}
}

// Reset clears the selection to "none" (-1).
func (d *dropdownModel) Reset() {
	d.active = -1
}

// ActiveLabel returns the label of the currently active item, or "--" if none.
func (d dropdownModel) ActiveLabel() string {
	if d.active < 0 || d.active >= len(d.items) {
		return "--"
	}
	return d.items[d.active]
}

// ViewTrigger renders the closed dropdown trigger (label + selection + arrow).
func (d dropdownModel) ViewTrigger() string {
	sel := d.ActiveLabel()
	text := d.label + ": " + sel + " ▾"

	var style lipgloss.Style
	if d.disabled {
		style = mutedStyle
	} else if d.isOpen {
		style = lipgloss.NewStyle().Bold(true).Foreground(primaryText)
	} else {
		style = lipgloss.NewStyle().Foreground(primaryText)
	}

	rendered := style.Render(text)
	return zone.Mark("dd-trigger-"+d.id, rendered)
}

// ViewMenu renders the open dropdown menu (bordered item list).
func (d dropdownModel) ViewMenu() string {
	if !d.isOpen {
		return ""
	}

	var lines []string
	for i, item := range d.items {
		var style lipgloss.Style
		prefix := "  "
		if i == d.cursor {
			style = dropdownSelectedStyle
			prefix = "> "
		} else {
			style = dropdownItemStyle
		}
		line := zone.Mark(d.itemZoneID(i), style.Render(prefix+item))
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	return dropdownBorderStyle.Render(content)
}

func (d dropdownModel) activeCmd() tea.Cmd {
	return func() tea.Msg {
		return dropdownActiveMsg{
			id:    d.id,
			index: d.active,
			label: d.ActiveLabel(),
		}
	}
}

func (d dropdownModel) itemZoneID(i int) string {
	return "dd-item-" + d.id + "-" + itoa(i)
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
