package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// registryEntry holds display data for one registry card.
type registryEntry struct {
	name        string
	url         string
	ref         string
	cloned      bool
	itemCount   int
	version     string
	description string
}

type registriesModel struct {
	entries    []registryEntry
	allEntries []registryEntry // unfiltered entries for search reset
	width      int
	height     int
	repoRoot   string
}

func newRegistriesModel(repoRoot string, cfg *config.Config, cat *catalog.Catalog) registriesModel {
	entries := make([]registryEntry, len(cfg.Registries))
	for i, r := range cfg.Registries {
		entry := registryEntry{
			name:      r.Name,
			url:       r.URL,
			ref:       r.Ref,
			cloned:    registry.IsCloned(r.Name),
			itemCount: cat.CountRegistry(r.Name),
		}
		if manifest, err := registry.LoadManifest(r.Name); err == nil && manifest != nil {
			entry.version = manifest.Version
			entry.description = manifest.Description
		}
		entries[i] = entry
	}
	return registriesModel{
		entries:    entries,
		allEntries: entries,
		repoRoot:   repoRoot,
	}
}

func (m registriesModel) Update(msg tea.Msg) (registriesModel, tea.Cmd) {
	// Navigation is handled by App.Update(). This model has no cursor state.
	return m, nil
}

func (m registriesModel) helpText() string {
	return "arrows navigate • enter browse • a add registry • r remove registry • s sync registry • esc back"
}

func (m registriesModel) View(cursor, scrollOffset int) (string, int) {
	s := renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Registries", ""},
	) + "\n"
	s += renderActionButtons(
		ActionButton{"a", "Add Registry", "action-a", actionBtnAddStyle},
		ActionButton{"s", "Sync", "action-s", actionBtnSyncStyle},
		ActionButton{"r", "Remove", "action-r", actionBtnRemoveStyle},
	) + "\n"

	if len(m.entries) == 0 {
		s += helpStyle.Render("  No registries configured.") + "\n\n"
		s += helpStyle.Render("  Press a to add a registry.") + "\n"
		return s, 0
	}

	contentW := m.width
	singleCol := contentW < 42
	cardW := (contentW - 5) / 2
	if singleCol {
		cardW = contentW - 2
	}
	if cardW < 18 {
		cardW = 18
	}

	cols := 2
	if singleCol {
		cols = 1
	}
	totalRows := (len(m.entries) + cols - 1) / cols
	cardRowHeight := 7 // registry cards are ~5 lines + 2 border
	if singleCol {
		cardRowHeight = 8
	}
	headerLines := 3 // breadcrumb + blank line
	availH := m.height - headerLines
	firstRow, visibleRows, newOffset := cardScrollRange(cursor, len(m.entries), cols, availH, cardRowHeight, scrollOffset)

	if firstRow > 0 {
		hiddenAbove := firstRow * cols
		if hiddenAbove > len(m.entries) {
			hiddenAbove = len(m.entries)
		}
		s += "  " + renderScrollUp(hiddenAbove, false) + "\n"
	}

	lastRow := firstRow + visibleRows
	if lastRow > totalRows {
		lastRow = totalRows
	}

	// Fixed height for two-column mode ensures uniform card alignment.
	// cardRowHeight includes border (2) + newline gap (1), so inner = cardRowHeight - 3.
	fixedH := 0
	if !singleCol {
		fixedH = cardRowHeight - 3
	}

	if singleCol {
		for row := firstRow; row < lastRow; row++ {
			i := row
			if i >= len(m.entries) {
				break
			}
			selected := i == cursor
			card := renderRegistryCard(m.entries[i], cardW, selected, fixedH)
			s += zone.Mark(fmt.Sprintf("registry-card-%d", i), card) + "\n"
		}
	} else {
		for row := firstRow; row < lastRow; row++ {
			i := row * 2
			if i >= len(m.entries) {
				break
			}
			left := zone.Mark(fmt.Sprintf("registry-card-%d", i), renderRegistryCard(m.entries[i], cardW, i == cursor, fixedH))
			var right string
			if i+1 < len(m.entries) {
				right = zone.Mark(fmt.Sprintf("registry-card-%d", i+1), renderRegistryCard(m.entries[i+1], cardW, i+1 == cursor, fixedH))
			}
			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		}
	}

	hiddenBelow := len(m.entries) - lastRow*cols
	if hiddenBelow < 0 {
		hiddenBelow = 0
	}
	if hiddenBelow > 0 {
		s += "  " + renderScrollDown(hiddenBelow, false) + "\n"
	}

	return s, newOffset
}

func renderRegistryCard(entry registryEntry, width int, selected bool, fixedHeight int) string {
	cardStyle := cardNormalStyle
	if selected {
		cardStyle = cardSelectedStyle
	}
	if fixedHeight > 0 {
		cardStyle = cardStyle.Height(fixedHeight)
	}

	status := helpStyle.Render("missing")
	if entry.cloned {
		status = installedStyle.Render("cloned")
	}

	urlDisplay := truncate(entry.url, width-4)
	desc := truncate(entry.description, width-4)
	title := truncate(entry.name, width-4)

	var meta string
	if entry.version != "" {
		meta = fmt.Sprintf("%s  v%s  %d items", status, helpStyle.Render(entry.version), entry.itemCount)
	} else {
		meta = fmt.Sprintf("%s  %d items", status, entry.itemCount)
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, meta)
	// Wrap URL in OSC 8 hyperlink so terminals link to the full URL,
	// not the truncated display text.
	urlRendered := helpStyle.Render(urlDisplay)
	if urlDisplay != entry.url {
		urlRendered = fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", entry.url, urlRendered)
	}
	lines = append(lines, urlRendered)
	if desc != "" {
		lines = append(lines, helpStyle.Render(desc))
	}

	return cardStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func filterRegistryEntries(entries []registryEntry, query string) []registryEntry {
	if query == "" {
		return entries
	}
	q := strings.ToLower(query)
	var result []registryEntry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.name), q) ||
			strings.Contains(strings.ToLower(e.url), q) ||
			strings.Contains(strings.ToLower(e.description), q) {
			result = append(result, e)
		}
	}
	return result
}
