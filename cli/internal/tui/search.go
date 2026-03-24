package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// searchQueryMsg is sent when the search query changes (character added/removed).
type searchQueryMsg struct {
	query string
}

// searchCancelMsg is sent when the user presses Esc to cancel search.
type searchCancelMsg struct{}

// searchConfirmMsg is sent when the user presses Enter to confirm the filter.
type searchConfirmMsg struct {
	query string
}

type searchModel struct {
	active bool
	query  string
	width  int
}

func newSearchModel() searchModel {
	return searchModel{}
}

func (m *searchModel) activate() {
	m.active = true
	m.query = ""
}

func (m *searchModel) deactivate() {
	m.active = false
}

// Update handles key events when the search bar is active.
// When inactive, it returns nil to signal no message was handled.
func (m *searchModel) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		m.deactivate()
		return func() tea.Msg { return searchCancelMsg{} }

	case tea.KeyEnter:
		q := m.query
		m.deactivate()
		return func() tea.Msg { return searchConfirmMsg{query: q} }

	case tea.KeyBackspace:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
		}
		return func() tea.Msg { return searchQueryMsg{query: m.query} }

	case tea.KeyRunes:
		m.query += string(keyMsg.Runes)
		return func() tea.Msg { return searchQueryMsg{query: m.query} }
	}

	return nil
}

// View renders the search bar when active. Returns empty string when inactive.
func (m searchModel) View() string {
	if !m.active {
		return ""
	}
	return labelStyle.Render("/") + " " + valueStyle.Render(m.query+"\u2588")
}

// filterItems returns items whose Name contains query (case-insensitive).
// Returns all items when query is empty.
func filterItems(items []catalog.ContentItem, query string) []catalog.ContentItem {
	if query == "" {
		return items
	}
	lower := strings.ToLower(query)
	var result []catalog.ContentItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), lower) {
			result = append(result, item)
		}
	}
	return result
}
