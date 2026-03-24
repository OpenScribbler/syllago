package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// modalType identifies the kind of modal overlay.
type modalType int

const (
	modalConfirm modalType = iota
	modalWarning
	modalWizard
	modalInput
)

// modalModel is a shared overlay component for confirm, warning, wizard,
// and input modals. All share the same visual frame: fixed 56-char width,
// max height termHeight-4, scrollable body with pinned title and buttons.
type modalModel struct {
	active       bool
	modalType    modalType
	title        string
	body         string   // plain text body
	bodyLines    []string // split for scrolling
	scrollOffset int
	buttonCursor int    // 0=left, 1=right
	leftButton   string // e.g. "Cancel"
	rightButton  string // e.g. "Confirm"

	// Wizard-specific
	stepCurrent  int
	stepTotal    int
	options      []string // selectable options
	optionCursor int
	selected     map[int]bool // for checkbox-style selection

	// Input-specific
	inputLabel string
	inputValue string

	// Dimensions
	termWidth  int
	termHeight int
}

// Message types returned by modal Update.
type modalCloseMsg struct{}

type modalConfirmMsg struct {
	modalType   modalType
	optionIndex int
	inputValue  string
	selections  map[int]bool
}

type modalCopyMsg struct {
	text string
}

// Constructor functions — all set active=true and the appropriate modalType.

func newConfirmModal(title, body, leftBtn, rightBtn string) modalModel {
	lines := splitLines(body)
	return modalModel{
		active:      true,
		modalType:   modalConfirm,
		title:       title,
		body:        body,
		bodyLines:   lines,
		leftButton:  leftBtn,
		rightButton: rightBtn,
		selected:    make(map[int]bool),
	}
}

func newWarningModal(title, body, leftBtn, rightBtn string) modalModel {
	lines := splitLines(body)
	return modalModel{
		active:      true,
		modalType:   modalWarning,
		title:       title,
		body:        body,
		bodyLines:   lines,
		leftButton:  leftBtn,
		rightButton: rightBtn,
		selected:    make(map[int]bool),
	}
}

func newWizardModal(title string, step, total int, options []string) modalModel {
	return modalModel{
		active:      true,
		modalType:   modalWizard,
		title:       title,
		stepCurrent: step,
		stepTotal:   total,
		options:     options,
		leftButton:  "Cancel",
		rightButton: "Confirm",
		selected:    make(map[int]bool),
	}
}

func newInputModal(title, label, placeholder string) modalModel {
	return modalModel{
		active:      true,
		modalType:   modalInput,
		title:       title,
		inputLabel:  label,
		inputValue:  placeholder,
		leftButton:  "Cancel",
		rightButton: "OK",
		selected:    make(map[int]bool),
	}
}

// splitLines splits text into lines, returning at least one empty line
// for empty input so the body zone always renders.
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

// maxBodyHeight returns the max lines available for scrollable body content.
// Layout: border(2) + padding(2) + title(1) + blank after title(1) + button separator(1) + buttons(1) = 8 fixed lines.
func (m modalModel) maxBodyHeight() int {
	maxH := m.termHeight - 4
	if maxH < 6 {
		maxH = 6
	}
	// Subtract fixed chrome: 2 border + 2 padding + 2 title zone + 2 button zone
	body := maxH - 8
	if body < 1 {
		body = 1
	}
	return body
}

// bodyContent returns the lines to display in the scrollable body zone.
func (m modalModel) bodyContent() []string {
	switch m.modalType {
	case modalWizard:
		var lines []string
		for i, opt := range m.options {
			check := "[ ]"
			if m.selected[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("  %s %s", check, opt)
			if i == m.optionCursor {
				line = selectedItemStyle.Render(line)
			}
			lines = append(lines, line)
		}
		return lines
	case modalInput:
		contentWidth := modalWidth - 4 - 4 // border(2) + padding(4) of modal, minus inner box borders
		if contentWidth < 10 {
			contentWidth = 10
		}
		var lines []string
		lines = append(lines, metaNameStyle.Render(m.inputLabel))
		boxBorder := lipgloss.NormalBorder()
		boxStyle := lipgloss.NewStyle().
			Border(boxBorder).
			BorderForeground(borderColor).
			Width(contentWidth)
		lines = append(lines, boxStyle.Render(m.inputValue))
		return lines
	default:
		return m.bodyLines
	}
}

// clampScroll ensures scrollOffset stays within valid bounds.
func (m *modalModel) clampScroll(totalLines, visibleLines int) {
	maxOffset := totalLines - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// Update handles keyboard input for the modal.
func (m modalModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case keyMsg.Type == tea.KeyEsc:
		m.active = false
		return m, func() tea.Msg { return modalCloseMsg{} }

	case keyMsg.Type == tea.KeyEnter:
		m.active = false
		return m, func() tea.Msg {
			return modalConfirmMsg{
				modalType:   m.modalType,
				optionIndex: m.optionCursor,
				inputValue:  m.inputValue,
				selections:  m.selected,
			}
		}

	case key.Matches(keyMsg, keys.Left):
		m.buttonCursor = 0

	case key.Matches(keyMsg, keys.Right):
		m.buttonCursor = 1

	case key.Matches(keyMsg, keys.Up):
		if m.modalType == modalWizard && len(m.options) > 0 {
			if m.optionCursor > 0 {
				m.optionCursor--
			}
		} else {
			m.scrollOffset--
			content := m.bodyContent()
			m.clampScroll(len(content), m.maxBodyHeight())
		}

	case key.Matches(keyMsg, keys.Down):
		if m.modalType == modalWizard && len(m.options) > 0 {
			if m.optionCursor < len(m.options)-1 {
				m.optionCursor++
			}
		} else {
			m.scrollOffset++
			content := m.bodyContent()
			m.clampScroll(len(content), m.maxBodyHeight())
		}

	case key.Matches(keyMsg, keys.Space):
		if m.modalType == modalWizard && len(m.options) > 0 {
			m.selected[m.optionCursor] = !m.selected[m.optionCursor]
		}

	case key.Matches(keyMsg, keys.ConfirmYes):
		if m.modalType == modalConfirm {
			m.active = false
			return m, func() tea.Msg {
				return modalConfirmMsg{
					modalType:   m.modalType,
					optionIndex: m.optionCursor,
					inputValue:  m.inputValue,
					selections:  m.selected,
				}
			}
		}

	case key.Matches(keyMsg, keys.ConfirmNo):
		if m.modalType == modalConfirm {
			m.active = false
			return m, func() tea.Msg { return modalCloseMsg{} }
		}

	case key.Matches(keyMsg, keys.Copy):
		if m.body != "" {
			return m, func() tea.Msg { return modalCopyMsg{text: m.body} }
		}
	}

	return m, nil
}

// View renders the modal overlay content (without centering — the caller
// handles overlay positioning).
func (m modalModel) View() string {
	if !m.active {
		return ""
	}

	contentWidth := modalWidth - 4 // subtract padding (2 each side)

	// Title zone
	titleText := metaNameStyle.Render(m.title)
	if m.modalType == modalWizard && m.stepTotal > 0 {
		titleText = metaNameStyle.Render(
			fmt.Sprintf("%s (%d of %d)", m.title, m.stepCurrent, m.stepTotal),
		)
	}

	// Body zone with scrolling
	content := m.bodyContent()
	maxBody := m.maxBodyHeight()
	totalLines := len(content)

	// Clamp scroll for display
	scrollOffset := m.scrollOffset
	maxOffset := totalLines - maxBody
	if maxOffset < 0 {
		maxOffset = 0
	}
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	var bodyParts []string

	// Scroll-up indicator
	if scrollOffset > 0 {
		bodyParts = append(bodyParts, helpStyle.Render(fmt.Sprintf("(%d more above)", scrollOffset)))
		maxBody-- // indicator takes a line
	}

	// Scroll-down indicator needed?
	belowCount := totalLines - scrollOffset - maxBody
	needsDownIndicator := belowCount > 0
	if needsDownIndicator {
		maxBody-- // reserve a line for the indicator
	}

	// Visible content
	end := scrollOffset + maxBody
	if end > totalLines {
		end = totalLines
	}
	for i := scrollOffset; i < end; i++ {
		bodyParts = append(bodyParts, content[i])
	}

	// Scroll-down indicator
	if needsDownIndicator {
		bodyParts = append(bodyParts, helpStyle.Render(fmt.Sprintf("(%d more below)", belowCount)))
	}

	bodyStr := strings.Join(bodyParts, "\n")

	// Button zone
	btnLine := renderButtons(m.leftButton, m.rightButton, m.buttonCursor, contentWidth)

	// Assemble: title + blank + body + blank + buttons
	inner := titleText + "\n\n" + bodyStr + "\n\n" + btnLine

	return modalStyle.
		Width(modalWidth).
		Render(inner)
}

// renderButtons renders two buttons centered on the line. The active button
// (indicated by cursor: 0=left, 1=right) uses buttonStyle; the inactive one
// uses buttonDisabledStyle.
func renderButtons(left, right string, cursor int, width int) string {
	var leftRendered, rightRendered string
	if cursor == 0 {
		leftRendered = buttonStyle.Render(left)
		rightRendered = buttonDisabledStyle.Render(right)
	} else {
		leftRendered = buttonDisabledStyle.Render(left)
		rightRendered = buttonStyle.Render(right)
	}

	pair := leftRendered + "  " + rightRendered
	pairWidth := lipgloss.Width(pair)

	pad := (width - pairWidth) / 2
	if pad < 0 {
		pad = 0
	}

	return strings.Repeat(" ", pad) + pair
}
