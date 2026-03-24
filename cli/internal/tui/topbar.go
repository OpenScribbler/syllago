package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// activeCategory tracks which dropdown group is currently active.
type activeCategory int

const (
	categoryContent    activeCategory = iota // Content types view
	categoryCollection                       // Collection view (Library/Registries/Loadouts)
)

// topBarModel manages the top navigation bar with three dropdowns.
type topBarModel struct {
	content    dropdownModel
	collection dropdownModel
	config     dropdownModel

	activeCategory activeCategory // which of Content/Collection is active
	width          int
}

func newTopBar() topBarModel {
	content := newDropdown("content", "Content", []string{
		"Skills", "Agents", "MCP Configs", "Rules", "Hooks", "Commands",
	})
	content.SetActive(0) // default: Skills

	collection := newDropdown("collection", "Collection", []string{
		"Library", "Registries", "Loadouts",
	})
	collection.Reset()         // no selection
	collection.disabled = true // grayed out initially

	config := newDropdown("config", "Config", []string{
		"Settings", "Sandbox",
	})
	config.Reset() // no selection

	return topBarModel{
		content:        content,
		collection:     collection,
		config:         config,
		activeCategory: categoryContent,
	}
}

// SetSize updates the topbar width.
func (t *topBarModel) SetSize(width int) {
	t.width = width
}

// HasOpenDropdown returns true if any dropdown is open.
func (t topBarModel) HasOpenDropdown() bool {
	return t.content.isOpen || t.collection.isOpen || t.config.isOpen
}

// Update handles input for the topbar and its dropdowns.
func (t topBarModel) Update(msg tea.Msg) (topBarModel, tea.Cmd) {
	// Route to the open dropdown first.
	if t.content.isOpen {
		var cmd tea.Cmd
		t.content, cmd = t.content.Update(msg)
		return t, cmd
	}
	if t.collection.isOpen {
		var cmd tea.Cmd
		t.collection, cmd = t.collection.Update(msg)
		return t, cmd
	}
	if t.config.isOpen {
		var cmd tea.Cmd
		t.config, cmd = t.config.Update(msg)
		return t, cmd
	}

	// Handle mouse clicks on triggers when no dropdown is open.
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
			if zone.Get("dd-trigger-content").InBounds(mouseMsg) {
				t.closeAll()
				t.content.Open()
				return t, nil
			}
			if zone.Get("dd-trigger-collection").InBounds(mouseMsg) {
				t.closeAll()
				t.collection.Open()
				return t, nil
			}
			if zone.Get("dd-trigger-config").InBounds(mouseMsg) {
				t.closeAll()
				t.config.Open()
				return t, nil
			}
		}
	}

	return t, nil
}

// OpenDropdown opens a specific dropdown by hotkey index (1=Content, 2=Collection, 3=Config).
func (t *topBarModel) OpenDropdown(n int) {
	t.closeAll()
	switch n {
	case 1:
		t.content.Open()
	case 2:
		t.collection.Open()
	case 3:
		t.config.Open()
	}
}

// HandleActiveMsg processes a dropdown selection and applies mutual exclusion.
func (t *topBarModel) HandleActiveMsg(msg dropdownActiveMsg) {
	switch msg.id {
	case "content":
		t.content.SetActive(msg.index)
		t.activeCategory = categoryContent
		t.collection.Reset()
		t.collection.disabled = true
		t.content.disabled = false
	case "collection":
		t.collection.SetActive(msg.index)
		t.activeCategory = categoryCollection
		t.content.Reset()
		t.content.disabled = true
		t.collection.disabled = false
	case "config":
		t.config.SetActive(msg.index)
	}
}

func (t *topBarModel) closeAll() {
	t.content.Close()
	t.collection.Close()
	t.config.Close()
}

// View renders the full topbar. Height grows when a dropdown is open.
func (t topBarModel) View() string {
	bar := t.renderBar()
	// If a dropdown is open, append its menu below the bar.
	if t.content.isOpen {
		menu := t.content.ViewMenu()
		return lipgloss.JoinVertical(lipgloss.Left, bar, menu)
	}
	if t.collection.isOpen {
		menu := t.collection.ViewMenu()
		return lipgloss.JoinVertical(lipgloss.Left, bar, menu)
	}
	if t.config.isOpen {
		menu := t.config.ViewMenu()
		return lipgloss.JoinVertical(lipgloss.Left, bar, menu)
	}
	return bar
}

// Height returns the rendered height of the topbar (1 when closed, more when open).
func (t topBarModel) Height() int {
	h := 1
	if t.content.isOpen {
		h += len(t.content.items) + 2 // items + border
	}
	if t.collection.isOpen {
		h += len(t.collection.items) + 2
	}
	if t.config.isOpen {
		h += len(t.config.items) + 2
	}
	return h
}

// renderBar renders the single-line closed topbar.
func (t topBarModel) renderBar() string {
	logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")

	triggers := strings.Join([]string{
		t.content.ViewTrigger(),
		t.collection.ViewTrigger(),
		t.config.ViewTrigger(),
	}, "  ")

	left := logo + "  " + triggers

	// Action buttons (Phase 4+ will make these functional).
	add := zone.Mark("btn-add", mutedStyle.Render("+ Add"))
	new := zone.Mark("btn-new", mutedStyle.Render("* New"))
	right := add + "  " + new

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := max(0, t.width-leftW-rightW)

	return left + strings.Repeat(" ", gap) + right
}
