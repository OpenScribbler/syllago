package tui

import (
	"fmt"
	"strings"

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
	height     int // available terminal height for sidebar panel

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

	// ── AI Tools section ──
	s += labelStyle.Render("  AI Tools") + "\n"

	// Content type rows
	for i, ct := range m.types {
		count := m.counts[ct]
		label := ct.Label()
		countStr := fmt.Sprintf("%2d", count)

		line := fmt.Sprintf("%-*s%s", inner-len(countStr)-2, label, countStr)
		if len(line) > inner {
			line = line[:inner]
		}

		var rowContent string
		if i == m.cursor {
			rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, line))
		} else {
			rowContent = "  " + itemStyle.Render(line)
		}
		s += zone.Mark(fmt.Sprintf("sidebar-%d", i), rowContent) + "\n"
	}

	// My Tools (end of AI Tools section)
	myIdx := len(m.types)
	myCountStr := fmt.Sprintf("%2d", m.localCount)
	myLine := fmt.Sprintf("%-*s%s", inner-len(myCountStr)-2, "My Tools", myCountStr)
	if len(myLine) > inner {
		myLine = myLine[:inner]
	}
	var rowContent string
	if myIdx == m.cursor {
		rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, myLine))
	} else {
		rowContent = "  " + itemStyle.Render(myLine)
	}
	s += zone.Mark(fmt.Sprintf("sidebar-%d", myIdx), rowContent) + "\n"

	// Separator
	s += helpStyle.Render("  "+"─────────────") + "\n"

	// ── Configuration section ──
	s += labelStyle.Render("  Configuration") + "\n"

	// Utility items: Import, Update, Settings
	utilItems := []struct {
		label string
		index int
	}{
		{"Import", len(m.types) + 1},
		{"Update", len(m.types) + 2},
		{"Settings", len(m.types) + 3},
	}

	for _, u := range utilItems {
		var rowContent string
		if u.index == m.cursor {
			rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, u.label))
		} else {
			rowContent = "  " + itemStyle.Render(u.label)
		}
		s += zone.Mark(fmt.Sprintf("sidebar-%d", u.index), rowContent) + "\n"
	}

	// Version pinned to bottom-right of sidebar
	if m.version != "" && m.height > 0 {
		ver := "v" + m.version
		pad := inner - len(ver)
		if pad < 0 {
			pad = 0
		}
		verLine := strings.Repeat(" ", pad) + versionStyle.Render(ver)
		contentLines := strings.Count(s, "\n")
		gap := m.height - contentLines - 1
		if gap > 0 {
			s += strings.Repeat("\n", gap)
		}
		s += verLine
	}

	style := sidebarBorderStyle.Width(sidebarWidth)
	if m.height > 0 {
		style = style.Height(m.height)
	}
	return style.Render(s)
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
