package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

const sidebarWidth = 18 // fixed width including border character

type sidebarModel struct {
	types      []catalog.ContentType
	counts     map[catalog.ContentType]int
	localCount int
	cursor     int
	focused    bool

	// Version/update state (displayed in sidebar header)
	version         string
	remoteVersion   string
	updateAvailable bool
	commitsBehind   int
}

func newSidebarModel(cat *catalog.Catalog, version string) sidebarModel {
	return sidebarModel{
		types:      catalog.AllContentTypes(),
		counts:     cat.CountByType(),
		localCount: cat.CountLocal(),
		version:    version,
	}
}

// totalItems returns the total number of navigable items in the sidebar
// (content types + My Tools + Import + Update + Settings).
func (m sidebarModel) totalItems() int {
	return len(m.types) + 4
}

func (m sidebarModel) Update(msg tea.Msg) (sidebarModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < m.totalItems()-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			m.cursor = m.totalItems() - 1
		}
	}
	return m, nil
}

func (m sidebarModel) View() string {
	// Inner width: sidebarWidth minus 1 for the right border character
	inner := sidebarWidth - 1

	var s string

	// Header: "nesco" title
	s += titleStyle.Render("nesco") + "\n\n"

	// Content type rows
	for i, ct := range m.types {
		count := m.counts[ct]
		prefix := "  "
		label := ct.Label()
		countStr := fmt.Sprintf("%2d", count)

		line := fmt.Sprintf("%-*s%s", inner-len(countStr)-2, label, countStr)
		if len(line) > inner {
			line = line[:inner]
		}

		var rowContent string
		if i == m.cursor {
			rowContent = "▸ " + selectedItemStyle.Render(line)
		} else {
			rowContent = prefix + itemStyle.Render(line)
		}
		s += zone.Mark(fmt.Sprintf("sidebar-%d", i), rowContent) + "\n"
	}

	// Separator
	s += helpStyle.Render("  "+"─────────────") + "\n"

	// Utility items: My Tools, Import, Update, Settings
	utilItems := []struct {
		label  string
		index  int
		hasDot bool // true = ◆ (has items), false = ◇
	}{
		{fmt.Sprintf("My Tools %2d", m.localCount), len(m.types), m.localCount > 0},
		{"Import", len(m.types) + 1, false},
		{"Update", len(m.types) + 2, false},
		{"Settings", len(m.types) + 3, false},
	}

	for _, u := range utilItems {
		diamond := "◇"
		if u.hasDot {
			diamond = "◆"
		}
		var rowContent string
		if u.index == m.cursor {
			rowContent = diamond + " " + selectedItemStyle.Render(u.label)
		} else {
			rowContent = diamond + " " + itemStyle.Render(u.label)
		}
		s += zone.Mark(fmt.Sprintf("sidebar-%d", u.index), rowContent) + "\n"
	}

	return sidebarBorderStyle.Width(sidebarWidth).Render(s)
}

// Selector methods for use in App.Update routing
func (m sidebarModel) isMyToolsSelected() bool { return m.cursor == len(m.types) }
func (m sidebarModel) isImportSelected() bool   { return m.cursor == len(m.types)+1 }
func (m sidebarModel) isUpdateSelected() bool   { return m.cursor == len(m.types)+2 }
func (m sidebarModel) isSettingsSelected() bool { return m.cursor == len(m.types)+3 }
func (m sidebarModel) selectedType() catalog.ContentType {
	if m.cursor >= len(m.types) {
		return ""
	}
	return m.types[m.cursor]
}
