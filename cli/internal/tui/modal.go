package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

type envSetupStep int

const (
	envStepChoose   envSetupStep = iota // "Set up new value" or "Already configured"
	envStepValue                        // enter the value
	envStepLocation                     // enter save location (.env file path)
	envStepSource                       // enter path to existing .env file
)

// envSetupModal is a multi-step wizard for configuring environment variables.
// It walks through each unset env var, letting the user either enter a new value
// (and choose where to save it) or point to an existing .env file.
type envSetupModal struct {
	active       bool
	varNames     []string // ordered list of unset env var names
	varIdx       int      // current var being prompted
	step         envSetupStep
	methodCursor int             // 0=set up new, 1=already configured
	input        textinput.Model // shared text input for value/location/source
	value        string          // temporarily holds entered value between steps
	message      string          // feedback message after each operation
	messageIsErr bool
}

func newEnvSetupModal(envTypes []string) envSetupModal {
	ti := textinput.New()
	ti.CharLimit = 500
	ti.Width = 44
	return envSetupModal{
		active:   true,
		step:     envStepChoose,
		varNames: envTypes,
		input:    ti,
	}
}

// advance moves to the next unset env var, or closes the modal if done.
func (m *envSetupModal) advance() {
	m.varIdx++
	m.input.Blur()
	if m.varIdx >= len(m.varNames) {
		m.active = false
		return
	}
	m.step = envStepChoose
	m.methodCursor = 0
}

func (m envSetupModal) Update(msg tea.Msg) (envSetupModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear message on any keypress
		if m.message != "" {
			m.message = ""
			m.messageIsErr = false
		}

		switch m.step {
		case envStepChoose:
			switch {
			case msg.Type == tea.KeyEsc:
				m.advance() // skip this var
			case key.Matches(msg, keys.Up) || msg.Type == tea.KeyUp:
				if m.methodCursor > 0 {
					m.methodCursor--
				}
			case key.Matches(msg, keys.Down) || msg.Type == tea.KeyDown:
				if m.methodCursor < 1 {
					m.methodCursor++
				}
			case msg.Type == tea.KeyEnter:
				varName := m.varNames[m.varIdx]
				if m.methodCursor == 0 {
					// "Set up new value"
					m.step = envStepValue
					m.input.Prompt = labelStyle.Render(varName+": ") + " "
					m.input.Placeholder = "enter value"
					m.input.SetValue("")
					m.input.Focus()
				} else {
					// "Already configured"
					m.step = envStepSource
					m.input.Prompt = labelStyle.Render("Path to .env file: ") + " "
					m.input.Placeholder = "e.g. ~/.env or /path/to/.env"
					m.input.SetValue("")
					m.input.Focus()
				}
			}

		case envStepValue:
			switch msg.Type {
			case tea.KeyEsc:
				m.input.Blur()
				m.step = envStepChoose
				return m, nil
			case tea.KeyEnter:
				if m.input.Value() == "" {
					return m, nil
				}
				m.value = m.input.Value()
				m.step = envStepLocation
				home, err := os.UserHomeDir()
				if err != nil {
					m.message = "Cannot determine home directory"
					m.messageIsErr = true
					return m, nil
				}
				defaultPath := filepath.Join(home, ".config", "syllago", ".env")
				m.input.Prompt = labelStyle.Render("Save to: ") + " "
				m.input.Placeholder = defaultPath
				m.input.SetValue(defaultPath)
				m.input.Focus()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case envStepLocation:
			switch msg.Type {
			case tea.KeyEsc:
				// Back to value input
				m.step = envStepValue
				varName := m.varNames[m.varIdx]
				m.input.Prompt = labelStyle.Render(varName+": ") + " "
				m.input.Placeholder = "enter value"
				m.input.SetValue(m.value)
				m.input.Focus()
				return m, nil
			case tea.KeyEnter:
				savePath := m.input.Value()
				if savePath == "" {
					return m, nil
				}
				name := m.varNames[m.varIdx]
				if err := saveEnvToFile(name, m.value, savePath); err != nil {
					m.message = fmt.Sprintf("Failed to save %s: %s", name, err)
					m.messageIsErr = true
				} else {
					os.Setenv(name, m.value)
					m.message = fmt.Sprintf("Saved %s to %s", name, savePath)
					m.messageIsErr = false
				}
				m.value = ""
				m.advance()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}

		case envStepSource:
			switch msg.Type {
			case tea.KeyEsc:
				m.input.Blur()
				m.step = envStepChoose
				return m, nil
			case tea.KeyEnter:
				filePath := m.input.Value()
				if filePath == "" {
					return m, nil
				}
				name := m.varNames[m.varIdx]
				if err := loadEnvFromFile(name, filePath); err != nil {
					m.message = fmt.Sprintf("Could not load %s from %s: %s", name, filePath, err)
					m.messageIsErr = true
				} else {
					m.message = fmt.Sprintf("Loaded %s from %s", name, filePath)
					m.messageIsErr = false
				}
				m.advance()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m envSetupModal) View() string {
	if !m.active {
		return ""
	}

	const modalWidth = 56
	const modalHeight = 14

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	varName := m.varNames[m.varIdx]
	progress := fmt.Sprintf("(%d of %d)", m.varIdx+1, len(m.varNames))

	var content string

	switch m.step {
	case envStepChoose:
		content = labelStyle.Render("Environment Variable Setup") + "\n"
		content += helpStyle.Render(fmt.Sprintf("  %s %s", varName, progress)) + "\n\n"

		options := []string{"Set up new value", "I already have it configured"}
		for i, opt := range options {
			prefix := "  "
			style := itemStyle
			if i == m.methodCursor {
				prefix = "> "
				style = selectedItemStyle
			}
			content += fmt.Sprintf("  %s%s\n", prefix, style.Render(opt))
		}
		content += "\n" + helpStyle.Render("[↑↓] Navigate   [Enter] Select   [Esc] Skip")

	case envStepValue:
		content = labelStyle.Render("Environment Variable Setup") + "\n"
		content += helpStyle.Render(fmt.Sprintf("  %s %s", varName, progress)) + "\n\n"
		content += "  " + m.input.View() + "\n\n"
		content += helpStyle.Render("[Enter] Next   [Esc] Back")

	case envStepLocation:
		content = labelStyle.Render("Environment Variable Setup") + "\n"
		content += helpStyle.Render(fmt.Sprintf("  Save %s to:", varName)) + "\n\n"
		content += "  " + m.input.View() + "\n\n"
		content += helpStyle.Render("[Enter] Save   [Esc] Back")

	case envStepSource:
		content = labelStyle.Render("Environment Variable Setup") + "\n"
		content += helpStyle.Render(fmt.Sprintf("  Load %s from an existing file:", varName)) + "\n\n"
		content += "  " + m.input.View() + "\n\n"
		content += helpStyle.Render("[Enter] Load   [Esc] Back")
	}

	if m.message != "" {
		if m.messageIsErr {
			content += "\n" + errorMsgStyle.Render(m.message)
		} else {
			content += "\n" + successMsgStyle.Render(m.message)
		}
	}

	return modalStyle.Render(content)
}

func (m envSetupModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}

// modalPurpose identifies what action the confirmModal is confirming.
type modalPurpose int

const (
	modalNone modalPurpose = iota
	modalInstall
	modalUninstall
	modalSave
	modalPromote
	modalAppScript
	modalLoadoutApply
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

// ---------------------------------------------------------------------------
// Install modal — multi-step wizard for location → method → install
// ---------------------------------------------------------------------------

type installStep int

const (
	installStepLocation   installStep = iota // pick global/project/custom
	installStepCustomPath                    // text input for custom path
	installStepMethod                        // pick symlink/copy
)

// openInstallModalMsg is sent by detailModel to ask App to open the install modal.
type openInstallModalMsg struct {
	item      catalog.ContentItem
	providers []provider.Provider // checked (detected) providers to install to
	repoRoot  string
}

// installModal is a multi-step wizard for choosing install location and method.
type installModal struct {
	active    bool
	confirmed bool
	step      installStep

	// Context for rendering
	item      catalog.ContentItem
	providers []provider.Provider
	repoRoot  string

	// Location step (0=global, 1=project, 2=custom)
	locationCursor int

	// Custom path step
	customPathInput textinput.Model

	// Method step (0=symlink, 1=copy)
	methodCursor int
}

func newInstallModal(item catalog.ContentItem, providers []provider.Provider, repoRoot string) installModal {
	return installModal{
		active:    true,
		step:      installStepLocation,
		item:      item,
		providers: providers,
		repoRoot:  repoRoot,
	}
}

// LocationCursor returns the location choice (0=global, 1=project, 2=custom).
func (m installModal) LocationCursor() int { return m.locationCursor }

// MethodCursor returns the method choice (0=symlink, 1=copy).
func (m installModal) MethodCursor() int { return m.methodCursor }

// CustomPath returns the user-entered custom path (only relevant when locationCursor==2).
func (m installModal) CustomPath() string {
	return strings.TrimSpace(m.customPathInput.Value())
}

func (m installModal) Update(msg tea.Msg) (installModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if m.step == installStepLocation {
			for i := 0; i < 3; i++ {
				if zone.Get(fmt.Sprintf("install-loc-%d", i)).InBounds(msg) {
					m.locationCursor = i
					if i == 2 { // Custom
						m.customPathInput = textinput.New()
						m.customPathInput.Placeholder = "/path/to/install/dir"
						m.customPathInput.CharLimit = 200
						m.customPathInput.Width = 40
						m.customPathInput.Focus()
						m.step = installStepCustomPath
					} else {
						m.step = installStepMethod
						m.methodCursor = 0
					}
					return m, nil
				}
			}
		}
		if m.step == installStepMethod {
			for i := 0; i < 2; i++ {
				if zone.Get(fmt.Sprintf("install-method-%d", i)).InBounds(msg) {
					m.methodCursor = i
					m.confirmed = true
					m.active = false
					return m, nil
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Custom path text input captures all keys
		if m.step == installStepCustomPath {
			switch msg.Type {
			case tea.KeyEsc:
				m.step = installStepLocation
				m.customPathInput.Blur()
				return m, nil
			case tea.KeyEnter:
				if strings.TrimSpace(m.customPathInput.Value()) != "" {
					m.step = installStepMethod
					m.methodCursor = 0
					m.customPathInput.Blur()
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.customPathInput, cmd = m.customPathInput.Update(msg)
			return m, cmd
		}

		switch m.step {
		case installStepLocation:
			switch {
			case msg.Type == tea.KeyEsc:
				m.active = false
			case key.Matches(msg, keys.Up) || msg.Type == tea.KeyUp:
				if m.locationCursor > 0 {
					m.locationCursor--
				}
			case key.Matches(msg, keys.Down) || msg.Type == tea.KeyDown:
				if m.locationCursor < 2 {
					m.locationCursor++
				}
			case msg.Type == tea.KeyEnter || msg.String() == "i":
				if m.locationCursor == 2 { // Custom
					m.customPathInput = textinput.New()
					m.customPathInput.Placeholder = "/path/to/install/dir"
					m.customPathInput.CharLimit = 200
					m.customPathInput.Width = 40
					m.customPathInput.Focus()
					m.step = installStepCustomPath
				} else {
					m.step = installStepMethod
					m.methodCursor = 0
				}
			}

		case installStepMethod:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = installStepLocation
			case key.Matches(msg, keys.Up) || msg.Type == tea.KeyUp:
				if m.methodCursor > 0 {
					m.methodCursor--
				}
			case key.Matches(msg, keys.Down) || msg.Type == tea.KeyDown:
				if m.methodCursor < 1 {
					m.methodCursor++
				}
			case msg.Type == tea.KeyEnter || msg.String() == "i":
				m.confirmed = true
				m.active = false
			}
		}
	}
	return m, nil
}

func (m installModal) View() string {
	if !m.active {
		return ""
	}

	// Fixed dimensions prevent jitter when switching between steps.
	// The tallest step (location with 3 options + dest preview) determines the height.
	const modalWidth = 56
	const modalHeight = 18

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	var content string

	switch m.step {
	case installStepLocation:
		content = labelStyle.Render("Install Location") + "\n\n"

		type opt struct{ name, desc string }
		options := []opt{
			{"Global (home directory)", "~/.<provider>/ — available to all projects"},
			{"Project (current directory)", "./<provider>/ — only this project"},
			{"Custom path", "Enter a custom installation path"},
		}

		for i, o := range options {
			prefix := "  "
			nameStyle := itemStyle
			if i == m.locationCursor {
				prefix = "> "
				nameStyle = selectedItemStyle
			}
			row := fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(o.name))
			row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
			content += zone.Mark(fmt.Sprintf("install-loc-%d", i), row)
		}

		// Destination preview
		content += m.destinationPreview()

		content += "\n" + helpStyle.Render("[↑↓] Navigate   [Enter] Select   [Esc] Cancel")

	case installStepCustomPath:
		content = labelStyle.Render("Custom Install Path") + "\n\n"
		content += m.customPathInput.View() + "\n\n"
		content += helpStyle.Render("[Enter] Confirm   [Esc] Back")

	case installStepMethod:
		content = labelStyle.Render("Install Method") + "\n\n"

		type opt struct{ name, desc string }
		options := []opt{
			{"Symlink (recommended)", "Stays in sync with repo, auto-updates on pull"},
			{"Copy", "Independent copy, won't change"},
		}

		for i, o := range options {
			prefix := "  "
			nameStyle := itemStyle
			if i == m.methodCursor {
				prefix = "> "
				nameStyle = selectedItemStyle
			}
			row := fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(o.name))
			row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
			content += zone.Mark(fmt.Sprintf("install-method-%d", i), row)
		}

		// Destination paths
		content += m.destinationPreview()

		content += "\n" + helpStyle.Render("[↑↓] Navigate   [Enter] Install   [Esc] Back")
	}

	return modalStyle.Render(content)
}

// destinationPreview renders the destination paths for checked providers.
func (m installModal) destinationPreview() string {
	var baseDir string
	switch m.locationCursor {
	case 0:
		if home, err := os.UserHomeDir(); err == nil {
			baseDir = home
		}
	case 1:
		if cwd, err := os.Getwd(); err == nil {
			baseDir = cwd
		}
	case 2:
		baseDir = strings.TrimSpace(m.customPathInput.Value())
	}
	if baseDir == "" {
		return ""
	}

	s := "\n" + helpStyle.Render("Destination:") + "\n"
	for _, p := range m.providers {
		if installer.IsJSONMerge(p, m.item.Type) {
			s += "  " + helpStyle.Render(p.Name+": ") + valueStyle.Render("(merged into config)") + "\n"
		} else {
			destDir := p.InstallDir(baseDir, m.item.Type)
			dest := filepath.Join(destDir, m.item.Name)
			s += "  " + helpStyle.Render(p.Name+": ") + valueStyle.Render(dest) + "\n"
		}
	}
	return s
}

func (m installModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(m.View(), background, overlay.Center, overlay.Center, 0, 0)
}
