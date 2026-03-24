package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// toastType determines the toast behavior and styling.
type toastType int

const (
	toastSuccess toastType = iota // green, "Done:", 3s auto-dismiss
	toastWarning                  // amber, "Warn:", 5s auto-dismiss
	toastError                    // red, "Error:", never auto-dismiss
)

// toastModel manages the toast overlay.
type toastModel struct {
	active       bool
	text         string
	lines        []string
	toastType    toastType
	scrollOffset int
	width        int
}

// toastDismissMsg is sent when the auto-dismiss timer fires.
type toastDismissMsg struct{}

// toastCopyMsg is sent when the user copies toast text.
type toastCopyMsg struct {
	text string
}

// showToast creates and activates a toast.
func showToast(text string, tt toastType) toastModel {
	lines := strings.Split(text, "\n")
	return toastModel{
		active:    true,
		text:      text,
		lines:     lines,
		toastType: tt,
	}
}

// autoDismissCmd returns a command that sends a dismiss message after the
// appropriate delay, or nil for error toasts (which never auto-dismiss).
func (m toastModel) autoDismissCmd() tea.Cmd {
	var dur time.Duration
	switch m.toastType {
	case toastSuccess:
		dur = 3 * time.Second
	case toastWarning:
		dur = 5 * time.Second
	case toastError:
		return nil // never auto-dismiss
	}
	return tea.Tick(dur, func(time.Time) tea.Msg {
		return toastDismissMsg{}
	})
}

// prefix returns the toast prefix based on type.
func (m toastModel) prefix() string {
	switch m.toastType {
	case toastSuccess:
		return "Done: "
	case toastWarning:
		return "Warn: "
	case toastError:
		return "Error: "
	}
	return ""
}

// style returns the text style based on toast type.
func (m toastModel) style() lipgloss.Style {
	switch m.toastType {
	case toastSuccess:
		return successMsgStyle
	case toastWarning:
		return warningStyle
	case toastError:
		return errorMsgStyle
	}
	return helpStyle
}

// Update handles toast input.
func (m toastModel) Update(msg tea.Msg) (toastModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case toastDismissMsg:
		m.active = false
		return m, nil

	case tea.KeyMsg:
		// c copies text
		if key.Matches(msg, keys.Copy) {
			m.active = false
			return m, func() tea.Msg {
				return toastCopyMsg{text: m.text}
			}
		}
		// Esc dismisses any toast
		if msg.Type == tea.KeyEsc {
			m.active = false
			return m, nil
		}
		// For error toasts, other keys pass through (toast stays visible)
		if m.toastType == toastError {
			// Up/Down scroll for multi-line errors
			if key.Matches(msg, keys.Up) && m.scrollOffset > 0 {
				m.scrollOffset--
			}
			if key.Matches(msg, keys.Down) && m.scrollOffset < len(m.lines)-5 {
				m.scrollOffset++
			}
			return m, nil
		}
		// For success/warning, any key dismisses
		m.active = false
		return m, nil
	}

	return m, nil
}

// View renders the toast overlay.
func (m toastModel) View() string {
	if !m.active {
		return ""
	}

	s := m.style()
	prefix := s.Render(m.prefix())

	maxVisible := 5
	visibleLines := m.lines
	if len(visibleLines) > maxVisible {
		end := m.scrollOffset + maxVisible
		if end > len(visibleLines) {
			end = len(visibleLines)
		}
		visibleLines = visibleLines[m.scrollOffset:end]
	}

	// First line gets prefix
	var rows []string
	for i, line := range visibleLines {
		if i == 0 && m.scrollOffset == 0 {
			row := prefix + s.Render(line)
			// Add "c copy" right-aligned
			copyLabel := helpStyle.Render("c copy")
			gap := m.width - lipgloss.Width(row) - lipgloss.Width(copyLabel) - 4
			if gap < 1 {
				gap = 1
			}
			rows = append(rows, row+strings.Repeat(" ", gap)+copyLabel)
		} else {
			rows = append(rows, "  "+s.Render(line))
		}
	}

	// Scroll indicators
	if m.scrollOffset > 0 {
		rows = append([]string{helpStyle.Render("  (" + itoa(m.scrollOffset) + " more above)")}, rows...)
	}
	if m.scrollOffset+maxVisible < len(m.lines) {
		below := len(m.lines) - m.scrollOffset - maxVisible
		rows = append(rows, helpStyle.Render("  ("+itoa(below)+" more below)"))
	}

	content := strings.Join(rows, "\n")
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Width(m.width - 2)
	return border.Render(content)
}
