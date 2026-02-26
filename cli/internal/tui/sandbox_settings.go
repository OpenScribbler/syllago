package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/nesco/cli/internal/config"
)

// sandboxSettingsModel manages the sandbox configuration TUI.
type sandboxSettingsModel struct {
	repoRoot string
	sb       config.SandboxConfig

	cursor int
	// editMode 0=none, 1=add-domain, 2=add-env, 3=add-port
	editMode  int
	editInput string

	message    string
	messageErr bool
	width      int
	height     int
}

func newSandboxSettingsModel(repoRoot string) sandboxSettingsModel {
	cfg, err := config.Load(repoRoot)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	return sandboxSettingsModel{
		repoRoot: repoRoot,
		sb:       cfg.Sandbox,
	}
}

const (
	sandboxRowDomains = iota
	sandboxRowEnv
	sandboxRowPorts
	sandboxRowCount
)

func (m sandboxSettingsModel) Update(msg tea.Msg) (sandboxSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""

		if m.editMode != 0 {
			switch {
			case msg.Type == tea.KeyEsc:
				m.editMode = 0
				m.editInput = ""
			case msg.Type == tea.KeyEnter:
				m.commitEdit()
				m.editMode = 0
				m.editInput = ""
				m.save()
			case msg.Type == tea.KeyBackspace:
				if len(m.editInput) > 0 {
					m.editInput = m.editInput[:len(m.editInput)-1]
				}
			default:
				m.editInput += msg.String()
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < sandboxRowCount-1 {
				m.cursor++
			}
		case msg.Type == tea.KeyEnter, key.Matches(msg, keys.Space):
			m.editMode = m.cursor + 1
			m.editInput = ""
		case msg.Type == tea.KeyDelete, msg.String() == "d":
			m.deleteSelected()
			m.save()
		case key.Matches(msg, keys.Save):
			m.save()
		}
	}
	return m, nil
}

func (m *sandboxSettingsModel) commitEdit() {
	val := strings.TrimSpace(m.editInput)
	if val == "" {
		return
	}
	switch m.editMode {
	case 1: // domain
		m.sb.AllowedDomains = appendUniqueTUI(m.sb.AllowedDomains, val)
	case 2: // env
		m.sb.AllowedEnv = appendUniqueTUI(m.sb.AllowedEnv, val)
	case 3: // port
		if p, err := strconv.Atoi(val); err == nil {
			m.sb.AllowedPorts = appendUniqueIntTUI(m.sb.AllowedPorts, p)
		}
	}
}

func (m *sandboxSettingsModel) deleteSelected() {
	switch m.cursor {
	case sandboxRowDomains:
		if len(m.sb.AllowedDomains) > 0 {
			m.sb.AllowedDomains = m.sb.AllowedDomains[:len(m.sb.AllowedDomains)-1]
		}
	case sandboxRowEnv:
		if len(m.sb.AllowedEnv) > 0 {
			m.sb.AllowedEnv = m.sb.AllowedEnv[:len(m.sb.AllowedEnv)-1]
		}
	case sandboxRowPorts:
		if len(m.sb.AllowedPorts) > 0 {
			m.sb.AllowedPorts = m.sb.AllowedPorts[:len(m.sb.AllowedPorts)-1]
		}
	}
}

func (m *sandboxSettingsModel) save() {
	cfg, err := config.Load(m.repoRoot)
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	cfg.Sandbox = m.sb
	if err := config.Save(m.repoRoot, cfg); err != nil {
		m.message = fmt.Sprintf("Save failed: %s", err)
		m.messageErr = true
	} else {
		m.message = "Sandbox settings saved"
		m.messageErr = false
	}
}

func (m sandboxSettingsModel) View() string {
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	s := home + helpStyle.Render(" > ") + titleStyle.Render("Sandbox") + "\n\n"

	labels := []string{"Allowed Domains", "Allowed Env Vars", "Allowed Ports"}
	values := []string{
		sandboxListOrNone(m.sb.AllowedDomains),
		sandboxListOrNone(m.sb.AllowedEnv),
		sandboxPortsOrNone(m.sb.AllowedPorts),
	}

	for i := 0; i < sandboxRowCount; i++ {
		prefix := "   "
		style := itemStyle
		if i == m.cursor {
			prefix = " > "
			style = selectedItemStyle
		}
		row := fmt.Sprintf("%s%s  %s", prefix, style.Render(labels[i]), helpStyle.Render(values[i]))
		s += zone.Mark(fmt.Sprintf("sandbox-row-%d", i), row) + "\n"
	}

	if m.editMode != 0 {
		prompt := [...]string{"", "Add domain: ", "Add env var: ", "Add port: "}[m.editMode]
		s += "\n" + labelStyle.Render(prompt) + m.editInput + "_\n"
		s += helpStyle.Render("enter to save  |  esc cancel") + "\n"
	}

	if m.message != "" {
		s += "\n"
		if m.messageErr {
			s += errorMsgStyle.Render("Error: " + m.message)
		} else {
			s += successMsgStyle.Render("Done: " + m.message)
		}
		s += "\n"
	}

	s += "\n" + helpStyle.Render("up/down navigate  |  enter add  |  d delete last  |  s save  |  esc back")
	return s
}

func sandboxListOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func sandboxPortsOrNone(ports []int) string {
	if len(ports) == 0 {
		return "(none)"
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

func appendUniqueTUI(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func appendUniqueIntTUI(slice []int, item int) []int {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
