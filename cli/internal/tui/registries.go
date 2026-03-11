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
	return "arrows navigate • enter browse • a add • d remove • r sync • esc back"
}

func (m registriesModel) View(cursor int) string {
	s := renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Registries", ""},
	) + "\n\n"

	if len(m.entries) == 0 {
		s += helpStyle.Render("  No registries configured.") + "\n\n"
		s += helpStyle.Render("  Press a to add a registry.") + "\n"
		return s
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

	if singleCol {
		for i, entry := range m.entries {
			selected := i == cursor
			card := renderRegistryCard(entry, cardW, selected)
			s += zone.Mark(fmt.Sprintf("registry-card-%d", i), card) + "\n"
		}
	} else {
		for i := 0; i < len(m.entries); i += 2 {
			left := zone.Mark(fmt.Sprintf("registry-card-%d", i), renderRegistryCard(m.entries[i], cardW, i == cursor))
			var right string
			if i+1 < len(m.entries) {
				right = zone.Mark(fmt.Sprintf("registry-card-%d", i+1), renderRegistryCard(m.entries[i+1], cardW, i+1 == cursor))
			}
			s += lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n"
		}
	}

	return s
}

func renderRegistryCard(entry registryEntry, width int, selected bool) string {
	cardStyle := cardNormalStyle
	if selected {
		cardStyle = cardSelectedStyle
	}

	status := helpStyle.Render("missing")
	if entry.cloned {
		status = installedStyle.Render("cloned")
	}

	version := "─"
	if entry.version != "" {
		version = entry.version
	}

	urlDisplay := entry.url
	maxURL := width - 4
	if len(urlDisplay) > maxURL {
		urlDisplay = urlDisplay[:maxURL-3] + "..."
	}

	desc := entry.description
	if len(desc) > width-4 {
		desc = desc[:width-7] + "..."
	}

	title := truncate(entry.name, width-4)
	meta := fmt.Sprintf("%s  v%s  %d items", status, helpStyle.Render(version), entry.itemCount)

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, meta)
	lines = append(lines, helpStyle.Render(urlDisplay))
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
