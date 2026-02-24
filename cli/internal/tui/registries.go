package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/registry"
)

// registryEntry holds display data for one registry row.
type registryEntry struct {
	name      string
	url       string
	ref       string
	cloned    bool
	itemCount int
}

type registriesModel struct {
	entries  []registryEntry
	cursor   int
	width    int
	height   int
	repoRoot string
}

func newRegistriesModel(repoRoot string, cfg *config.Config, cat *catalog.Catalog) registriesModel {
	entries := make([]registryEntry, len(cfg.Registries))
	for i, r := range cfg.Registries {
		entries[i] = registryEntry{
			name:      r.Name,
			url:       r.URL,
			ref:       r.Ref,
			cloned:    registry.IsCloned(r.Name),
			itemCount: cat.CountRegistry(r.Name),
		}
	}
	return registriesModel{
		entries:  entries,
		repoRoot: repoRoot,
	}
}

func (m registriesModel) Update(msg tea.Msg) (registriesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			for i := range m.entries {
				if zone.Get(fmt.Sprintf("registry-row-%d", i)).InBounds(msg) {
					m.cursor = i
					// Synthesize Enter to drill in
					return m, func() tea.Msg {
						return tea.KeyMsg{Type: tea.KeyEnter}
					}
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			if len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			}
		}
	}
	return m, nil
}

func (m registriesModel) selectedName() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return ""
	}
	return m.entries[m.cursor].name
}

func (m registriesModel) View() string {
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	s := home + helpStyle.Render(" > ") + titleStyle.Render("Registries") + "\n\n"

	if len(m.entries) == 0 {
		s += helpStyle.Render("  No registries configured.") + "\n\n"
		s += helpStyle.Render("  Add one with: nesco registry add <git-url>") + "\n"
		s += "\n" + helpStyle.Render("esc back")
		return s
	}

	// Header
	s += tableHeaderStyle.Render(fmt.Sprintf("   %-20s  %-8s  %-6s  %s", "Name", "Status", "Items", "URL")) + "\n"
	s += helpStyle.Render("   " + strings.Repeat("─", 20) + "  " + strings.Repeat("─", 8) + "  " + strings.Repeat("─", 6) + "  " + strings.Repeat("─", 30)) + "\n"

	for i, entry := range m.entries {
		prefix := "   "
		nameStyle := itemStyle
		if i == m.cursor {
			prefix = " > "
			nameStyle = selectedItemStyle
		}

		status := helpStyle.Render("missing")
		if entry.cloned {
			status = installedStyle.Render("cloned ")
		}

		countStr := fmt.Sprintf("%6d", entry.itemCount)
		url := entry.url
		if len(url) > 40 {
			url = url[:37] + "..."
		}

		row := fmt.Sprintf("%s%-20s  %s  %s  %s",
			prefix,
			nameStyle.Render(truncate(entry.name, 20)),
			status,
			helpStyle.Render(countStr),
			helpStyle.Render(url),
		)
		s += zone.Mark(fmt.Sprintf("registry-row-%d", i), row) + "\n"
	}

	s += "\n"
	s += helpStyle.Render("up/down navigate • enter browse items • esc back") + "\n"
	s += helpStyle.Render("add: nesco registry add <url> • sync: nesco registry sync • remove: nesco registry remove <name>") + "\n"

	return s
}
