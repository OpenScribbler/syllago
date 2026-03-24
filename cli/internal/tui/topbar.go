package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// topBarSelectMsg is sent when a dropdown item is selected.
type topBarSelectMsg struct {
	category dropdownID
	item     string
}

// topBarModel is the top navigation bar with three dropdowns and action buttons.
type topBarModel struct {
	contentMenu    dropdownMenu // Skills, Agents, MCP Configs, Rules, Hooks, Commands
	collectionMenu dropdownMenu // Library, Registries, Loadouts
	configMenu     dropdownMenu // Settings, Sandbox
	activeCategory dropdownID   // which category is active (content vs collection)
	openMenu       dropdownID   // which menu is currently open (-1 = none)
	menuOpen       bool
	width          int
}

// newTopBarModel creates the top bar with default dropdown items.
func newTopBarModel() topBarModel {
	return topBarModel{
		contentMenu:    newDropdownMenu([]string{"Skills", "Agents", "MCP Configs", "Rules", "Hooks", "Commands"}),
		collectionMenu: newDropdownMenu([]string{"Library", "Registries", "Loadouts"}),
		configMenu:     newDropdownMenu([]string{"Settings", "Sandbox"}),
		activeCategory: dropdownContent,
		openMenu:       -1,
	}
}

// getMenu returns a pointer to the dropdown for the given ID.
func (m *topBarModel) getMenu(id dropdownID) *dropdownMenu {
	switch id {
	case dropdownContent:
		return &m.contentMenu
	case dropdownCollection:
		return &m.collectionMenu
	case dropdownConfig:
		return &m.configMenu
	}
	return nil
}

// toggleMenu opens the given dropdown, closing any other open menu.
// If the requested menu is already open, it closes it.
func (m *topBarModel) toggleMenu(id dropdownID) {
	if m.menuOpen && m.openMenu == id {
		// Close the current menu
		m.getMenu(id).close()
		m.menuOpen = false
		m.openMenu = -1
		return
	}
	// Close any open menu first
	if m.menuOpen {
		m.getMenu(m.openMenu).close()
	}
	// Open the requested menu
	menu := m.getMenu(id)
	menu.open = true
	m.menuOpen = true
	m.openMenu = id
}

// closeAll closes whichever menu is open.
func (m *topBarModel) closeAll() {
	if m.menuOpen {
		m.getMenu(m.openMenu).close()
	}
	m.menuOpen = false
	m.openMenu = -1
}

// Update handles keyboard input for dropdown navigation.
func (m topBarModel) Update(msg tea.Msg) (topBarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Toggle dropdowns with 1/2/3
		if key.Matches(msg, keys.Dropdown1) {
			m.toggleMenu(dropdownContent)
			return m, nil
		}
		if key.Matches(msg, keys.Dropdown2) {
			m.toggleMenu(dropdownCollection)
			return m, nil
		}
		if key.Matches(msg, keys.Dropdown3) {
			m.toggleMenu(dropdownConfig)
			return m, nil
		}

		// When a menu is open, handle navigation
		if m.menuOpen {
			menu := m.getMenu(m.openMenu)
			switch {
			case key.Matches(msg, keys.Up):
				menu.moveUp()
				return m, nil
			case key.Matches(msg, keys.Down):
				menu.moveDown()
				return m, nil
			case msg.Type == tea.KeyEnter:
				selected := menu.selected()
				category := m.openMenu
				// Update active category for content/collection
				if category == dropdownContent || category == dropdownCollection {
					m.activeCategory = category
				}
				m.closeAll()
				return m, func() tea.Msg {
					return topBarSelectMsg{category: category, item: selected}
				}
			case msg.Type == tea.KeyEsc:
				m.closeAll()
				return m, nil
			}
		}
	}
	return m, nil
}

// View renders the top bar with dropdowns and action buttons.
func (m topBarModel) View(width int) string {
	// Build the inline bar content
	var b strings.Builder

	b.WriteString(logoStyle.Render(" SYL"))
	b.WriteString("    ")

	// Content dropdown label
	b.WriteString(m.renderLabel(dropdownContent, "Content: ", m.contentMenu.selected()))
	b.WriteString("    ")

	// Collection dropdown label
	b.WriteString(m.renderLabel(dropdownCollection, "Collection: ", m.collectionMenu.selected()))
	b.WriteString("    ")

	// Config dropdown label
	b.WriteString(m.renderLabel(dropdownConfig, "", "Config"))

	// Action buttons right-aligned
	add := actionBtnAddStyle.Render("+ Add")
	newBtn := actionBtnNewStyle.Render("* New")
	actions := "    " + add + "  " + newBtn

	b.WriteString(actions)

	inner := lipgloss.NewStyle().Width(width - 2).Render(b.String())
	return topBarStyle.Width(width).Render(inner)
}

// renderLabel renders a single dropdown label with active/inactive styling.
func (m topBarModel) renderLabel(id dropdownID, prefix, value string) string {
	indicator := " \u25be"

	switch id {
	case dropdownContent:
		if m.activeCategory == dropdownContent {
			return activeDropdownStyle.Render(prefix+value) + indicator
		}
		return inactiveDropdownStyle.Render(prefix+"--") + indicator
	case dropdownCollection:
		if m.activeCategory == dropdownCollection {
			return collectionDropdownStyle.Render(prefix+value) + indicator
		}
		return inactiveDropdownStyle.Render(prefix+"--") + indicator
	case dropdownConfig:
		return inactiveDropdownStyle.Render(value) + indicator
	}
	return ""
}
