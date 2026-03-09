package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

const sidebarWidth = 18 // fixed width including border character

type sidebarModel struct {
	types          []catalog.ContentType // content types (excludes Loadouts)
	counts         map[catalog.ContentType]int
	libraryCount   int
	loadoutsCount  int
	registryCount  int // number of configured registries
	cursor         int
	focused        bool
	height         int // available terminal height for sidebar panel

	// Version/update state (displayed in sidebar header)
	version         string
	remoteVersion   string
	updateAvailable bool
	commitsBehind   int
}

func newSidebarModel(cat *catalog.Catalog, version string, registryCount int) sidebarModel {
	// Filter Loadouts from content types — it appears in Collections section.
	var filtered []catalog.ContentType
	for _, ct := range catalog.AllContentTypes() {
		if ct != catalog.Loadouts {
			filtered = append(filtered, ct)
		}
	}
	return sidebarModel{
		types:         filtered,
		counts:        cat.CountByType(),
		libraryCount:  cat.CountLibrary(),
		loadoutsCount: cat.CountByType()[catalog.Loadouts],
		registryCount: registryCount,
		version:       version,
	}
}

// totalItems returns the total number of navigable items in the sidebar
// (content types + Library + Loadouts + Add + Update + Settings + Registries + Sandbox).
func (m sidebarModel) totalItems() int {
	return len(m.types) + 7
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

	// Header: "syllago" title
	s += titleStyle.Render("syllago") + "\n\n"

	// ── Content section ──
	s += labelStyle.Render("  Content") + "\n"

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

	// Separator
	s += helpStyle.Render("  " + "─────────────") + "\n"

	// ── Collections section ──
	s += labelStyle.Render("  Collections") + "\n"

	// Library
	libIdx := len(m.types)
	libCountStr := fmt.Sprintf("%2d", m.libraryCount)
	libLine := fmt.Sprintf("%-*s%s", inner-len(libCountStr)-2, "Library", libCountStr)
	if len(libLine) > inner {
		libLine = libLine[:inner]
	}
	var rowContent string
	if libIdx == m.cursor {
		rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, libLine))
	} else {
		rowContent = "  " + itemStyle.Render(libLine)
	}
	s += zone.Mark(fmt.Sprintf("sidebar-%d", libIdx), rowContent) + "\n"

	// Loadouts
	loadIdx := len(m.types) + 1
	loadCountStr := fmt.Sprintf("%2d", m.loadoutsCount)
	loadLine := fmt.Sprintf("%-*s%s", inner-len(loadCountStr)-2, "Loadouts", loadCountStr)
	if len(loadLine) > inner {
		loadLine = loadLine[:inner]
	}
	if loadIdx == m.cursor {
		rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, loadLine))
	} else {
		rowContent = "  " + itemStyle.Render(loadLine)
	}
	s += zone.Mark(fmt.Sprintf("sidebar-%d", loadIdx), rowContent) + "\n"

	// Separator
	s += helpStyle.Render("  " + "─────────────") + "\n"

	// ── Configuration section ──
	s += labelStyle.Render("  Configuration") + "\n"

	// Utility items: Add, Update, Settings, Registries, Sandbox
	utilItems := []struct {
		label string
		index int
	}{
		{"Add", len(m.types) + 2},
		{"Update", len(m.types) + 3},
		{"Settings", len(m.types) + 4},
		{"Registries", len(m.types) + 5},
		{"Sandbox", len(m.types) + 6},
	}

	for _, u := range utilItems {
		var rowContent string
		// Registries shows item count like content types
		if u.label == "Registries" && m.registryCount > 0 {
			countStr := fmt.Sprintf("%2d", m.registryCount)
			line := fmt.Sprintf("%-*s%s", inner-len(countStr)-2, u.label, countStr)
			if u.index == m.cursor {
				rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, line))
			} else {
				rowContent = "  " + itemStyle.Render(line)
			}
		} else {
			if u.index == m.cursor {
				rowContent = selectedItemStyle.Render(fmt.Sprintf("▸ %-*s", inner-2, u.label))
			} else {
				rowContent = "  " + itemStyle.Render(fmt.Sprintf("%-*s", inner-2, u.label))
			}
		}
		s += zone.Mark(fmt.Sprintf("sidebar-%d", u.index), rowContent) + "\n"
	}

	// Version pinned to bottom-right of sidebar, only when it fits
	if m.version != "" && m.height > 0 {
		contentLines := strings.Count(s, "\n")
		// Only show version if there's at least 1 line of gap below content
		if contentLines+1 < m.height {
			ver := "v" + m.version
			pad := inner - len(ver)
			if pad < 0 {
				pad = 0
			}
			verLine := strings.Repeat(" ", pad) + versionStyle.Render(ver)
			gap := m.height - contentLines - 1
			s += strings.Repeat("\n", gap) + verLine
		}
	}

	style := sidebarBorderStyle.Width(sidebarWidth)
	if m.height > 0 {
		style = style.Height(m.height)
	}
	return style.Render(s)
}

// Selector methods for use in App.Update routing
func (m sidebarModel) isLibrarySelected() bool    { return m.cursor == len(m.types) }
func (m sidebarModel) isLoadoutsSelected() bool   { return m.cursor == len(m.types)+1 }
func (m sidebarModel) isAddSelected() bool        { return m.cursor == len(m.types)+2 }
func (m sidebarModel) isUpdateSelected() bool     { return m.cursor == len(m.types)+3 }
func (m sidebarModel) isSettingsSelected() bool   { return m.cursor == len(m.types)+4 }
func (m sidebarModel) isRegistriesSelected() bool { return m.cursor == len(m.types)+5 }
func (m sidebarModel) isSandboxSelected() bool    { return m.cursor == len(m.types)+6 }
func (m sidebarModel) selectedType() catalog.ContentType {
	if m.cursor >= len(m.types) {
		return ""
	}
	return m.types[m.cursor]
}
