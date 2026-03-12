package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
)

type helpOverlayModel struct {
	active       bool
	scrollOffset int
	height       int // available viewport height
}

func (m *helpOverlayModel) Update(msg tea.KeyMsg) {
	switch {
	case key.Matches(msg, keys.Up):
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case key.Matches(msg, keys.Down):
		m.scrollOffset++
	case key.Matches(msg, keys.PageUp):
		m.scrollOffset -= m.height / 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	case key.Matches(msg, keys.PageDown):
		m.scrollOffset += m.height / 2
	case msg.Type == tea.KeyHome:
		m.scrollOffset = 0
	case msg.Type == tea.KeyEnd:
		m.scrollOffset = 99999 // clamped below
	case msg.Type == tea.KeyEsc:
		m.active = false
		m.scrollOffset = 0
	}
}

func (m helpOverlayModel) View(s screen) string {
	if !m.active {
		return ""
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Keyboard Shortcuts"))
	lines = append(lines, "")

	// Global shortcuts
	lines = append(lines, labelStyle.Render("Global:"))
	lines = append(lines, "  "+helpStyle.Render("?         help (this overlay)"))
	lines = append(lines, "  "+helpStyle.Render("ctrl+c    quit"))
	lines = append(lines, "  "+helpStyle.Render("/         search"))
	lines = append(lines, "  "+helpStyle.Render("esc       back / cancel"))
	lines = append(lines, "")

	// Context-sensitive shortcuts
	switch s {
	case screenCategory:
		lines = append(lines, labelStyle.Render("Home:"))
		lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
		lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
		lines = append(lines, "  "+helpStyle.Render("enter     select"))
		lines = append(lines, "  "+helpStyle.Render("tab       toggle sidebar / cards"))
		lines = append(lines, "  "+helpStyle.Render("H         toggle hidden items"))
		lines = append(lines, "  "+helpStyle.Render("q         quit"))

	case screenItems:
		lines = append(lines, labelStyle.Render("Items:"))
		lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
		lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
		lines = append(lines, "  "+helpStyle.Render("enter     view details"))
		lines = append(lines, "  "+helpStyle.Render("r         remove (library items)"))
		lines = append(lines, "  "+helpStyle.Render("tab       toggle sidebar"))

	case screenDetail:
		lines = append(lines, labelStyle.Render("Detail:"))
		lines = append(lines, "  "+helpStyle.Render("tab       next tab"))
		lines = append(lines, "  "+helpStyle.Render("shift+tab previous tab"))
		lines = append(lines, "  "+helpStyle.Render("1/2/3     jump to tab"))
		lines = append(lines, "  "+helpStyle.Render("up/down   scroll / navigate"))
		lines = append(lines, "  "+helpStyle.Render("i         install"))
		lines = append(lines, "  "+helpStyle.Render("u         uninstall"))
		lines = append(lines, "  "+helpStyle.Render("r         remove from library"))
		lines = append(lines, "  "+helpStyle.Render("c         copy prompt"))
		lines = append(lines, "  "+helpStyle.Render("e         env var setup (MCP)"))
		lines = append(lines, "  "+helpStyle.Render("p         share (library)"))

	case screenLibraryCards:
		lines = append(lines, labelStyle.Render("Library:"))
		lines = append(lines, "  "+helpStyle.Render("arrows    navigate cards"))
		lines = append(lines, "  "+helpStyle.Render("enter     browse category"))
		lines = append(lines, "  "+helpStyle.Render("tab       toggle sidebar / cards"))
		lines = append(lines, "  "+helpStyle.Render("a         add content"))

	case screenLoadoutCards:
		lines = append(lines, labelStyle.Render("Loadouts:"))
		lines = append(lines, "  "+helpStyle.Render("arrows    navigate cards"))
		lines = append(lines, "  "+helpStyle.Render("enter     browse provider"))
		lines = append(lines, "  "+helpStyle.Render("tab       toggle sidebar / cards"))
		lines = append(lines, "  "+helpStyle.Render("a         create loadout"))

	case screenRegistries:
		lines = append(lines, labelStyle.Render("Registries:"))
		lines = append(lines, "  "+helpStyle.Render("arrows    navigate cards"))
		lines = append(lines, "  "+helpStyle.Render("enter     browse registry"))
		lines = append(lines, "  "+helpStyle.Render("tab       toggle sidebar / cards"))
		lines = append(lines, "  "+helpStyle.Render("a         add registry"))
		lines = append(lines, "  "+helpStyle.Render("r         remove registry"))
		lines = append(lines, "  "+helpStyle.Render("s         sync registry"))

	case screenImport:
		lines = append(lines, labelStyle.Render("Add:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate options"))
		lines = append(lines, "  "+helpStyle.Render("enter     select"))
		lines = append(lines, "  "+helpStyle.Render("space     toggle selection"))
		lines = append(lines, "  "+helpStyle.Render("a         select all"))
		lines = append(lines, "  "+helpStyle.Render("d         done (confirm)"))

	case screenUpdate:
		lines = append(lines, labelStyle.Render("Update:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate options"))
		lines = append(lines, "  "+helpStyle.Render("enter     select"))

	case screenSettings:
		lines = append(lines, labelStyle.Render("Settings:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate"))
		lines = append(lines, "  "+helpStyle.Render("enter     edit setting"))
		lines = append(lines, "  "+helpStyle.Render("s         save"))

	case screenSandbox:
		lines = append(lines, labelStyle.Render("Sandbox:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate"))
		lines = append(lines, "  "+helpStyle.Render("enter     toggle / select"))
		lines = append(lines, "  "+helpStyle.Render("s         save"))
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Press ? or esc to close"))

	// Scroll if content exceeds available height
	if m.height > 0 && len(lines) > m.height {
		offset := m.scrollOffset
		if offset > len(lines)-m.height {
			offset = len(lines) - m.height
		}
		if offset < 0 {
			offset = 0
		}
		end := offset + m.height
		if end > len(lines) {
			end = len(lines)
		}
		var s string
		if offset > 0 {
			s += renderScrollUp(offset, true) + "\n"
		}
		s += strings.Join(lines[offset:end], "\n")
		remaining := len(lines) - end
		if remaining > 0 {
			s += "\n" + renderScrollDown(remaining, true)
		}
		return s
	}

	return strings.Join(lines, "\n")
}
