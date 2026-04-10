package tui_v1

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

type searchModel struct {
	input      textinput.Model
	active     bool
	matchCount int // -1 = not showing count
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "type to search..."
	ti.Prompt = searchPromptStyle.Render("Search: ")
	ti.CharLimit = 50
	return searchModel{input: ti, matchCount: -1}
}

func (m searchModel) activated() searchModel {
	m.active = true
	m.input.Focus()
	return m
}

func (m searchModel) deactivated() searchModel {
	m.active = false
	m.matchCount = -1
	m.input.Blur()
	m.input.SetValue("")
	return m
}

func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m searchModel) View() string {
	if !m.active {
		return ""
	}
	v := m.input.View()
	if m.matchCount >= 0 {
		v += " " + helpStyle.Render(fmt.Sprintf("(%d matches)", m.matchCount))
	}
	return v
}

func (m searchModel) query() string {
	return strings.TrimSpace(m.input.Value())
}

// filterItems returns items matching the search query (case-insensitive substring match).
func filterItems(items []catalog.ContentItem, query string) []catalog.ContentItem {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var result []catalog.ContentItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), q) ||
			strings.Contains(strings.ToLower(item.Description), q) ||
			strings.Contains(strings.ToLower(item.Provider), q) {
			result = append(result, item)
		}
	}
	return result
}
