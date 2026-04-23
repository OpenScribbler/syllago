package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/doctor"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type systemMode int

const (
	systemModeDoctor    systemMode = iota
	systemModeProviders systemMode = iota
)

type systemModel struct {
	mode         systemMode
	checks       []doctor.CheckResult
	allProviders []provider.Provider
	selectedProv int
	loading      bool
	projectRoot  string
	width        int
	height       int
}

// systemLoadedMsg carries the result of an async doctor run + provider scan.
type systemLoadedMsg struct {
	checks       []doctor.CheckResult
	allProviders []provider.Provider
}

func newSystemModel(projectRoot string, width, height int) systemModel {
	return systemModel{
		projectRoot: projectRoot,
		width:       width,
		height:      height,
		loading:     true,
	}
}

func (m systemModel) Init() tea.Cmd {
	return m.loadCmd()
}

func (m systemModel) loadCmd() tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		result := doctor.Run(root)
		return systemLoadedMsg{
			checks:       result.Checks,
			allProviders: provider.AllProviders,
		}
	}
}

func (m systemModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.mode = (m.mode + 1) % 2
			m.selectedProv = 0
		case "shift+tab":
			m.mode = (m.mode - 1 + 2) % 2
			m.selectedProv = 0
		case "r":
			m.loading = true
			m.checks = nil
			return m, m.loadCmd()
		case "up", "k":
			if m.mode == systemModeProviders && m.selectedProv > 0 {
				m.selectedProv--
			}
		case "down", "j":
			if m.mode == systemModeProviders && m.selectedProv < len(m.allProviders)-1 {
				m.selectedProv++
			}
		}
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case systemLoadedMsg:
		m.loading = false
		m.checks = msg.checks
		m.allProviders = msg.allProviders
	}
	return m, nil
}

func (m systemModel) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Mode tab clicks
	if zone.Get("cfg-system-tab-doctor").InBounds(msg) {
		m.mode = systemModeDoctor
		m.selectedProv = 0
		return m, nil
	}
	if zone.Get("cfg-system-tab-providers").InBounds(msg) {
		m.mode = systemModeProviders
		m.selectedProv = 0
		return m, nil
	}
	// Provider list item clicks
	if m.mode == systemModeProviders {
		for i := range m.allProviders {
			if zone.Get(fmt.Sprintf("cfg-system-prov-%d", i)).InBounds(msg) {
				m.selectedProv = i
				return m, nil
			}
		}
	}
	return m, nil
}

func (m systemModel) View() string {
	innerW := m.width - 2
	innerH := m.height - 2

	// Tab row
	tabs := m.renderTabs(innerW)

	// Content
	var content string
	if m.mode == systemModeDoctor {
		content = m.renderDoctor(innerW, innerH-1)
	} else {
		content = m.renderProviders(innerW, innerH-1)
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabs, content)
}

func (m systemModel) renderTabs(innerW int) string {
	var doctorTab, provTab string
	if m.mode == systemModeDoctor {
		doctorTab = zone.Mark("cfg-system-tab-doctor", activeTabStyle.Render("Doctor"))
		provTab = zone.Mark("cfg-system-tab-providers", inactiveTabStyle.Render("Providers"))
	} else {
		doctorTab = zone.Mark("cfg-system-tab-doctor", inactiveTabStyle.Render("Doctor"))
		provTab = zone.Mark("cfg-system-tab-providers", activeTabStyle.Render("Providers"))
	}
	row := " " + doctorTab + " " + provTab
	pad := max(0, innerW-lipgloss.Width(row))
	return row + strings.Repeat(" ", pad)
}

func (m systemModel) renderDoctor(width, height int) string {
	if m.loading {
		return lipgloss.NewStyle().
			Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Running diagnostics...")
	}
	if len(m.checks) == 0 {
		return lipgloss.NewStyle().
			Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("No checks available")
	}

	var lines []string
	for _, c := range m.checks {
		var marker string
		switch c.Status {
		case doctor.CheckOK:
			marker = lipgloss.NewStyle().Foreground(successColor).Render("[ok]  ")
		case doctor.CheckWarn:
			marker = lipgloss.NewStyle().Foreground(warningColor).Render("[warn]")
		case doctor.CheckErr:
			marker = lipgloss.NewStyle().Foreground(dangerColor).Render("[err] ")
		}
		line := marker + " " + truncate(c.Message, width-10)
		lines = append(lines, line)
		for _, d := range c.Details {
			lines = append(lines, "       "+mutedStyle.Render(truncate(d, width-12)))
		}
	}

	// Pad to fill height
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m systemModel) renderProviders(width, height int) string {
	if m.loading {
		return lipgloss.NewStyle().
			Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Scanning providers...")
	}

	var lines []string
	for i, p := range m.allProviders {
		var status string
		if p.Detected {
			status = lipgloss.NewStyle().Foreground(successColor).Render("detected")
		} else {
			status = mutedStyle.Render("not detected")
		}
		name := truncate(p.Name, width-18)
		row := fmt.Sprintf("  %-30s %s", name, status)
		if i == m.selectedProv {
			row = selectedRowStyle.Render(truncate(row, width))
		}
		row = zone.Mark(fmt.Sprintf("cfg-system-prov-%d", i), row)
		lines = append(lines, row)
	}

	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

// SetSize updates the model dimensions.
func (m *systemModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
