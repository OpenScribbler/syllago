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
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}

// renderButtons renders a pair of centered action buttons with a cursor indicator.
// cursor 0 = left button active, cursor 1 = right button active.
// contentWidth is the available width for centering (0 = no centering).
func renderButtons(left, right string, cursor, contentWidth int) string {
	var l, r string
	if cursor == 0 {
		l = "▸ " + buttonStyle.Render(left)
		r = "  " + buttonDisabledStyle.Render(right)
	} else {
		l = "  " + buttonDisabledStyle.Render(left)
		r = "▸ " + buttonStyle.Render(right)
	}
	bar := l + "   " + r
	if contentWidth > 0 {
		bar = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, bar)
	}
	return bar
}

// modalPurpose identifies what action the confirmModal is confirming.
type modalPurpose int

const (
	modalNone modalPurpose = iota
	modalInstall
	modalUninstall
	modalSave
	modalShare
	modalAppScript
	modalLoadoutApply
	modalHookBrokenWarning
	modalRegistryRemove
	modalNonSyllagoRedirect
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
	btnCursor int // 0=Confirm, 1=Cancel (default 1)
}

func newConfirmModal(title, body string) confirmModal {
	return confirmModal{title: title, body: body, active: true, btnCursor: 1}
}

func (m confirmModal) Update(msg tea.Msg) (confirmModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.confirmed = m.btnCursor == 0
			m.active = false
		case tea.KeyEsc:
			m.confirmed = false
			m.active = false
		case tea.KeyLeft:
			if m.btnCursor > 0 {
				m.btnCursor--
			}
		case tea.KeyRight:
			if m.btnCursor < 1 {
				m.btnCursor++
			}
		}
		switch {
		case key.Matches(msg, keys.ConfirmYes):
			m.confirmed = true
			m.active = false
		case key.Matches(msg, keys.ConfirmNo):
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

	const modalWidth = 40
	const modalHeight = 10
	const innerHeight = modalHeight - 2

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	content := labelStyle.Render(m.title) + "\n\n"
	if m.body != "" {
		content += valueStyle.Render(m.body) + "\n\n"
	}

	buttons := renderButtons("Confirm", "Cancel", m.btnCursor, 36)

	// Pin buttons to bottom
	contentLines := strings.Count(content, "\n")
	spacer := innerHeight - contentLines - 1
	if spacer < 0 {
		spacer = 0
	}
	content += strings.Repeat("\n", spacer) + buttons

	return modalStyle.Render(content)
}

// overlayView returns the modal centered over the given background content,
// using bubbletea-overlay for positioning.
func (m confirmModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
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
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
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
	btnCursor int // 0=Select/Install, 1=Cancel (default 0)

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

// symlinkDisabled returns true if any checked provider explicitly marks the
// item's content type as not supporting symlinks. A nil map or absent key
// means "assumed supported" (per Provider.SymlinkSupport docs).
func (m installModal) symlinkDisabled() bool {
	for _, p := range m.providers {
		if supported, ok := p.SymlinkSupport[m.item.Type]; ok && !supported {
			return true
		}
	}
	return false
}

// defaultMethodCursor returns the cursor position to use when entering the
// method step: 0 (symlink) normally, 1 (copy) when symlink is disabled.
func (m installModal) defaultMethodCursor() int {
	if m.symlinkDisabled() {
		return 1
	}
	return 0
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
					m.methodCursor = m.defaultMethodCursor()
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
			case msg.Type == tea.KeyLeft:
				if m.btnCursor > 0 {
					m.btnCursor--
				}
			case msg.Type == tea.KeyRight:
				if m.btnCursor < 1 {
					m.btnCursor++
				}
			case key.Matches(msg, keys.Up) || msg.Type == tea.KeyUp:
				if m.locationCursor > 0 {
					m.locationCursor--
				}
			case key.Matches(msg, keys.Down) || msg.Type == tea.KeyDown:
				if m.locationCursor < 2 {
					m.locationCursor++
				}
			case msg.Type == tea.KeyEnter:
				if m.btnCursor == 1 { // Cancel
					m.active = false
					return m, nil
				}
				if m.locationCursor == 2 { // Custom
					m.customPathInput = textinput.New()
					m.customPathInput.Placeholder = "/path/to/install/dir"
					m.customPathInput.CharLimit = 200
					m.customPathInput.Width = 40
					m.customPathInput.Focus()
					m.step = installStepCustomPath
				} else {
					m.step = installStepMethod
					m.methodCursor = m.defaultMethodCursor()
					m.btnCursor = 0
				}
			}

		case installStepMethod:
			switch {
			case msg.Type == tea.KeyEsc:
				m.step = installStepLocation
				m.btnCursor = 0
			case msg.Type == tea.KeyLeft:
				if m.btnCursor > 0 {
					m.btnCursor--
				}
			case msg.Type == tea.KeyRight:
				if m.btnCursor < 1 {
					m.btnCursor++
				}
			case key.Matches(msg, keys.Up) || msg.Type == tea.KeyUp:
				// Don't navigate to symlink (0) when it's disabled for this content type
				if m.methodCursor > 0 && !m.symlinkDisabled() {
					m.methodCursor--
				}
			case key.Matches(msg, keys.Down) || msg.Type == tea.KeyDown:
				if m.methodCursor < 1 {
					m.methodCursor++
				}
			case msg.Type == tea.KeyEnter:
				if m.btnCursor == 1 { // Cancel/Back
					m.step = installStepLocation
					m.btnCursor = 0
					return m, nil
				}
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

	// Fixed dimensions — every step uses the same box size to prevent jitter.
	const modalWidth = 56
	const modalHeight = 18
	// Inner height = modalHeight - 2 (1 top + 1 bottom padding)
	const innerHeight = modalHeight - 2

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	var content string
	var buttons string

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
			content += row
		}

		content += m.destinationPreview()

		buttons = renderButtons("Select", "Cancel", m.btnCursor, 52)

	case installStepCustomPath:
		content = labelStyle.Render("Custom Install Path") + "\n\n"
		content += m.customPathInput.View() + "\n\n"
		buttons = renderButtons("Confirm", "Back", m.btnCursor, 52)

	case installStepMethod:
		content = labelStyle.Render("Install Method") + "\n\n"

		symlinkOff := m.symlinkDisabled()
		type opt struct{ name, desc string }
		options := []opt{
			{"Symlink (recommended)", "Stays in sync with repo, auto-updates on pull"},
			{"Copy", "Independent copy, won't change"},
		}

		for i, o := range options {
			if i == 0 && symlinkOff {
				// Symlink disabled — render as muted, non-selectable
				row := fmt.Sprintf("    %s\n", helpStyle.Render(o.name+" (not supported for this content type)"))
				row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
				content += row
				continue
			}
			prefix := "  "
			nameStyle := itemStyle
			if i == m.methodCursor {
				prefix = "> "
				nameStyle = selectedItemStyle
			}
			row := fmt.Sprintf("  %s%s\n", prefix, nameStyle.Render(o.name))
			row += fmt.Sprintf("      %s\n", helpStyle.Render(o.desc))
			content += row
		}

		content += m.destinationPreview()

		buttons = renderButtons("Install", "Back", m.btnCursor, 52)
	}

	// Pin buttons to bottom: fill remaining space with blank lines
	contentLines := strings.Count(content, "\n")
	spacer := innerHeight - contentLines - 1
	if spacer < 0 {
		spacer = 0
	}
	content += strings.Repeat("\n", spacer) + buttons

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

	// Content area is modalWidth(56) - 4(padding) = 52 chars.
	// Truncate paths to prevent terminal wrapping which breaks layout.
	const maxLineWidth = 52

	s := "\n" + helpStyle.Render("Destination:") + "\n"
	for _, p := range m.providers {
		prefix := "  " + p.Name + ": "
		if installer.IsJSONMerge(p, m.item.Type) {
			s += helpStyle.Render(prefix) + valueStyle.Render("(merged into config)") + "\n"
		} else {
			destDir := p.InstallDir(baseDir, m.item.Type)
			dest := filepath.Join(destDir, m.item.Name)
			// Truncate path if it would cause the line to wrap
			maxPath := maxLineWidth - len(prefix)
			if maxPath < 10 {
				maxPath = 10
			}
			if len(dest) > maxPath {
				dest = "…" + dest[len(dest)-maxPath+1:]
			}
			s += helpStyle.Render(prefix) + valueStyle.Render(dest) + "\n"
		}
	}
	return s
}

func (m installModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}

// ──────────────────────────────────────────────────────────
// Registry Add Modal
// ──────────────────────────────────────────────────────────

// registryAddModal is a single-step modal for entering a git URL to add as a registry.
type registryAddModal struct {
	active       bool
	confirmed    bool
	urlInput     textinput.Model
	nameInput    textinput.Model
	focusedField int // 0 = url, 1 = name (optional override)
	btnCursor    int // 0 = Add, 1 = Cancel
	message      string
	messageIsErr bool
}

const (
	registryAddModalWidth  = 56
	registryAddModalHeight = 14
	registryAddInnerHeight = registryAddModalHeight - 2
)

func newRegistryAddModal() registryAddModal {
	ui := textinput.New()
	ui.Prompt = labelStyle.Render("URL: ")
	ui.Placeholder = "https://github.com/owner/repo"
	ui.CharLimit = 500
	ui.Width = 40
	ui.Focus()

	ni := textinput.New()
	ni.Prompt = labelStyle.Render("Name: ")
	ni.Placeholder = "optional — auto-derived from URL"
	ni.CharLimit = 100
	ni.Width = 40

	return registryAddModal{
		active:   true,
		urlInput: ui,
		nameInput: ni,
	}
}

func (m registryAddModal) Update(msg tea.Msg) (registryAddModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""
		switch {
		case msg.Type == tea.KeyEsc:
			m.active = false
			return m, nil
		case msg.Type == tea.KeyTab:
			if m.focusedField == 0 {
				m.focusedField = 1
				m.urlInput.Blur()
				m.nameInput.Focus()
			} else {
				m.focusedField = 0
				m.nameInput.Blur()
				m.urlInput.Focus()
			}
			return m, nil
		case msg.Type == tea.KeyLeft:
			if m.btnCursor > 0 {
				m.btnCursor--
			}
			return m, nil
		case msg.Type == tea.KeyRight:
			if m.btnCursor < 1 {
				m.btnCursor++
			}
			return m, nil
		case msg.Type == tea.KeyEnter:
			if m.btnCursor == 1 { // Cancel
				m.active = false
				return m, nil
			}
			url := strings.TrimSpace(m.urlInput.Value())
			if url == "" {
				m.message = "URL is required"
				m.messageIsErr = true
				return m, nil
			}
			m.confirmed = true
			m.active = false
			return m, nil
		}
		var cmd tea.Cmd
		if m.focusedField == 0 {
			m.urlInput, cmd = m.urlInput.Update(msg)
		} else {
			m.nameInput, cmd = m.nameInput.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m registryAddModal) View() string {
	if !m.active {
		return ""
	}

	content := labelStyle.Render("Add Registry") + "\n\n"
	content += m.urlInput.View() + "\n"
	content += m.nameInput.View()
	if m.message != "" && m.messageIsErr {
		content += "\n" + errorMsgStyle.Render(m.message)
	}
	content += "\n" + helpStyle.Render("tab switch field")

	buttons := renderButtons("Add", "Cancel", m.btnCursor, 52)

	contentLines := strings.Count(content, "\n")
	spacer := registryAddInnerHeight - contentLines - 1
	if spacer < 0 {
		spacer = 0
	}
	content += strings.Repeat("\n", spacer) + buttons

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(modalBorderColor).
		Background(modalBgColor).
		Padding(1, 2).
		Width(registryAddModalWidth).
		Height(registryAddModalHeight).
		Render(content)
}

func (m registryAddModal) overlayView(background string) string {
	if !m.active {
		return background
	}
	return overlay.Composite(zone.Mark("modal-zone", m.View()), background, overlay.Center, overlay.Center, 0, 0)
}
