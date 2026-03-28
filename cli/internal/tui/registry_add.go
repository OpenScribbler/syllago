package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

var validBranchRe = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

type registryAddMsg struct {
	url     string
	name    string
	ref     string
	isLocal bool
}

type registryAddModal struct {
	active          bool
	width           int
	height          int
	sourceGit       bool
	urlValue        string
	nameValue       string
	branchValue     string
	cursor          int
	focusIdx        int
	nameManuallySet bool
	err             string
	existingNames   []string
	cfg             *config.Config
}

func newRegistryAddModal() registryAddModal {
	return registryAddModal{sourceGit: true}
}

func (m *registryAddModal) Open(existingNames []string, cfg *config.Config) {
	m.active = true
	m.sourceGit = true
	m.focusIdx = 1
	m.urlValue = ""
	m.nameValue = ""
	m.branchValue = ""
	m.cursor = 0
	m.nameManuallySet = false
	m.err = ""
	m.existingNames = existingNames
	m.cfg = cfg
}

func (m *registryAddModal) Close() {
	m.active = false
	m.sourceGit = false
	m.urlValue = ""
	m.nameValue = ""
	m.branchValue = ""
	m.cursor = 0
	m.focusIdx = 0
	m.nameManuallySet = false
	m.err = ""
	m.existingNames = nil
	m.cfg = nil
}

func (m *registryAddModal) focusedValue() *string {
	switch m.focusIdx {
	case 1:
		return &m.urlValue
	case 2:
		return &m.nameValue
	case 3:
		if m.sourceGit {
			return &m.branchValue
		}
		return nil
	default:
		return nil
	}
}

func (m registryAddModal) isTextField() bool {
	switch m.focusIdx {
	case 1, 2:
		return true
	case 3:
		return m.sourceGit
	default:
		return false
	}
}

func (m registryAddModal) nextFocusIdx(current, dir int) int {
	const maxIdx = 5
	next := current
	for {
		next += dir
		if next > maxIdx {
			next = 0
		} else if next < 0 {
			next = maxIdx
		}
		// Skip index 3 (branch field) when not in git mode.
		if next == 3 && !m.sourceGit {
			continue
		}
		return next
	}
}

func (m registryAddModal) Update(msg tea.Msg) (registryAddModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	}
	return m, nil
}

func (m registryAddModal) updateMouse(msg tea.MouseMsg) (registryAddModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if zone.Get("regadd-cancel").InBounds(msg) {
		return m.cancel()
	}
	if zone.Get("regadd-add").InBounds(msg) {
		return m.submit()
	}
	if zone.Get("regadd-url").InBounds(msg) {
		m.focusIdx = 1
		m.cursor = len([]rune(m.urlValue))
		return m, nil
	}
	if zone.Get("regadd-name").InBounds(msg) {
		m.focusIdx = 2
		m.cursor = len([]rune(m.nameValue))
		return m, nil
	}
	if zone.Get("regadd-branch").InBounds(msg) && m.sourceGit {
		m.focusIdx = 3
		m.cursor = len([]rune(m.branchValue))
		return m, nil
	}
	if zone.Get("regadd-source").InBounds(msg) {
		m.focusIdx = 0
		return m, nil
	}

	// Click outside modal dismisses.
	if !zone.Get("registry-add-zone").InBounds(msg) {
		return m.cancel()
	}

	return m, nil
}

func (m registryAddModal) updateKey(msg tea.KeyMsg) (registryAddModal, tea.Cmd) {
	// Clear error on any keypress.
	m.err = ""

	switch msg.Type {
	case tea.KeyEsc:
		return m.cancel()

	case tea.KeyCtrlS:
		return m.submit()

	case tea.KeyTab:
		m.focusIdx = m.nextFocusIdx(m.focusIdx, +1)
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}

	case tea.KeyShiftTab:
		m.focusIdx = m.nextFocusIdx(m.focusIdx, -1)
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}

	case tea.KeyEnter:
		switch m.focusIdx {
		case 0: // Radio → URL
			m.focusIdx = 1
			m.cursor = len([]rune(m.urlValue))
		case 1: // URL → Name
			m.focusIdx = 2
			m.cursor = len([]rune(m.nameValue))
		case 2: // Name → Branch (git) or Cancel (local)
			if m.sourceGit {
				m.focusIdx = 3
				m.cursor = len([]rune(m.branchValue))
			} else {
				m.focusIdx = 4
			}
		case 3: // Branch → Add
			m.focusIdx = 5
		case 4: // Cancel
			return m.cancel()
		case 5: // Add
			return m.submit()
		}

	case tea.KeySpace:
		if m.focusIdx == 0 {
			// Radio toggle
			m.sourceGit = !m.sourceGit
		} else if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			newRunes := make([]rune, 0, len(runes)+1)
			newRunes = append(newRunes, runes[:m.cursor]...)
			newRunes = append(newRunes, ' ')
			newRunes = append(newRunes, runes[m.cursor:]...)
			*val = string(newRunes)
			m.cursor++
			m = m.afterTextChange()
		}

	case tea.KeyUp, tea.KeyDown:
		if m.focusIdx == 0 {
			m.sourceGit = !m.sourceGit
		}

	case tea.KeyBackspace:
		if m.isTextField() && m.cursor > 0 {
			val := m.focusedValue()
			runes := []rune(*val)
			*val = string(runes[:m.cursor-1]) + string(runes[m.cursor:])
			m.cursor--
			m = m.afterTextChange()
		}

	case tea.KeyDelete:
		if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			if m.cursor < len(runes) {
				*val = string(runes[:m.cursor]) + string(runes[m.cursor+1:])
				m = m.afterTextChange()
			}
		}

	case tea.KeyLeft:
		if m.isTextField() && m.cursor > 0 {
			m.cursor--
		} else if m.focusIdx == 4 {
			m.focusIdx = 5
		} else if m.focusIdx == 5 {
			m.focusIdx = 4
		}

	case tea.KeyRight:
		if m.isTextField() && m.cursor < len([]rune(*m.focusedValue())) {
			m.cursor++
		} else if m.focusIdx == 4 {
			m.focusIdx = 5
		} else if m.focusIdx == 5 {
			m.focusIdx = 4
		}

	case tea.KeyHome, tea.KeyCtrlA:
		if m.isTextField() {
			m.cursor = 0
		}

	case tea.KeyEnd, tea.KeyCtrlE:
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}

	case tea.KeyRunes:
		if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.cursor]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.cursor:]...)
			*val = string(newRunes)
			m.cursor += len(msg.Runes)
			m = m.afterTextChange()
		}
	}

	return m, nil
}

// afterTextChange handles auto-derivation of the name field from the URL,
// and tracking whether the name was manually set.
func (m registryAddModal) afterTextChange() registryAddModal {
	switch m.focusIdx {
	case 1: // URL field changed — auto-derive name if not manually set.
		if !m.nameManuallySet {
			if m.sourceGit {
				m.nameValue = registry.NameFromURL(m.urlValue)
			} else {
				m.nameValue = filepath.Base(m.urlValue)
			}
		}
	case 2: // Name field changed — track manual edits.
		if len(m.nameValue) > 0 {
			m.nameManuallySet = true
		} else {
			m.nameManuallySet = false
		}
	}
	return m
}

func (m registryAddModal) submit() (registryAddModal, tea.Cmd) {
	if errMsg := m.validate(); errMsg != "" {
		m.err = errMsg
		return m, nil
	}
	url := m.urlValue
	if !m.sourceGit {
		if abs, err := filepath.Abs(url); err == nil {
			if evaled, err2 := filepath.EvalSymlinks(abs); err2 == nil {
				url = evaled
			} else {
				url = abs
			}
		}
	}
	name := m.nameValue
	ref := m.branchValue
	isLocal := !m.sourceGit
	m.Close()
	return m, func() tea.Msg {
		return registryAddMsg{url: url, name: name, ref: ref, isLocal: isLocal}
	}
}

func (m registryAddModal) cancel() (registryAddModal, tea.Cmd) {
	m.Close()
	return m, nil
}

func hasUnsupportedProtocol(url string) bool {
	return !strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "http://") &&
		!strings.HasPrefix(url, "ssh://") &&
		!strings.HasPrefix(url, "git@")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (m registryAddModal) View() string {
	if !m.active {
		return ""
	}

	modalW := min(64, m.width-10)
	if modalW < 34 {
		modalW = 34
	}
	contentW := modalW - borderSize
	usableW := contentW - 2
	pad := " "

	// Title
	titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Add Registry")
	title := pad + titleText

	// Source label + radio
	sourceLabel := pad + mutedStyle.Render("Source")
	radio := m.renderRadio(usableW)

	// URL/Path label + field
	urlLabelText := "URL"
	if !m.sourceGit {
		urlLabelText = "Path"
	}
	urlLabel := pad + mutedStyle.Render(urlLabelText)
	urlInput := m.renderField(m.urlValue, 1, usableW, "regadd-url")

	// Name label + field
	nameLabelText := "Name (derived from URL)"
	nameLabel := pad + mutedStyle.Render(nameLabelText)
	nameInput := m.renderField(m.nameValue, 2, usableW, "regadd-name")

	// Build content lines
	lines := []string{
		title,
		"",
		sourceLabel,
		radio,
		"",
		urlLabel,
		urlInput,
		"",
		nameLabel,
		nameInput,
	}

	// Branch field (git mode only)
	if m.sourceGit {
		branchLabel := pad + mutedStyle.Render("Branch (optional)")
		branchInput := m.renderField(m.branchValue, 3, usableW, "regadd-branch")
		lines = append(lines, "", branchLabel, branchInput)
	}

	lines = append(lines, "")

	// Error line
	if m.err != "" {
		errLine := pad + lipgloss.NewStyle().Foreground(dangerColor).Render(m.err)
		lines = append(lines, errLine)
	}

	// Buttons
	cancelBtn := m.renderButton("Cancel", 4, "regadd-cancel")
	addBtn := m.renderButton("Add", 5, "regadd-add")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, " ", addBtn)
	buttonsW := lipgloss.Width(buttons)
	buttonPad := max(0, usableW-buttonsW)
	buttonRow := pad + strings.Repeat(" ", buttonPad) + buttons
	lines = append(lines, buttonRow)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(content)

	return zone.Mark("registry-add-zone", box)
}

// renderRadio renders the source type radio buttons.
func (m registryAddModal) renderRadio(usableW int) string {
	pad := " "
	gitMark := "( )"
	localMark := "( )"
	if m.sourceGit {
		gitMark = "(\u2022)"
	} else {
		localMark = "(\u2022)"
	}

	style := lipgloss.NewStyle().Foreground(primaryText)
	if m.focusIdx == 0 {
		style = style.Bold(true).Foreground(accentColor)
	}

	gitLine := pad + style.Render(gitMark+" Git URL")
	localLine := pad + style.Render(localMark+" Local directory")

	return zone.Mark("regadd-source", gitLine+"\n"+localLine)
}

// renderField renders a text input field with background tinting and cursor.
func (m registryAddModal) renderField(value string, fieldIdx, usableW int, zoneID string) string {
	bg := inputInactiveBG
	if m.focusIdx == fieldIdx {
		bg = inputActiveBG
	}
	displayVal := m.renderValueWithCursor(value, fieldIdx, usableW-2)
	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		Width(usableW).
		Padding(0, 1)
	return zone.Mark(zoneID, " "+style.Render(displayVal)+" ")
}

// renderValueWithCursor renders text with a block cursor when the field is focused.
func (m registryAddModal) renderValueWithCursor(value string, fieldIdx, maxW int) string {
	if m.focusIdx != fieldIdx {
		return truncate(value, maxW)
	}

	runes := []rune(value)
	if m.cursor >= len(runes) {
		return truncate(value+"\u2588", maxW)
	}
	before := string(runes[:m.cursor])
	under := string(runes[m.cursor : m.cursor+1])
	after := string(runes[m.cursor+1:])
	cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
	return truncate(before+cursorChar+after, maxW)
}

// renderButton renders a button label with focus styling.
func (m registryAddModal) renderButton(label string, idx int, zoneID string) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIdx == idx {
		style = style.
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
			Background(accentColor)
	} else {
		style = style.
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
	}
	return zone.Mark(zoneID, style.Render(label))
}

func (m registryAddModal) validate() string {
	if m.urlValue == "" {
		return "URL is required"
	}
	if m.sourceGit && hasUnsupportedProtocol(m.urlValue) {
		return "Only https://, ssh://, and git@ URLs are supported"
	}
	if m.sourceGit && strings.Contains(m.urlValue, "ext::") {
		return "Only https://, ssh://, and git@ URLs are supported"
	}
	if m.sourceGit && m.cfg != nil && !m.cfg.IsRegistryAllowed(m.urlValue) {
		return "URL not permitted by registry allowlist"
	}
	if !m.sourceGit && !dirExists(m.urlValue) {
		return "Directory does not exist"
	}
	if m.nameValue == "" {
		return "Name is required"
	}
	if !catalog.IsValidRegistryName(m.nameValue) {
		return "Invalid name (use letters, numbers, - and _ with optional owner/repo format)"
	}
	for _, existing := range m.existingNames {
		if strings.EqualFold(m.nameValue, existing) {
			return fmt.Sprintf("Registry %q already exists", m.nameValue)
		}
	}
	if m.sourceGit && m.branchValue != "" && !validBranchRe.MatchString(m.branchValue) {
		return "Branch name can only contain letters, numbers, ., _, / and -"
	}
	return ""
}
