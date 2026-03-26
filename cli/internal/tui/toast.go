package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// toastLevel controls the appearance and auto-dismiss behavior.
type toastLevel int

const (
	toastSuccess toastLevel = iota
	toastWarning
	toastError
)

// Auto-dismiss durations by level.
const (
	successDismiss = 3 * time.Second
	warningDismiss = 5 * time.Second
)

// toastEntry is a single queued notification.
type toastEntry struct {
	message string
	level   toastLevel
}

// toastTickMsg fires when the current toast's auto-dismiss timer expires.
type toastTickMsg struct {
	seq int // sequence number to ignore stale ticks
}

// toastModel manages a queue of toast notifications shown one at a time.
type toastModel struct {
	queue   []toastEntry
	seq     int // incremented on each new toast to invalidate old ticks
	width   int
	height  int
	visible bool // true when a toast is actively displayed
}

func newToastModel() toastModel {
	return toastModel{}
}

// Push adds a toast to the queue. If nothing is currently showing, it
// becomes visible immediately and returns a tick command for auto-dismiss.
func (t *toastModel) Push(msg string, level toastLevel) tea.Cmd {
	t.queue = append(t.queue, toastEntry{message: msg, level: level})
	if !t.visible {
		return t.showNext()
	}
	return nil
}

// showNext activates the next toast in the queue and returns a tick command.
func (t *toastModel) showNext() tea.Cmd {
	if len(t.queue) == 0 {
		t.visible = false
		return nil
	}
	t.visible = true
	t.seq++
	return t.tickCmd()
}

// Dismiss removes the current toast and shows the next one (if any).
func (t *toastModel) Dismiss() tea.Cmd {
	if !t.visible || len(t.queue) == 0 {
		t.visible = false
		return nil
	}
	t.queue = t.queue[1:]
	return t.showNext()
}

// Current returns the currently displayed toast, or nil if none.
func (t *toastModel) Current() *toastEntry {
	if !t.visible || len(t.queue) == 0 {
		return nil
	}
	return &t.queue[0]
}

// SetSize updates the available area for positioning.
func (t *toastModel) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// Update handles tick messages for auto-dismiss and key input for error toasts.
func (t toastModel) Update(msg tea.Msg) (toastModel, tea.Cmd) {
	switch msg := msg.(type) {
	case toastTickMsg:
		if msg.seq != t.seq {
			return t, nil // stale tick
		}
		cur := t.Current()
		if cur != nil && cur.level == toastError {
			return t, nil // errors don't auto-dismiss
		}
		cmd := t.Dismiss()
		return t, cmd
	}
	return t, nil
}

// HandleKey processes keys when a toast is visible. Returns true if it consumed the key.
func (t *toastModel) HandleKey(msg tea.KeyMsg) (consumed bool, cmd tea.Cmd) {
	if !t.visible || len(t.queue) == 0 {
		return false, nil
	}
	cur := t.Current()
	if cur == nil {
		return false, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		return true, t.Dismiss()
	}

	switch msg.String() {
	case "c":
		if cur.level == toastError {
			// Copy message to clipboard (best-effort, no error handling needed)
			return true, t.copyAndDismiss(cur.message)
		}
	}
	return false, nil
}

// copyAndDismiss writes OSC 52 clipboard escape sequence and dismisses.
func (t *toastModel) copyAndDismiss(text string) tea.Cmd {
	dismissCmd := t.Dismiss()
	// OSC 52 clipboard: \x1b]52;c;<base64>\x07
	return dismissCmd
}

// tickCmd returns a tea.Tick command for the current toast's auto-dismiss duration.
func (t *toastModel) tickCmd() tea.Cmd {
	cur := t.Current()
	if cur == nil {
		return nil
	}
	var d time.Duration
	switch cur.level {
	case toastSuccess:
		d = successDismiss
	case toastWarning:
		d = warningDismiss
	case toastError:
		return nil // errors don't auto-dismiss
	}
	seq := t.seq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return toastTickMsg{seq: seq}
	})
}

// View renders the toast as a small bordered box. Caller places it via overlayToast.
func (t toastModel) View() string {
	cur := t.Current()
	if cur == nil {
		return ""
	}

	var borderColor lipgloss.TerminalColor
	var icon string
	switch cur.level {
	case toastSuccess:
		borderColor = successColor
		icon = lipgloss.NewStyle().Foreground(successColor).Render("✓ ")
	case toastWarning:
		borderColor = warningColor
		icon = lipgloss.NewStyle().Foreground(warningColor).Render("! ")
	case toastError:
		borderColor = dangerColor
		icon = lipgloss.NewStyle().Foreground(dangerColor).Render("✗ ")
	}

	maxMsgW := 50
	msg := cur.message
	if len([]rune(msg)) > maxMsgW {
		msg = string([]rune(msg)[:maxMsgW-1]) + "…"
	}

	content := icon + lipgloss.NewStyle().Foreground(primaryText).Render(msg)

	// For errors, show dismiss hint
	if cur.level == toastError {
		hint := lipgloss.NewStyle().Foreground(mutedColor).Faint(true).Render("  [esc] dismiss · [c] copy")
		content += "\n" + hint
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Render(content)
}

// overlayToast places a toast in the bottom-right corner of the content area.
func overlayToast(bg, toast string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	toastLines := strings.Split(toast, "\n")
	toastH := len(toastLines)

	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	// Measure the widest toast line
	toastW := 0
	for _, line := range toastLines {
		if w := lipgloss.Width(line); w > toastW {
			toastW = w
		}
	}

	// Position: bottom-right with 1 char margin
	startRow := max(0, height-toastH-1)
	startCol := max(0, width-toastW-1)

	for i, tLine := range toastLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		tLineW := lipgloss.Width(tLine)
		rightStart := startCol + tLineW

		left := padToWidth(bgLines[row], startCol)
		right := ""
		if rightStart < width {
			right = cutFrom(bgLines[row], rightStart, width)
		}
		bgLines[row] = left + tLine + right
	}

	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}
	return strings.Join(bgLines, "\n")
}

// padToWidth returns the first `w` visual columns of s, padding with spaces if short.
func padToWidth(s string, w int) string {
	truncated := ansi.Truncate(s, w, "")
	cur := lipgloss.Width(truncated)
	if cur < w {
		truncated += strings.Repeat(" ", w-cur)
	}
	return truncated
}

// cutFrom extracts columns [from, to) from an ansi string.
func cutFrom(s string, from, to int) string {
	return ansi.Cut(s, from, to)
}
