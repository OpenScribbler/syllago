package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

type settingsModel struct {
	repoRoot string
	cfg      *config.Config

	cursor       int // main settings row cursor
	message      string
	messageIsErr bool
	width        int
	height       int
}

func newSettingsModel(repoRoot string) settingsModel {
	cfg, err := config.Load(repoRoot)
	if err != nil {
		cfg = &config.Config{}
	}
	return settingsModel{
		repoRoot: repoRoot,
		cfg:      cfg,
	}
}

// settingsRowCount returns the number of configurable rows.
func (m settingsModel) settingsRowCount() int {
	return 3 // update-check, auto-update, registry-auto-sync
}

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			for i := 0; i < m.settingsRowCount(); i++ {
				if zone.Get(fmt.Sprintf("settings-row-%d", i)).InBounds(msg) {
					m.cursor = i
					m.activateRow()
					return m, nil
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		m.message = ""

		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < m.settingsRowCount()-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Space), msg.Type == tea.KeyEnter:
			m.activateRow()
		}
	}
	return m, nil
}

// activateRow handles enter/space on the current row.
func (m *settingsModel) activateRow() {
	if m.cfg.Preferences == nil {
		m.cfg.Preferences = make(map[string]string)
	}
	switch m.cursor {
	case 0: // update check toggle (defaults to on)
		if m.cfg.Preferences["updateCheck"] == "false" {
			delete(m.cfg.Preferences, "updateCheck") // absent = on (default)
		} else {
			m.cfg.Preferences["updateCheck"] = "false"
		}
	case 1: // auto-update toggle
		if m.cfg.Preferences["autoUpdate"] == "true" {
			m.cfg.Preferences["autoUpdate"] = "false"
		} else {
			m.cfg.Preferences["autoUpdate"] = "true"
		}
	case 2: // registry auto-sync toggle
		if m.cfg.Preferences["registryAutoSync"] == "true" {
			m.cfg.Preferences["registryAutoSync"] = "false"
		} else {
			m.cfg.Preferences["registryAutoSync"] = "true"
		}
	}
	m.save()
}

// save persists config to disk.
func (m *settingsModel) save() {
	if err := config.Save(m.repoRoot, m.cfg); err != nil {
		m.message = fmt.Sprintf("Save failed: %s", err)
		m.messageIsErr = true
	} else {
		m.message = "Settings saved"
		m.messageIsErr = false
	}
}

// settingsDescriptions maps cursor index to a description shown in the bottom detail area.
var settingsDescriptions = []string{
	"Check for new versions on launch. Requires network access to the git remote.\nDisable for air-gapped or offline environments.",
	"Pull updates automatically when a new version is detected on the remote.",
	"Sync git registries automatically when syllago launches (5-second timeout).\nRegistries must be added via `syllago registry add` first.",
}

func (m settingsModel) View() string {
	s := renderBreadcrumb(
		BreadcrumbSegment{"Home", "crumb-home"},
		BreadcrumbSegment{"Settings", ""},
	) + "\n\n"

	// Row 0: Update check (defaults to on)
	updateCheckVal := "on"
	if m.cfg.Preferences["updateCheck"] == "false" {
		updateCheckVal = "off"
	}
	s += m.renderRow(0, "Update check", updateCheckVal)

	// Row 1: Auto-update
	autoVal := "off"
	if m.cfg.Preferences["autoUpdate"] == "true" {
		autoVal = "on"
	}
	s += m.renderRow(1, "Auto-update", autoVal)

	// Row 2: Registry auto-sync
	autoSyncVal := "off"
	if m.cfg.Preferences["registryAutoSync"] == "true" {
		autoSyncVal = "on"
	}
	s += m.renderRow(2, "Registry auto-sync", autoSyncVal)

	// Bottom detail area (fixed 3-line height to prevent jitter)
	if m.cursor >= 0 && m.cursor < len(settingsDescriptions) {
		s += renderDescriptionBox(settingsDescriptions[m.cursor], 45, 3)
	}

	return s
}

func (m settingsModel) renderRow(index int, label, value string) string {
	prefix := "  "
	style := itemStyle
	if index == m.cursor {
		prefix = "> "
		style = selectedItemStyle
	}
	row := fmt.Sprintf("  %s%s  %s", prefix, style.Render(label), helpStyle.Render(value))
	return zone.Mark(fmt.Sprintf("settings-row-%d", index), row) + "\n"
}

func (m settingsModel) helpText() string {
	return "up/down navigate • enter/space toggle • esc back"
}
