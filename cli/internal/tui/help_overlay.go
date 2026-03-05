package tui

import "strings"

type helpOverlayModel struct {
	active bool
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
		lines = append(lines, labelStyle.Render("Category Screen:"))
		lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
		lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
		lines = append(lines, "  "+helpStyle.Render("enter     select"))
		lines = append(lines, "  "+helpStyle.Render("q         quit"))

	case screenItems:
		lines = append(lines, labelStyle.Render("Items Screen:"))
		lines = append(lines, "  "+helpStyle.Render("up/k      move up"))
		lines = append(lines, "  "+helpStyle.Render("down/j    move down"))
		lines = append(lines, "  "+helpStyle.Render("enter     view details"))

	case screenDetail:
		lines = append(lines, labelStyle.Render("Detail Screen:"))
		lines = append(lines, "  "+helpStyle.Render("tab       next tab"))
		lines = append(lines, "  "+helpStyle.Render("shift+tab previous tab"))
		lines = append(lines, "  "+helpStyle.Render("1/2/3     jump to tab"))
		lines = append(lines, "  "+helpStyle.Render("up/down   scroll / navigate"))
		lines = append(lines, "  "+helpStyle.Render("i         install"))
		lines = append(lines, "  "+helpStyle.Render("u         uninstall"))
		lines = append(lines, "  "+helpStyle.Render("c         copy prompt"))
		lines = append(lines, "  "+helpStyle.Render("s         save prompt"))
		lines = append(lines, "  "+helpStyle.Render("e         env var setup (MCP)"))
		lines = append(lines, "  "+helpStyle.Render("p         share (library)"))

	case screenImport:
		lines = append(lines, labelStyle.Render("Import Screen:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate options"))
		lines = append(lines, "  "+helpStyle.Render("enter     select"))
		lines = append(lines, "  "+helpStyle.Render("space     toggle selection"))
		lines = append(lines, "  "+helpStyle.Render("a         select all"))
		lines = append(lines, "  "+helpStyle.Render("d         done (confirm)"))

	case screenSettings:
		lines = append(lines, labelStyle.Render("Settings Screen:"))
		lines = append(lines, "  "+helpStyle.Render("up/down   navigate"))
		lines = append(lines, "  "+helpStyle.Render("enter     edit setting"))
		lines = append(lines, "  "+helpStyle.Render("s         save"))
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Press ? or esc to close"))

	return strings.Join(lines, "\n")
}
