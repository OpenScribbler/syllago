package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// helpOverlay renders a full-screen keyboard shortcut reference.
type helpOverlay struct {
	active bool
	width  int
	height int
}

func newHelpOverlay() helpOverlay {
	return helpOverlay{}
}

// Toggle flips the overlay on/off.
func (h *helpOverlay) Toggle() {
	h.active = !h.active
}

// SetSize sets the available dimensions.
func (h *helpOverlay) SetSize(w, h2 int) {
	h.width = w
	h.height = h2
}

// Update handles keys and mouse when active.
func (h helpOverlay) Update(msg tea.Msg) (helpOverlay, tea.Cmd) {
	if !h.active {
		return h, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == keyHelp || msg.Type == tea.KeyEsc:
			h.active = false
		case msg.Type == tea.KeyCtrlC:
			return h, tea.Quit
		}
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if zone.Get("help-close").InBounds(msg) {
				h.active = false
			}
		}
	}
	return h, nil
}

// View renders the help overlay as a full-screen page.
func (h helpOverlay) View() string {
	if !h.active {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	keyStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(primaryText)
	sepStyle := lipgloss.NewStyle().Foreground(mutedColor)

	section := func(title string, items [][2]string) string {
		lines := []string{headerStyle.Render(title), sepStyle.Render(strings.Repeat("\u2500", len(title)+2))}
		for _, item := range items {
			key := keyStyle.Render(lipgloss.NewStyle().Width(14).Render(item[0]))
			desc := descStyle.Render(item[1])
			lines = append(lines, key+desc)
		}
		return strings.Join(lines, "\n")
	}

	// Trust Glyphs lives in col1 so the two columns end up at similar heights
	// (Nav+FileTree+Glyphs ≈ Actions+Gallery). Otherwise col2 runs ~10 lines
	// taller than col1 and the box has a "giant empty spot" on the left.
	// Render glyphs through their styles so the legend matches what users see
	// on registry cards — lipgloss preserves nested ANSI runs through the
	// Width() padding inside section().
	col1 := lipgloss.JoinVertical(lipgloss.Left,
		section("Navigation", [][2]string{
			{"1 / 2 / 3", "Switch group"},
			{"h / l", "Cycle sub-tabs"},
			{"j / k", "Move up/down"},
			{"Enter", "Select / drill in"},
			{"Esc", "Back / close"},
			{"q", "Back / quit"},
		}),
		"",
		section("File Tree", [][2]string{
			{"h / l", "Switch pane"},
			{"Enter", "Open file"},
			{"Esc", "Close detail"},
		}),
		"",
		section("Trust Glyphs", [][2]string{
			{trustVerifiedStyle.Render("✓"), "Verified — fresh attestation"},
			{trustStaleStyle.Render("!"), "Stale — re-sync to refresh"},
			{trustRevokedStyle.Render("R"), "Revoked — item withdrawn"},
		}),
	)

	col2 := lipgloss.JoinVertical(lipgloss.Left,
		section("Actions", [][2]string{
			{"a", "Add content"},
			{"n", "Create new (coming soon)"},
			{"i", "Install to provider"},
			{"e", "Edit name/description"},
			{"d", "Remove from library"},
			{"x", "Uninstall from provider"},
			{"t", "Inspect trust details"},
			{"/", "Search"},
			{"s / S", "Sort / reverse sort"},
			{"R", "Refresh catalog"},
		}),
		"",
		section("Gallery", [][2]string{
			{"arrows", "Navigate grid"},
			{"Tab", "Grid / contents"},
			{"Enter", "Drill into card"},
		}),
	)

	// Adaptive column width: scale to the available terminal so descriptions
	// don't wrap. Floor at 36 (the legacy width — keeps the 80-col case from
	// blowing past the screen) and cap at 55 (anything wider just gets
	// awkward whitespace between key and description).
	colW := 44
	if h.width > 0 {
		colW = (h.width - 11) / 2
		if colW < 36 {
			colW = 36
		}
		if colW > 55 {
			colW = 55
		}
	}
	col1Styled := lipgloss.NewStyle().Width(colW).Render(col1)
	col2Styled := lipgloss.NewStyle().Width(colW).Render(col2)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, col1Styled, "   ", col2Styled)

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryText).
		Render("Keyboard Shortcuts")

	// Close button — right-aligned within inner width
	closeBtn := zone.Mark("help-close", activeButtonStyle.Render("[Esc] Close"))

	// Build bordered box manually (lipgloss Border + Width mangles zone markers)
	innerW := colW*2 + 7 // 2 columns + gap + padding
	borderFg := accentColor

	padLine := func(s string) string {
		w := lipgloss.Width(s)
		if g := innerW - w; g > 0 {
			return s + strings.Repeat(" ", g)
		}
		return lipgloss.NewStyle().MaxWidth(innerW).Render(s)
	}

	bc := lipgloss.NewStyle().Foreground(borderFg)
	top := bc.Render("╭") + bc.Render(strings.Repeat("─", innerW)) + bc.Render("╮")
	bot := bc.Render("╰") + bc.Render(strings.Repeat("─", innerW)) + bc.Render("╯")
	lr := bc.Render("│")

	row := func(s string) string {
		return lr + padLine(s) + lr
	}

	// Close button centered
	closeBtnW := lipgloss.Width(closeBtn)
	closePad := max(0, (innerW-closeBtnW)/2)
	closeRow := strings.Repeat(" ", closePad) + closeBtn

	var lines []string
	lines = append(lines, top)
	lines = append(lines, row("  "+title))
	lines = append(lines, row(""))

	for _, cl := range strings.Split(columns, "\n") {
		lines = append(lines, row("  "+cl))
	}

	lines = append(lines, row(""))
	lines = append(lines, row(closeRow))
	lines = append(lines, bot)

	return strings.Join(lines, "\n")
}
