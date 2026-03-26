package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// metaPanelData holds pre-computed display strings for the metadata panel.
type metaPanelData struct {
	installed  string // "CC,GC,Cu" or "--"
	typeDetail string // type-specific detail (hooks/MCP/loadouts) or ""
}

// computeMetaPanelData computes installed status and type-specific detail for an item.
func computeMetaPanelData(item catalog.ContentItem, providers []provider.Provider, repoRoot string) metaPanelData {
	var abbrevs []string
	for _, prov := range providers {
		if installer.CheckStatus(item, prov, repoRoot) == installer.StatusInstalled {
			abbrevs = append(abbrevs, providerAbbrev(prov.Slug))
		}
	}
	installed := "--"
	if len(abbrevs) > 0 {
		installed = strings.Join(abbrevs, ",")
	}

	return metaPanelData{
		installed:  installed,
		typeDetail: computeTypeDetail(item),
	}
}

// renderMetaPanel renders the 3-line metadata content for any content item.
// This is a shared component used by both the library table and explorer views.
func renderMetaPanel(item *catalog.ContentItem, data metaPanelData, width int) string {
	if item == nil {
		blank := strings.Repeat(" ", width)
		return blank + "\n" + blank + "\n" + blank
	}

	chip := func(key, val string, w int) string {
		valW := w - len(key) - 2
		val = padRight(truncate(sanitizeLine(val), valW), valW)
		return boldStyle.Render(key+": ") + mutedStyle.Render(val)
	}

	gap := "  "

	tryAdd := func(line, c string) string {
		candidate := line + gap + c
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
		return line
	}

	// --- Line 1: name (40), type, files, origin, installed ---
	nameMaxW := 40
	displayName := truncate(sanitizeLine(itemDisplayName(*item)), nameMaxW)
	line1 := " " + boldStyle.Render(padRight(displayName, nameMaxW))
	line1 += "  " + chip("Type", typeLabel(item.Type), 14)
	line1 = tryAdd(line1, chip("Files", itoa(len(item.Files)), 9))

	origin := "syllago"
	if item.Meta != nil && item.Meta.SourceProvider != "" {
		origin = item.Meta.SourceProvider
	} else if item.Provider != "" {
		origin = item.Provider
	}
	line1 = tryAdd(line1, chip("Origin", origin, 19))

	line1 = tryAdd(line1, boldStyle.Render("Installed: ")+mutedStyle.Render(data.installed))

	// --- Line 2: scope, registry, path ---
	scope := "project"
	if item.Meta != nil && item.Meta.SourceScope != "" {
		scope = item.Meta.SourceScope
	} else if item.Library {
		scope = "global"
	}
	line2 := " " + chip("Scope", scope, 15)

	regName := "not in a registry"
	if item.Registry != "" {
		regName = item.Registry
	} else if item.Meta != nil && item.Meta.SourceRegistry != "" {
		regName = item.Meta.SourceRegistry
	}
	line2 += gap + chip("Registry", regName, 30)

	if item.Path != "" {
		path := item.Path
		if home, err := homeDir(); err == nil && strings.HasPrefix(path, home) {
			path = "~" + path[len(home):]
		}
		pathW := max(20, width-lipgloss.Width(line2)-10)
		line2 = tryAdd(line2, boldStyle.Render("Path: ")+mutedStyle.Render(truncateMiddle(path, pathW)))
	}

	// --- Line 3: type-specific detail + rename button ---
	line3 := ""
	if data.typeDetail != "" {
		segments := strings.Split(sanitizeLine(data.typeDetail), " · ")
		styled := " "
		for i, seg := range segments {
			if i > 0 {
				styled += gap
			}
			if idx := strings.Index(seg, ": "); idx >= 0 {
				styled += boldStyle.Render(seg[:idx+2]) + mutedStyle.Render(seg[idx+2:])
			} else {
				styled += mutedStyle.Render(seg)
			}
		}
		line3 = styled
	}

	// Per-item action buttons ordered: [x] Uninstall, [d] Remove, [e] Edit
	// All three always visible. Disabled buttons grayed out (no jank from appearing/disappearing).
	canUninstall := data.installed != "--"
	canRemove := item.Library

	var btns []string
	if canUninstall {
		btns = append(btns, zone.Mark("meta-uninstall", activeButtonStyle.Render("[x] Uninstall")))
	} else {
		btns = append(btns, disabledButtonStyle.Render("[x] Uninstall"))
	}
	if canRemove {
		btns = append(btns, zone.Mark("meta-remove", activeButtonStyle.Render("[d] Remove")))
	} else {
		btns = append(btns, disabledButtonStyle.Render("[d] Remove"))
	}
	btns = append(btns, zone.Mark("meta-edit", activeButtonStyle.Render("[e] Edit")))
	btnRow := strings.Join(btns, " ")
	btnRowW := lipgloss.Width(btnRow)

	// Truncate type detail to ensure buttons always fit — buttons must never be clipped.
	maxDetailW := max(0, width-btnRowW-2) // -2 for minimum gap
	line3W := lipgloss.Width(line3)
	if line3W > maxDetailW {
		line3 = lipgloss.NewStyle().MaxWidth(maxDetailW).Render(line3)
		line3W = lipgloss.Width(line3)
	}
	btnGap := max(1, width-line3W-btnRowW)
	line3 += strings.Repeat(" ", btnGap) + btnRow

	pad := func(s string) string {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
		if g := width - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}
	return pad(line1) + "\n" + pad(line2) + "\n" + pad(line3)
}
