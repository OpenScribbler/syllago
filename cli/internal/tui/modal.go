package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

type envSetupStep int

const (
	envStepSelectType envSetupStep = iota
	envStepConfigure
	envStepConfirm
)

// envSetupModal is a multi-step wizard for configuring environment variables.
type envSetupModal struct {
	active   bool
	step     envSetupStep
	envTypes []string         // e.g. ["API_KEY", "AUTH_TOKEN"]
	cursor   int              // selected env type in step 1
	inputs   []textinput.Model // config fields in step 2
	inputIdx int              // focused input in step 2
	applied  bool
}

func newEnvSetupModal(envTypes []string) envSetupModal {
	return envSetupModal{
		active:   true,
		step:     envStepSelectType,
		envTypes: envTypes,
	}
}

func (m envSetupModal) Update(msg tea.Msg) (envSetupModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case envStepSelectType:
			switch msg.Type {
			case tea.KeyUp:
				if m.cursor > 0 {
					m.cursor--
				}
			case tea.KeyDown:
				if m.cursor < len(m.envTypes)-1 {
					m.cursor++
				}
			case tea.KeyEnter:
				// Build a single config input for the selected env var
				m.inputs = buildEnvInputs(m.envTypes[m.cursor])
				m.inputIdx = 0
				if len(m.inputs) > 0 {
					m.inputs[0].Focus()
				}
				m.step = envStepConfigure
			case tea.KeyEsc:
				m.active = false
			}
		case envStepConfigure:
			switch msg.Type {
			case tea.KeyEnter:
				m.step = envStepConfirm
				return m, nil
			case tea.KeyEsc:
				m.step = envStepSelectType
				return m, nil
			case tea.KeyTab:
				if len(m.inputs) > 1 {
					m.inputs[m.inputIdx].Blur()
					m.inputIdx = (m.inputIdx + 1) % len(m.inputs)
					m.inputs[m.inputIdx].Focus()
				}
			}
			if m.inputIdx < len(m.inputs) {
				var cmd tea.Cmd
				m.inputs[m.inputIdx], cmd = m.inputs[m.inputIdx].Update(msg)
				return m, cmd
			}
		case envStepConfirm:
			switch msg.Type {
			case tea.KeyEnter:
				m.applied = true
				m.active = false
			case tea.KeyEsc:
				m.step = envStepConfigure
			}
		}
	}
	return m, nil
}

func (m envSetupModal) View() string {
	if !m.active {
		return ""
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(44)

	var content string
	switch m.step {
	case envStepSelectType:
		content = labelStyle.Render("Step 1: Select environment variable") + "\n\n"
		for i, t := range m.envTypes {
			if i == m.cursor {
				content += selectedItemStyle.Render("▸ "+t) + "\n"
			} else {
				content += "  " + t + "\n"
			}
		}
		content += "\n" + helpStyle.Render("[↑↓] Navigate   [Enter] Select   [Esc] Cancel")
	case envStepConfigure:
		content = labelStyle.Render("Step 2: Configure "+m.envTypes[m.cursor]) + "\n\n"
		for _, inp := range m.inputs {
			content += inp.View() + "\n"
		}
		content += "\n" + helpStyle.Render("[Tab] Next field   [Enter] Continue   [Esc] Back")
	case envStepConfirm:
		content = labelStyle.Render("Step 3: Confirm") + "\n\n"
		val := ""
		if len(m.inputs) > 0 {
			val = m.inputs[0].Value()
		}
		content += valueStyle.Render("Set "+m.envTypes[m.cursor]+" = "+val) + "\n\n"
		content += helpStyle.Render("[Enter] Apply   [Esc] Back")
	}
	return modalStyle.Render(content)
}

func (m envSetupModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}

// buildEnvInputs returns a textinput model for entering a value for the given env var name.
func buildEnvInputs(envType string) []textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "value for " + envType
	ti.CharLimit = 200
	ti.Width = 36
	return []textinput.Model{ti}
}

// modalPurpose identifies what action the confirmModal is confirming.
type modalPurpose int

const (
	modalNone       modalPurpose = iota
	modalInstall
	modalUninstall
	modalSave
	modalPromote
	modalAppScript
)

// openModalMsg is sent by sub-models (e.g. detailModel) to ask App to open a modal.
type openModalMsg struct {
	purpose modalPurpose
	title   string
	body    string
}

// confirmModal is a centered confirmation dialog.
// It wraps bubbletea-overlay for positioning.
type confirmModal struct {
	title     string
	body      string // multi-line body text
	active    bool
	confirmed bool
	purpose   modalPurpose
}

func newConfirmModal(title, body string) confirmModal {
	return confirmModal{title: title, body: body, active: true}
}

func (m confirmModal) Update(msg tea.Msg) (confirmModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.confirmed = true
			m.active = false
		case tea.KeyEsc:
			m.confirmed = false
			m.active = false
		}
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.active = false
		case "n", "N":
			m.confirmed = false
			m.active = false
		}
	}
	return m, nil
}

func (m confirmModal) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(40)

	content := labelStyle.Render(m.title) + "\n\n"
	if m.body != "" {
		content += valueStyle.Render(m.body) + "\n\n"
	}
	content += helpStyle.Render("[Enter/y] Confirm   [Esc/n] Cancel")

	return modalStyle.Render(content)
}

// overlayView returns the modal centered over the given background content,
// using bubbletea-overlay for positioning.
func (m confirmModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}

// saveModal is a modal dialog with a text input for entering a filename.
type saveModal struct {
	active    bool
	input     textinput.Model
	confirmed bool
	value     string // set on confirm
}

func newSaveModal(placeholder string) saveModal {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 36
	return saveModal{active: true, input: ti}
}

func (m saveModal) Update(msg tea.Msg) (saveModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if strings.TrimSpace(m.input.Value()) != "" {
				m.value = strings.TrimSpace(m.input.Value())
				m.confirmed = true
				m.active = false
				return m, nil
			}
		case tea.KeyEsc:
			m.active = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m saveModal) View() string {
	if !m.active {
		return ""
	}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(40)
	content := labelStyle.Render("Save prompt as:") + "\n\n"
	content += m.input.View() + "\n\n"
	content += helpStyle.Render("[Enter] Save   [Esc] Cancel")
	return modalStyle.Render(content)
}

func (m saveModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}
