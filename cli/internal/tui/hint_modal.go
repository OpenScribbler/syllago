package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// hintModal is a one-time informational overlay shown after a registry is
// added. It guides the user toward their next action and, when dismissed,
// saves hints.registry_add_dismissed to global config so it does not appear
// again.
type hintModal struct {
	active bool
	width  int
	height int
}

func newHintModal() hintModal { return hintModal{} }

func (m *hintModal) Open()  { m.active = true }
func (m *hintModal) Close() { m.active = false }

func (m *hintModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles key and mouse input when active.
func (m hintModal) Update(msg tea.Msg) (hintModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", "q":
			m.active = false
			return m, func() tea.Msg { return hintDismissedMsg{} }
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if zone.Get("hint-dismiss").InBounds(msg) {
				m.active = false
				return m, func() tea.Msg { return hintDismissedMsg{} }
			}
		}
	}
	return m, nil
}

// View renders the hint modal box.
func (m hintModal) View() string {
	if !m.active {
		return ""
	}

	modalW := 56
	innerW := modalW - 2
	contentW := innerW - 4

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	bodyStyle := lipgloss.NewStyle().Foreground(mutedColor).MaxWidth(contentW)

	title := titleStyle.Render("Registry Added!")

	body := bodyStyle.Render(strings.Join([]string{
		"Content is now visible in the Library under",
		"\"not in library\" entries.",
		"",
		"Browse:  syllago list --source registry",
		"Install: syllago add <name> --from <reg>",
		"TUI:     switch to Content [2] to browse",
	}, "\n"))

	btnStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(accentColor).
		Padding(0, 2)
	btn := zone.Mark("hint-dismiss",
		lipgloss.NewStyle().Width(contentW).Align(lipgloss.Center).Render(
			btnStyle.Render("Got it"),
		),
	)

	inner := lipgloss.NewStyle().
		Padding(1, 2).
		Width(innerW).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Width(contentW).Align(lipgloss.Center).Render(title),
			"",
			body,
			"",
			btn,
		))

	top := "╭" + strings.Repeat("─", innerW) + "╮"
	bot := "╰" + strings.Repeat("─", innerW) + "╯"

	var rows []string
	rows = append(rows, top)
	for _, ln := range strings.Split(inner, "\n") {
		rows = append(rows, "│"+ln+"│")
	}
	rows = append(rows, bot)

	return lipgloss.NewStyle().Foreground(accentColor).
		Render(strings.Join(rows, "\n"))
}
