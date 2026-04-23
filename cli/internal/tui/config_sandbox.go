package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/sandbox"
)

type sandboxTab int

const (
	sandboxTabDomains sandboxTab = iota
	sandboxTabPorts   sandboxTab = iota
	sandboxTabEnv     sandboxTab = iota
)

const sandboxTabCount = 3

type sandboxConfigModel struct {
	cfg          *config.Config
	tab          sandboxTab
	cursor       int
	showAddModal bool
	inputValue   string
	checkResult  sandbox.CheckResult
	width        int
	height       int
}

// sandboxCheckLoadedMsg carries the result of an async sandbox check.
type sandboxCheckLoadedMsg struct {
	result sandbox.CheckResult
}

// sandboxSavedMsg carries an updated config after a domain/port/env mutation.
type sandboxSavedMsg struct {
	cfg *config.Config
}

func newSandboxConfigModel(cfg *config.Config, width, height int) sandboxConfigModel {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return sandboxConfigModel{
		cfg:    cfg,
		width:  width,
		height: height,
	}
}

func (m sandboxConfigModel) Init() tea.Cmd {
	return m.loadCheckCmd()
}

func (m sandboxConfigModel) loadCheckCmd() tea.Cmd {
	return func() tea.Msg {
		result := sandbox.Check("", "", "")
		return sandboxCheckLoadedMsg{result: result}
	}
}

func (m sandboxConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showAddModal {
			return m.updateModal(msg)
		}
		switch msg.String() {
		case "tab":
			m.tab = (m.tab + 1) % sandboxTabCount
			m.cursor = 0
		case "shift+tab":
			m.tab = (m.tab - 1 + sandboxTabCount) % sandboxTabCount
			m.cursor = 0
		case "a":
			m.showAddModal = true
			m.inputValue = ""
		case "d":
			return m, m.deleteSelected()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.currentListLen()-1 {
				m.cursor++
			}
		}
	case sandboxCheckLoadedMsg:
		m.checkResult = msg.result
	case sandboxSavedMsg:
		m.cfg = msg.cfg
	}
	return m, nil
}

// updateModal handles keypresses when the add modal is open.
func (m sandboxConfigModel) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showAddModal = false
		m.inputValue = ""
	case "enter":
		if m.inputValue != "" {
			cmd := m.addEntryCmd(m.inputValue)
			m.showAddModal = false
			m.inputValue = ""
			return m, cmd
		}
	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
	default:
		if len(msg.Runes) > 0 {
			m.inputValue += string(msg.Runes)
		}
	}
	return m, nil
}

// currentListLen returns the length of the currently displayed list.
func (m sandboxConfigModel) currentListLen() int {
	switch m.tab {
	case sandboxTabDomains:
		return len(m.cfg.Sandbox.AllowedDomains)
	case sandboxTabPorts:
		return len(m.cfg.Sandbox.AllowedPorts)
	case sandboxTabEnv:
		return len(m.cfg.Sandbox.AllowedEnv)
	}
	return 0
}

// deleteSelected removes the item at cursor from the current tab's list and emits a save cmd.
func (m sandboxConfigModel) deleteSelected() tea.Cmd {
	if m.currentListLen() == 0 {
		return nil
	}
	cur := m.cursor
	newCfg := *m.cfg
	newSandbox := m.cfg.Sandbox
	switch m.tab {
	case sandboxTabDomains:
		domains := make([]string, 0, len(newSandbox.AllowedDomains)-1)
		for i, d := range newSandbox.AllowedDomains {
			if i != cur {
				domains = append(domains, d)
			}
		}
		newSandbox.AllowedDomains = domains
	case sandboxTabPorts:
		ports := make([]int, 0, len(newSandbox.AllowedPorts)-1)
		for i, p := range newSandbox.AllowedPorts {
			if i != cur {
				ports = append(ports, p)
			}
		}
		newSandbox.AllowedPorts = ports
	case sandboxTabEnv:
		envs := make([]string, 0, len(newSandbox.AllowedEnv)-1)
		for i, e := range newSandbox.AllowedEnv {
			if i != cur {
				envs = append(envs, e)
			}
		}
		newSandbox.AllowedEnv = envs
	}
	newCfg.Sandbox = newSandbox
	return func() tea.Msg { return sandboxSavedMsg{cfg: &newCfg} }
}

// addEntryCmd appends the entered value to the current tab's list.
func (m sandboxConfigModel) addEntryCmd(value string) tea.Cmd {
	newCfg := *m.cfg
	newSandbox := m.cfg.Sandbox
	switch m.tab {
	case sandboxTabDomains:
		newSandbox.AllowedDomains = appendUniqueSandbox(newSandbox.AllowedDomains, value)
	case sandboxTabPorts:
		// For ports tab, we accept numeric strings but skip invalid input
		var port int
		if _, err := fmt.Sscan(value, &port); err == nil {
			newSandbox.AllowedPorts = appendUniquePortSandbox(newSandbox.AllowedPorts, port)
		}
	case sandboxTabEnv:
		newSandbox.AllowedEnv = appendUniqueSandbox(newSandbox.AllowedEnv, value)
	}
	newCfg.Sandbox = newSandbox
	return func() tea.Msg { return sandboxSavedMsg{cfg: &newCfg} }
}

func appendUniqueSandbox(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func appendUniquePortSandbox(slice []int, item int) []int {
	for _, p := range slice {
		if p == item {
			return slice
		}
	}
	return append(slice, item)
}

func (m sandboxConfigModel) View() string {
	innerW := m.width - 2
	innerH := m.height - 2

	// Status line
	statusLine := m.renderStatus(innerW)

	// Tab row
	tabs := m.renderTabs(innerW)

	// List content
	listH := max(0, innerH-2)
	content := m.renderList(innerW, listH)

	body := lipgloss.JoinVertical(lipgloss.Left, statusLine, tabs, content)

	if m.showAddModal {
		return overlayModal(body, m.renderAddModal(innerW), m.width, m.height)
	}
	return body
}

func (m sandboxConfigModel) renderStatus(width int) string {
	var parts []string
	if m.checkResult.BwrapOK {
		parts = append(parts, lipgloss.NewStyle().Foreground(successColor).Render("bwrap [ok]"))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(dangerColor).Render("bwrap [err]"))
	}
	if m.checkResult.SocatOK {
		parts = append(parts, lipgloss.NewStyle().Foreground(successColor).Render("socat [ok]"))
	} else {
		parts = append(parts, lipgloss.NewStyle().Foreground(dangerColor).Render("socat [err]"))
	}
	line := " " + strings.Join(parts, "  ")
	pad := max(0, width-lipgloss.Width(line))
	return line + strings.Repeat(" ", pad)
}

func (m sandboxConfigModel) renderTabs(width int) string {
	labels := []string{"Domains", "Ports", "Env"}
	var parts []string
	for i, label := range labels {
		if sandboxTab(i) == m.tab {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
	}
	row := " " + strings.Join(parts, " ")
	pad := max(0, width-lipgloss.Width(row))
	return row + strings.Repeat(" ", pad)
}

func (m sandboxConfigModel) renderList(width, height int) string {
	var items []string
	var hint string

	switch m.tab {
	case sandboxTabDomains:
		if len(m.cfg.Sandbox.AllowedDomains) == 0 {
			items = append(items, mutedStyle.Render("  No domains configured (defaults apply)"))
		} else {
			for i, d := range m.cfg.Sandbox.AllowedDomains {
				row := "  " + truncate(d, width-4)
				if i == m.cursor {
					row = selectedRowStyle.Render(truncate(row, width))
				}
				items = append(items, row)
			}
		}
		hint = mutedStyle.Render("  [a] Add domain   [d] Remove")
	case sandboxTabPorts:
		if len(m.cfg.Sandbox.AllowedPorts) == 0 {
			items = append(items, mutedStyle.Render("  No ports configured (defaults apply)"))
		} else {
			for i, p := range m.cfg.Sandbox.AllowedPorts {
				row := fmt.Sprintf("  %d", p)
				if i == m.cursor {
					row = selectedRowStyle.Render(truncate(row, width))
				}
				items = append(items, row)
			}
		}
		hint = mutedStyle.Render("  [a] Add port   [d] Remove")
	case sandboxTabEnv:
		if len(m.cfg.Sandbox.AllowedEnv) == 0 {
			items = append(items, mutedStyle.Render("  No env vars configured (defaults apply)"))
		} else {
			for i, e := range m.cfg.Sandbox.AllowedEnv {
				row := "  " + truncate(e, width-4)
				if i == m.cursor {
					row = selectedRowStyle.Render(truncate(row, width))
				}
				items = append(items, row)
			}
		}
		hint = mutedStyle.Render("  [a] Add env var   [d] Remove")
	}

	// Pad to fill height, leaving 1 row for hints
	hintH := 1
	listH := max(0, height-hintH)
	for len(items) < listH {
		items = append(items, "")
	}

	lines := items[:min(len(items), listH)]
	lines = append(lines, hint)
	return strings.Join(lines, "\n")
}

func (m sandboxConfigModel) renderAddModal(width int) string {
	modalW := min(width-4, 50)
	innerW := modalW - 4

	var prompt string
	switch m.tab {
	case sandboxTabDomains:
		prompt = "Add domain:"
	case sandboxTabPorts:
		prompt = "Add port:"
	case sandboxTabEnv:
		prompt = "Add env var:"
	}

	inputLine := m.inputValue + "_"
	inputRendered := lipgloss.NewStyle().
		Foreground(primaryColor).
		Render(truncate(inputLine, innerW))

	top := "╭" + strings.Repeat("─", modalW-2) + "╮"
	mid1 := "│  " + boldStyle.Render(truncate(prompt, innerW)) + strings.Repeat(" ", max(0, innerW-len(prompt))) + "  │"
	mid2 := "│  " + inputRendered + strings.Repeat(" ", max(0, innerW-lipgloss.Width(inputRendered)+1)) + "  │"
	mid3 := "│  " + mutedStyle.Render("[Enter] Add   [Esc] Cancel") + strings.Repeat(" ", max(0, innerW-26)) + "  │"
	bot := "╰" + strings.Repeat("─", modalW-2) + "╯"

	return strings.Join([]string{top, mid1, mid2, mid3, bot}, "\n")
}

// SetSize updates the model dimensions.
func (m *sandboxConfigModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
