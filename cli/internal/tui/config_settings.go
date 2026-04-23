package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/OpenScribbler/syllago/cli/internal/updater"
)

type settingsPanel int

const (
	settingsPanelConfig    settingsPanel = iota
	settingsPanelTelemetry settingsPanel = iota
	settingsPanelAbout     settingsPanel = iota
)

const settingsPanelCount = 3

type settingsModel struct {
	panel      settingsPanel
	cfg        *config.Config
	registries []string
	version    string
	width      int
	height     int

	// Telemetry state (populated by settingsTelemetryStatusMsg)
	telemetryEnabled bool
	telemetryAnonID  string

	// Update check state
	checkingUpdate bool
	latestVersion  string
	updateAvail    bool
}

// settingsTelemetryStatusMsg carries async telemetry status.
type settingsTelemetryStatusMsg struct {
	enabled bool
	anonID  string
}

// settingsUpdateCheckedMsg carries async update check result.
type settingsUpdateCheckedMsg struct {
	latestVersion string
	isNewer       bool
}

func newSettingsModel(cfg *config.Config, registries []string, version string, width, height int) settingsModel {
	return settingsModel{
		panel:      settingsPanelConfig,
		cfg:        cfg,
		registries: registries,
		version:    version,
		width:      width,
		height:     height,
	}
}

func (m settingsModel) Init() tea.Cmd {
	return m.loadTelemetryCmd()
}

func (m settingsModel) loadTelemetryCmd() tea.Cmd {
	return func() tea.Msg {
		s := telemetry.Status()
		return settingsTelemetryStatusMsg{enabled: s.Enabled, anonID: s.AnonymousID}
	}
}

func (m settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.panel = (m.panel + 1) % settingsPanelCount
		case "shift+tab":
			m.panel = (m.panel - 1 + settingsPanelCount) % settingsPanelCount
		case "t":
			if m.panel == settingsPanelTelemetry {
				newVal := !m.telemetryEnabled
				m.telemetryEnabled = newVal
				return m, m.saveTelemetryCmd(newVal)
			}
		case "r":
			if m.panel == settingsPanelTelemetry {
				return m, m.resetAnonIDCmd()
			}
		case "u":
			if m.panel == settingsPanelAbout {
				m.checkingUpdate = true
				return m, m.checkUpdateCmd()
			}
		}
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case settingsTelemetryStatusMsg:
		m.telemetryEnabled = msg.enabled
		m.telemetryAnonID = msg.anonID
	case settingsUpdateCheckedMsg:
		m.checkingUpdate = false
		m.latestVersion = msg.latestVersion
		m.updateAvail = msg.isNewer
	}
	return m, nil
}

func (m settingsModel) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Panel tab clicks
	for i := range settingsPanelCount {
		if zone.Get(fmt.Sprintf("cfg-settings-tab-%d", i)).InBounds(msg) {
			m.panel = settingsPanel(i)
			return m, nil
		}
	}
	// Telemetry toggle click
	if zone.Get("cfg-settings-telemetry-toggle").InBounds(msg) && m.panel == settingsPanelTelemetry {
		newVal := !m.telemetryEnabled
		m.telemetryEnabled = newVal
		return m, m.saveTelemetryCmd(newVal)
	}
	// Reset anon ID click
	if zone.Get("cfg-settings-telemetry-reset").InBounds(msg) && m.panel == settingsPanelTelemetry {
		return m, m.resetAnonIDCmd()
	}
	// Update check click
	if zone.Get("cfg-settings-update-check").InBounds(msg) && m.panel == settingsPanelAbout {
		m.checkingUpdate = true
		return m, m.checkUpdateCmd()
	}
	return m, nil
}

func (m settingsModel) saveTelemetryCmd(enabled bool) tea.Cmd {
	return func() tea.Msg {
		_ = telemetry.SetEnabled(enabled)
		s := telemetry.Status()
		return settingsTelemetryStatusMsg{enabled: s.Enabled, anonID: s.AnonymousID}
	}
}

func (m settingsModel) resetAnonIDCmd() tea.Cmd {
	return func() tea.Msg {
		newID, _ := telemetry.Reset()
		return settingsTelemetryStatusMsg{enabled: m.telemetryEnabled, anonID: newID}
	}
}

func (m settingsModel) checkUpdateCmd() tea.Cmd {
	ver := m.version
	return func() tea.Msg {
		info, err := updater.CheckLatest(ver)
		if err != nil {
			return settingsUpdateCheckedMsg{latestVersion: ver, isNewer: false}
		}
		return settingsUpdateCheckedMsg{latestVersion: info.Version, isNewer: info.UpdateAvail}
	}
}

func (m settingsModel) View() string {
	innerW := m.width - 2
	innerH := m.height - 2

	tabs := m.renderTabs(innerW)
	var content string
	switch m.panel {
	case settingsPanelConfig:
		content = m.renderConfigPanel(innerW, innerH-1)
	case settingsPanelTelemetry:
		content = m.renderTelemetryPanel(innerW, innerH-1)
	case settingsPanelAbout:
		content = m.renderAboutPanel(innerW, innerH-1)
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabs, content)
}

func (m settingsModel) renderTabs(innerW int) string {
	labels := []string{"Config", "Telemetry", "About"}
	var parts []string
	for i, label := range labels {
		var rendered string
		if settingsPanel(i) == m.panel {
			rendered = activeTabStyle.Render(label)
		} else {
			rendered = inactiveTabStyle.Render(label)
		}
		rendered = zone.Mark(fmt.Sprintf("cfg-settings-tab-%d", i), rendered)
		parts = append(parts, rendered)
	}
	row := " " + strings.Join(parts, " ")
	pad := max(0, innerW-lipgloss.Width(row))
	return row + strings.Repeat(" ", pad)
}

func (m settingsModel) renderConfigPanel(width, height int) string {
	var lines []string

	lines = append(lines, sectionTitleStyle.Render("Configured Registries"))
	if len(m.registries) == 0 {
		lines = append(lines, mutedStyle.Render("  No registries configured"))
	} else {
		for _, r := range m.registries {
			lines = append(lines, "  "+truncate(r, width-4))
		}
	}

	lines = append(lines, "")
	lines = append(lines, sectionTitleStyle.Render("Configured Providers"))
	if m.cfg == nil || len(m.cfg.Providers) == 0 {
		lines = append(lines, mutedStyle.Render("  No providers configured"))
	} else {
		for _, slug := range m.cfg.Providers {
			lines = append(lines, "  "+slug)
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m settingsModel) renderTelemetryPanel(width, height int) string {
	var lines []string

	var enabledLabel string
	if m.telemetryEnabled {
		enabledLabel = lipgloss.NewStyle().Foreground(successColor).Render("enabled")
	} else {
		enabledLabel = lipgloss.NewStyle().Foreground(dangerColor).Render("disabled")
	}
	toggleRow := fmt.Sprintf("  Telemetry:    %s", enabledLabel)
	toggleRow = zone.Mark("cfg-settings-telemetry-toggle", toggleRow)
	lines = append(lines, toggleRow)
	lines = append(lines, fmt.Sprintf("  Anonymous ID: %s", mutedStyle.Render(truncate(m.telemetryAnonID, width-18))))
	lines = append(lines, "")

	toggleBtn := activeButtonStyle.Render("[t] Toggle")
	toggleBtn = zone.Mark("cfg-settings-telemetry-toggle", toggleBtn)
	resetBtn := mutedStyle.Render("[r] Reset ID")
	resetBtn = zone.Mark("cfg-settings-telemetry-reset", resetBtn)
	lines = append(lines, "  "+toggleBtn+"   "+resetBtn)

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m settingsModel) renderAboutPanel(width, height int) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("  syllago %s", boldStyle.Render(m.version)))
	lines = append(lines, "")

	if m.checkingUpdate {
		lines = append(lines, mutedStyle.Render("  Checking for updates..."))
	} else if m.latestVersion != "" {
		if m.updateAvail {
			lines = append(lines, lipgloss.NewStyle().Foreground(warningColor).Render(
				fmt.Sprintf("  Update available: %s", m.latestVersion)))
		} else {
			lines = append(lines, lipgloss.NewStyle().Foreground(successColor).Render(
				"  You are up to date"))
		}
	}

	lines = append(lines, "")
	updateBtn := activeButtonStyle.Render("[u] Check for updates")
	updateBtn = zone.Mark("cfg-settings-update-check", updateBtn)
	lines = append(lines, "  "+updateBtn)

	_ = width
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

// SetSize updates the model dimensions.
func (m *settingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
