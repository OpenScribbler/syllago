package tui

import (
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// toastMsg is sent by any component to trigger a toast notification.
// App.Update() catches this and sets the active toast state.
type toastMsg struct {
	text       string
	isErr      bool
	isProgress bool // in-progress indicator (no "Done:" prefix, accent border)
}

// toastModel holds the state for the active toast overlay.
type toastModel struct {
	active       bool
	text         string
	isErr        bool
	isProgress   bool
	spinner      spinner.Model
	scrollOffset int // for long error messages
	width        int // content pane width (updated on WindowSizeMsg)
}

// show activates the toast with the given message.
func (t *toastModel) show(msg toastMsg) {
	t.active = true
	t.text = msg.text
	t.isErr = msg.isErr
	t.isProgress = msg.isProgress
	t.scrollOffset = 0
	if msg.isProgress {
		sp := spinner.New()
		sp.Spinner = spinner.Dot
		sp.Style = lipgloss.NewStyle().Foreground(accentColor)
		t.spinner = sp
	}
}

// tickSpinner returns the tea.Cmd to start spinner animation.
// Should be called after show() when isProgress is true.
func (t *toastModel) tickSpinner() tea.Cmd {
	if !t.isProgress {
		return nil
	}
	return t.spinner.Tick
}

// updateSpinner forwards a spinner tick message and returns the next tick cmd.
func (t *toastModel) updateSpinner(msg tea.Msg) tea.Cmd {
	if !t.active || !t.isProgress {
		return nil
	}
	var cmd tea.Cmd
	t.spinner, cmd = t.spinner.Update(msg)
	return cmd
}

// dismiss clears the toast.
func (t *toastModel) dismiss() {
	t.active = false
	t.text = ""
	t.scrollOffset = 0
}

// copyToClipboard copies the sanitized error text to the system clipboard.
// Returns an error message if the copy fails.
func (t *toastModel) copyToClipboard() string {
	sanitized := sanitizeForClipboard(t.text)
	if err := clipboard.WriteAll(sanitized); err != nil {
		return "Copy failed — clipboard tool not found"
	}
	return ""
}

// isScrollable returns true if the toast content exceeds the visible line limit.
func (t *toastModel) isScrollable() bool {
	if !t.active || t.text == "" {
		return false
	}
	innerW := t.width - 8
	if innerW < 20 {
		innerW = 20
	}
	prefix := "Done: "
	if t.isErr {
		prefix = "Error: "
	} else if t.isProgress {
		prefix = ""
	}
	wrapped := wordwrap.String(prefix+t.text, innerW)
	return len(strings.Split(wrapped, "\n")) > 5
}

// clampScroll ensures scrollOffset stays within valid bounds.
func (t *toastModel) clampScroll() {
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
	innerW := t.width - 8
	if innerW < 20 {
		innerW = 20
	}
	prefix := "Done: "
	if t.isErr {
		prefix = "Error: "
	} else if t.isProgress {
		prefix = ""
	}
	wrapped := wordwrap.String(prefix+t.text, innerW)
	lines := strings.Split(wrapped, "\n")
	maxOffset := len(lines) - 5
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.scrollOffset > maxOffset {
		t.scrollOffset = maxOffset
	}
}

// view renders the toast box.
func (t toastModel) view() string {
	if !t.active || t.text == "" {
		return ""
	}

	// Toast inner width: content pane width minus border/padding (4) minus margin (4)
	innerW := t.width - 8
	if innerW < 20 {
		innerW = 20
	}

	var prefix string
	var borderColor lipgloss.AdaptiveColor
	if t.isErr {
		prefix = "Error: "
		borderColor = dangerColor
	} else if t.isProgress {
		prefix = t.spinner.View() + " "
		borderColor = accentColor
	} else {
		prefix = "Done: "
		borderColor = successColor
	}

	fullText := prefix + t.text
	wrapped := wordwrap.String(fullText, innerW)

	var content string
	if t.isProgress {
		content = wrapped
	} else if t.isErr {
		lines := strings.Split(wrapped, "\n")
		// Error toast: fixed 5 visible lines, scrollable
		visibleLines := 5
		if len(lines) <= visibleLines {
			content = wrapped
		} else {
			offset := t.scrollOffset
			maxOffset := len(lines) - visibleLines
			if offset > maxOffset {
				offset = maxOffset
			}
			if offset < 0 {
				offset = 0
			}
			end := offset + visibleLines
			if end > len(lines) {
				end = len(lines)
			}
			content = strings.Join(lines[offset:end], "\n")
			if offset > 0 {
				content = renderScrollUp(offset, true) + "\n" + content
			}
			if end < len(lines) {
				content += "\n" + renderScrollDown(len(lines)-end, true)
			}
		}
		content += "\n" + helpStyle.Render("c copy • esc dismiss")
	} else {
		lines := strings.Split(wrapped, "\n")
		visibleLines := 5
		if len(lines) <= visibleLines {
			content = wrapped
		} else {
			offset := t.scrollOffset
			maxOffset := len(lines) - visibleLines
			if offset > maxOffset {
				offset = maxOffset
			}
			if offset < 0 {
				offset = 0
			}
			end := offset + visibleLines
			if end > len(lines) {
				end = len(lines)
			}
			content = strings.Join(lines[offset:end], "\n")
			if offset > 0 {
				content = renderScrollUp(offset, true) + "\n" + content
			}
			if end < len(lines) {
				content += "\n" + renderScrollDown(len(lines)-end, true)
			}
		}
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(innerW + 4). // account for padding
		MaxWidth(t.width)

	return style.Render(content)
}

// sanitizeForClipboard strips sensitive information from error text before copying.
func sanitizeForClipboard(msg string) string {
	// Strip absolute file paths
	pathRe := regexp.MustCompile(`/(?:home|Users)/[^\s:]+`)
	msg = pathRe.ReplaceAllString(msg, "<path>")

	// Strip git remote URLs (may contain tokens)
	gitRe := regexp.MustCompile(`https?://[^\s]*\.git\b`)
	msg = gitRe.ReplaceAllString(msg, "<url>")

	// Strip environment variable values that look like secrets
	secretRe := regexp.MustCompile(`(?i)(?:_KEY|_SECRET|_TOKEN|_PASSWORD)=\S+`)
	msg = secretRe.ReplaceAllStringFunc(msg, func(match string) string {
		idx := strings.Index(match, "=")
		if idx >= 0 {
			return match[:idx+1] + "<redacted>"
		}
		return match
	})

	return msg
}
