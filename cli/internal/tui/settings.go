package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// settingsEditMode represents which sub-picker is open.
type settingsEditMode int

const (
	editNone      settingsEditMode = iota
	editProviders                  // multi-select picker for providers
)

type settingsModel struct {
	repoRoot  string
	cfg       *config.Config
	providers []provider.Provider

	cursor   int              // main settings row cursor
	editMode settingsEditMode // which sub-picker is active
	subItems []settingsPickerItem
	subCur   int // cursor within sub-picker

	dirty      bool // true if any setting changed since load/save
	message    string
	messageErr bool
	width      int
	height     int
}

type settingsPickerItem struct {
	label   string
	checked bool
}

func newSettingsModel(repoRoot string, providers []provider.Provider) settingsModel {
	cfg, err := config.Load(repoRoot)
	if err != nil {
		cfg = &config.Config{}
	}
	return settingsModel{
		repoRoot:  repoRoot,
		cfg:       cfg,
		providers: providers,
	}
}

// settingsRowCount returns the number of configurable rows.
func (m settingsModel) settingsRowCount() int {
	return 3 // auto-update, providers, registry-auto-sync
}

func (m settingsModel) Update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if m.editMode != editNone {
				// Sub-picker: click toggles item
				for i := range m.subItems {
					if zone.Get(fmt.Sprintf("settings-sub-%d", i)).InBounds(msg) {
						m.subCur = i
						m.subItems[i].checked = !m.subItems[i].checked
						return m, nil
					}
				}
			} else {
				// Main rows: click activates
				for i := 0; i < m.settingsRowCount(); i++ {
					if zone.Get(fmt.Sprintf("settings-row-%d", i)).InBounds(msg) {
						m.cursor = i
						m.activateRow()
						return m, nil
					}
				}
			}
		}
		return m, nil
	case tea.KeyMsg:
		m.message = ""

		// Sub-picker active: handle sub-picker keys
		if m.editMode != editNone {
			switch {
			case msg.Type == tea.KeyEsc:
				m.applySubPicker()
				m.editMode = editNone
				m.subItems = nil
				m.subCur = 0
			case key.Matches(msg, keys.Up):
				if m.subCur > 0 {
					m.subCur--
				}
			case key.Matches(msg, keys.Down):
				if m.subCur < len(m.subItems)-1 {
					m.subCur++
				}
			case key.Matches(msg, keys.Space), msg.Type == tea.KeyEnter:
				if m.subCur < len(m.subItems) {
					m.subItems[m.subCur].checked = !m.subItems[m.subCur].checked
				}
			}
			return m, nil
		}

		// Main settings view
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
		case key.Matches(msg, keys.Save):
			m.save()
		}
	}
	return m, nil
}

// activateRow handles enter/space on the current row.
func (m *settingsModel) activateRow() {
	switch m.cursor {
	case 0: // auto-update toggle
		if m.cfg.Preferences == nil {
			m.cfg.Preferences = make(map[string]string)
		}
		if m.cfg.Preferences["autoUpdate"] == "true" {
			m.cfg.Preferences["autoUpdate"] = "false"
		} else {
			m.cfg.Preferences["autoUpdate"] = "true"
		}
		m.dirty = true
	case 1: // providers multi-select
		m.editMode = editProviders
		m.subCur = 0
		enabled := make(map[string]bool)
		for _, slug := range m.cfg.Providers {
			enabled[slug] = true
		}
		m.subItems = nil
		for _, p := range m.providers {
			m.subItems = append(m.subItems, settingsPickerItem{
				label:   p.Name + " (" + p.Slug + ")",
				checked: enabled[p.Slug],
			})
		}
	case 2: // registry auto-sync toggle
		if m.cfg.Preferences == nil {
			m.cfg.Preferences = make(map[string]string)
		}
		if m.cfg.Preferences["registryAutoSync"] == "true" {
			m.cfg.Preferences["registryAutoSync"] = "false"
		} else {
			m.cfg.Preferences["registryAutoSync"] = "true"
		}
		m.dirty = true
	}
}

// applySubPicker writes sub-picker state back to cfg.
func (m *settingsModel) applySubPicker() {
	switch m.editMode {
	case editProviders:
		var slugs []string
		for i, item := range m.subItems {
			if item.checked && i < len(m.providers) {
				slugs = append(slugs, m.providers[i].Slug)
			}
		}
		m.cfg.Providers = slugs
	}
	m.dirty = true
}

// save persists config to disk.
func (m *settingsModel) save() {
	if err := config.Save(m.repoRoot, m.cfg); err != nil {
		m.message = fmt.Sprintf("Save failed: %s", err)
		m.messageErr = true
	} else {
		m.message = "Settings saved"
		m.messageErr = false
	}
}

// settingsDescriptions maps cursor index to a description shown in the bottom detail area.
var settingsDescriptions = []string{
	"Pull updates automatically when a new version is detected on the remote.",
	"Providers are AI coding tools (Claude Code, Cursor, Gemini CLI, etc.).\nEnable the ones you use -- syllago imports their existing configs\nand can export your catalog items back to them.",
	"Sync git registries automatically when syllago launches (5-second timeout).\nRegistries must be added via `syllago registry add` first.",
}

func (m settingsModel) View() string {
	s := zone.Mark("crumb-home", helpStyle.Render("Home")) + " " + helpStyle.Render(">") + " " + titleStyle.Render("Settings") + "\n\n"

	// Row 0: Auto-update
	autoVal := "off"
	if m.cfg.Preferences["autoUpdate"] == "true" {
		autoVal = "on"
	}
	s += m.renderRow(0, "Auto-update", autoVal)

	// Row 1: Providers
	provVal := "(none)"
	if len(m.cfg.Providers) > 0 {
		provVal = strings.Join(m.cfg.Providers, ", ")
	}
	s += m.renderRow(1, "Providers", provVal)

	// Row 2: Registry auto-sync
	autoSyncVal := "off"
	if m.cfg.Preferences["registryAutoSync"] == "true" {
		autoSyncVal = "on"
	}
	s += m.renderRow(2, "Registry auto-sync", autoSyncVal)

	// Sub-picker overlay
	if m.editMode != editNone {
		s += "\n"
		s += labelStyle.Render("Select providers:") + "\n\n"

		for i, item := range m.subItems {
			prefix := "  "
			style := itemStyle
			if i == m.subCur {
				prefix = "> "
				style = selectedItemStyle
			}

			check := "[ ]"
			if item.checked {
				check = installedStyle.Render("[✓]")
			}

			row := fmt.Sprintf("  %s%s %s", prefix, check, style.Render(item.label))
			s += zone.Mark(fmt.Sprintf("settings-sub-%d", i), row) + "\n"
		}
	}

	// Status message
	if m.message != "" {
		s += "\n"
		if m.messageErr {
			s += errorMsgStyle.Render("Error: " + m.message)
		} else {
			s += successMsgStyle.Render("Done: " + m.message)
		}
		s += "\n"
	}

	// Bottom detail area (fixed 3-line height to prevent jitter)
	if m.editMode == editNone && m.cursor >= 0 && m.cursor < len(settingsDescriptions) {
		const detailLines = 3
		s += "\n " + helpStyle.Render(strings.Repeat("─", 45)) + "\n"
		lines := strings.Split(settingsDescriptions[m.cursor], "\n")
		for i := 0; i < detailLines; i++ {
			if i < len(lines) {
				s += " " + helpStyle.Render(lines[i]) + "\n"
			} else {
				s += "\n"
			}
		}
		s += " " + helpStyle.Render(strings.Repeat("─", 45)) + "\n"
	}

	return s
}

func (m settingsModel) renderRow(index int, label, value string) string {
	prefix := "   "
	style := itemStyle
	if index == m.cursor && m.editMode == editNone {
		prefix = " > "
		style = selectedItemStyle
	}
	row := fmt.Sprintf("%s%s  %s", prefix, style.Render(label), helpStyle.Render(value))
	return zone.Mark(fmt.Sprintf("settings-row-%d", index), row) + "\n"
}

func (m settingsModel) helpText() string {
	if m.editMode != editNone {
		return "up/down: navigate   Space: toggle   Esc: done"
	}
	return "up/down: navigate   Enter/Space: edit   s: save   Esc: back"
}

// HasPendingAction returns true if a sub-picker is open.
func (m settingsModel) HasPendingAction() bool {
	return m.editMode != editNone
}

// CancelAction closes any open sub-picker.
func (m *settingsModel) CancelAction() {
	m.applySubPicker()
	m.editMode = editNone
	m.subItems = nil
	m.subCur = 0
}
