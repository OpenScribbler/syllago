package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// fileSelectedMsg is sent when a file is selected in the split view's file tree.
type fileSelectedMsg struct {
	filename string
}

// compatEntry represents a provider compatibility entry for hooks.
type compatEntry struct {
	provider string
	status   string // "ok", "partial", "limited", "none"
	note     string
}

// splitModel renders a split content zone with a file tree on the left
// and a preview pane on the right. Used by Skills and Hooks.
type splitModel struct {
	files      []string // file paths (relative to item directory)
	fileCursor int
	fileScroll int
	preview    previewModel // from content_preview.go
	focusLeft  bool         // true = file tree focused, false = preview focused

	// Hooks-specific: tab toggle between Files and Compat views
	showCompat bool          // when true, left pane shows compat matrix instead of files
	compatData []compatEntry // provider compatibility entries

	width  int
	height int
}

// newSplitModel creates a split model with the given file list and primary file content.
func newSplitModel(files []string, primaryFile string, primaryContent string) splitModel {
	return splitModel{
		files:     files,
		focusLeft: true,
		preview:   newPreviewModel(primaryFile, primaryContent),
	}
}

// leftWidth returns the width allocated to the left pane (~35%).
func (m splitModel) leftWidth() int {
	// 1 char for the vertical separator
	w := m.width * 35 / 100
	if w < 10 {
		w = 10
	}
	if w >= m.width-1 {
		w = m.width - 2
	}
	return w
}

// rightWidth returns the width allocated to the right pane.
func (m splitModel) rightWidth() int {
	return m.width - m.leftWidth() - 1 // -1 for separator
}

// visibleFileLines returns how many file entries fit in the left pane viewport.
// Subtracts 1 for the title row.
func (m splitModel) visibleFileLines() int {
	avail := m.height - 1 // title row
	if avail < 0 {
		return 0
	}
	return avail
}

// clampFileCursor ensures fileCursor and fileScroll stay within valid bounds.
func (m *splitModel) clampFileCursor() {
	if len(m.files) == 0 {
		m.fileCursor = 0
		m.fileScroll = 0
		return
	}
	if m.fileCursor < 0 {
		m.fileCursor = 0
	}
	if m.fileCursor >= len(m.files) {
		m.fileCursor = len(m.files) - 1
	}
	vis := m.visibleFileLines()
	if vis <= 0 {
		return
	}
	// Adjust scroll to keep cursor visible
	if m.fileCursor < m.fileScroll {
		m.fileScroll = m.fileCursor
	}
	if m.fileCursor >= m.fileScroll+vis {
		m.fileScroll = m.fileCursor - vis + 1
	}
	if m.fileScroll < 0 {
		m.fileScroll = 0
	}
}

// Update handles keyboard input for the split view.
func (m splitModel) Update(msg tea.Msg) (splitModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Left):
			m.focusLeft = true
			return m, nil
		case key.Matches(msg, keys.Right):
			m.focusLeft = false
			return m, nil
		case key.Matches(msg, keys.Tab):
			if len(m.compatData) > 0 {
				m.showCompat = !m.showCompat
			}
			return m, nil
		case key.Matches(msg, keys.Up):
			if m.focusLeft {
				if !m.showCompat && len(m.files) > 0 {
					m.fileCursor--
					m.clampFileCursor()
					return m, func() tea.Msg {
						return fileSelectedMsg{filename: m.files[m.fileCursor]}
					}
				}
			} else {
				var cmd tea.Cmd
				m.preview, cmd = m.preview.Update(msg)
				return m, cmd
			}
		case key.Matches(msg, keys.Down):
			if m.focusLeft {
				if !m.showCompat && len(m.files) > 0 {
					m.fileCursor++
					m.clampFileCursor()
					return m, func() tea.Msg {
						return fileSelectedMsg{filename: m.files[m.fileCursor]}
					}
				}
			} else {
				var cmd tea.Cmd
				m.preview, cmd = m.preview.Update(msg)
				return m, cmd
			}
		default:
			// Forward other keys to preview when focused right
			if !m.focusLeft {
				var cmd tea.Cmd
				m.preview, cmd = m.preview.Update(msg)
				return m, cmd
			}
		}
	case tea.MouseMsg:
		if !m.focusLeft {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// View renders the split pane layout.
func (m splitModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	lw := m.leftWidth()
	rw := m.rightWidth()

	// Update preview dimensions
	m.preview.width = rw
	m.preview.height = m.height

	leftView := m.renderLeftPane(lw)
	rightView := m.preview.View()

	// Join left and right with a vertical separator
	leftLines := strings.Split(leftView, "\n")
	rightLines := strings.Split(rightView, "\n")

	// Pad to equal height
	for len(leftLines) < m.height {
		leftLines = append(leftLines, strings.Repeat(" ", lw))
	}
	for len(rightLines) < m.height {
		rightLines = append(rightLines, "")
	}

	sep := lipgloss.NewStyle().Foreground(mutedColor).Render("\u2502") // │

	var b strings.Builder
	for i := 0; i < m.height; i++ {
		left := leftLines[i]
		// Pad left line to leftWidth
		leftW := lipgloss.Width(left)
		if leftW < lw {
			left += strings.Repeat(" ", lw-leftW)
		}

		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		b.WriteString(left)
		b.WriteString(sep)
		b.WriteString(right)
		if i < m.height-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderLeftPane renders either the file tree or compat matrix.
func (m splitModel) renderLeftPane(width int) string {
	var b strings.Builder

	if m.showCompat {
		b.WriteString(inlineTitle("Compat", width, primaryColor))
		b.WriteString("\n")
		b.WriteString(m.renderCompatMatrix(width))
	} else {
		b.WriteString(inlineTitle("Files", width, primaryColor))
		b.WriteString("\n")
		b.WriteString(m.renderFileTree(width))
	}

	return b.String()
}

// renderFileTree renders the flat file list with indentation for directories.
func (m splitModel) renderFileTree(width int) string {
	if len(m.files) == 0 {
		return helpStyle.Render("  (no files)")
	}

	vis := m.visibleFileLines()
	if vis <= 0 {
		return ""
	}

	end := m.fileScroll + vis
	if end > len(m.files) {
		end = len(m.files)
	}

	var b strings.Builder
	for i := m.fileScroll; i < end; i++ {
		file := m.files[i]

		// Determine indentation based on path depth
		indent := ""
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			// Count depth levels for indentation
			depth := strings.Count(file, "/")
			indent = strings.Repeat("  ", depth)
		}

		// Cursor prefix
		prefix := "  "
		style := itemStyle
		if i == m.fileCursor && m.focusLeft {
			prefix = "> "
			style = selectedItemStyle
		}

		entry := indent + file
		maxW := width - lipgloss.Width(prefix)
		if maxW > 0 {
			entry = truncateStr(entry, maxW)
		}

		b.WriteString(prefix)
		b.WriteString(style.Render(entry))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderCompatMatrix renders the provider compatibility table.
func (m splitModel) renderCompatMatrix(width int) string {
	if len(m.compatData) == 0 {
		return helpStyle.Render("  (no data)")
	}

	// Column headers
	provCol := "Provider"
	statusCol := "Status"
	provWidth := width / 2
	if provWidth < 12 {
		provWidth = 12
	}

	header := fmt.Sprintf("%-*s %s", provWidth, provCol, statusCol)
	header = truncateStr(header, width)

	var b strings.Builder
	b.WriteString(labelStyle.Render(header))
	b.WriteString("\n")

	// Separator
	sep := strings.Repeat("\u2500", provWidth) + " " + strings.Repeat("\u2500", width-provWidth-1)
	sep = truncateStr(sep, width)
	b.WriteString(helpStyle.Render(sep))

	for _, entry := range m.compatData {
		b.WriteString("\n")

		prov := truncateStr(entry.provider, provWidth)
		padded := fmt.Sprintf("%-*s", provWidth, prov)

		var statusStyle lipgloss.Style
		var statusIcon string
		switch entry.status {
		case "ok":
			statusStyle = installedStyle
			statusIcon = "[ok]"
		case "partial", "limited":
			statusStyle = warningStyle
			if entry.status == "partial" {
				statusIcon = "[~~]"
			} else {
				statusIcon = "[~~]"
			}
		default: // "none" or unknown
			statusStyle = notInstalledStyle
			statusIcon = "[--]"
		}

		statusText := statusIcon
		if entry.note != "" {
			statusText += " " + entry.note
		}

		line := padded + " " + statusStyle.Render(statusText)
		b.WriteString(line)
	}

	return b.String()
}
