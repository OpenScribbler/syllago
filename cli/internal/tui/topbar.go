package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// tabGroup represents a top-level navigation group (Content, Collections, Config).
type tabGroup struct {
	label   string
	hotkey  string   // display hint: "1", "2", "3"
	tabs    []string // sub-tab labels
	actions []string // right-aligned action button labels
}

// tabChangedMsg is fired when the active group or sub-tab changes.
type tabChangedMsg struct {
	group    int
	tab      int
	tabLabel string
}

// topBarModel manages a two-tier tab navigation bar with a bordered frame.
type topBarModel struct {
	groups      []tabGroup
	activeGroup int
	activeTab   int // sub-tab index within the active group
	width       int
}

func newTopBar() topBarModel {
	return topBarModel{
		groups: []tabGroup{
			{
				label:   "Content",
				hotkey:  "1",
				tabs:    []string{"Skills", "Agents", "MCP", "Rules", "Hooks", "Commands"},
				actions: []string{"+ Add", "Create"},
			},
			{
				label:   "Collections",
				hotkey:  "2",
				tabs:    []string{"Library", "Registries", "Loadouts"},
				actions: []string{"+ Add", "Create"},
			},
			{
				label:   "Config",
				hotkey:  "3",
				tabs:    []string{"Settings", "Sandbox"},
				actions: nil,
			},
		},
		activeGroup: 0, // Content
		activeTab:   0, // Skills
	}
}

// SetSize updates the topbar width.
func (t *topBarModel) SetSize(width int) {
	t.width = width
}

// ActiveGroupLabel returns the label of the active group.
func (t topBarModel) ActiveGroupLabel() string {
	return t.groups[t.activeGroup].label
}

// ActiveTabLabel returns the label of the active sub-tab.
func (t topBarModel) ActiveTabLabel() string {
	g := t.groups[t.activeGroup]
	if t.activeTab >= 0 && t.activeTab < len(g.tabs) {
		return g.tabs[t.activeTab]
	}
	return ""
}

// SetGroup switches to a group by index (0-based) and resets sub-tab to 0.
func (t *topBarModel) SetGroup(index int) tea.Cmd {
	if index < 0 || index >= len(t.groups) {
		return nil
	}
	t.activeGroup = index
	t.activeTab = 0
	return t.tabChangedCmd()
}

// NextTab moves to the next sub-tab within the active group (wraps).
func (t *topBarModel) NextTab() tea.Cmd {
	g := t.groups[t.activeGroup]
	t.activeTab = (t.activeTab + 1) % len(g.tabs)
	return t.tabChangedCmd()
}

// PrevTab moves to the previous sub-tab within the active group (wraps).
func (t *topBarModel) PrevTab() tea.Cmd {
	g := t.groups[t.activeGroup]
	t.activeTab = (t.activeTab - 1 + len(g.tabs)) % len(g.tabs)
	return t.tabChangedCmd()
}

// Update handles mouse clicks on groups, tabs, and action buttons.
func (t topBarModel) Update(msg tea.Msg) (topBarModel, tea.Cmd) {
	mouseMsg, ok := msg.(tea.MouseMsg)
	if !ok {
		return t, nil
	}
	if mouseMsg.Action != tea.MouseActionPress || mouseMsg.Button != tea.MouseButtonLeft {
		return t, nil
	}

	// Check group tab clicks
	for i := range t.groups {
		if zone.Get("group-" + itoa(i)).InBounds(mouseMsg) {
			cmd := t.SetGroup(i)
			return t, cmd
		}
	}

	// Check sub-tab clicks (only for active group)
	g := t.groups[t.activeGroup]
	for i := range g.tabs {
		if zone.Get("tab-" + itoa(t.activeGroup) + "-" + itoa(i)).InBounds(mouseMsg) {
			t.activeTab = i
			return t, t.tabChangedCmd()
		}
	}

	return t, nil
}

// Height returns the rendered height of the topbar (always 5: top border + groups + separator + tabs + bottom border).
func (t topBarModel) Height() int {
	return 5
}

// View renders the full bordered topbar with two-tier tabs.
func (t topBarModel) View() string {
	innerW := t.width - 2 // subtract left+right border chars

	// Row 1: group tabs
	groupRow := t.renderGroupRow(innerW)

	// Separator
	sep := "├" + strings.Repeat("─", innerW) + "┤"

	// Row 2: sub-tabs + action buttons
	tabRow := t.renderTabRow(innerW)

	// Top border with ──syllago── inline
	topBorder := t.renderTopBorder(innerW)

	// Bottom border
	botBorder := "╰" + strings.Repeat("─", innerW) + "╯"

	return strings.Join([]string{
		topBorder,
		"│" + groupRow + "│",
		sep,
		"│" + tabRow + "│",
		botBorder,
	}, "\n")
}

// renderTopBorder renders ╭──syllago────────...╮ with colored logo.
func (t topBarModel) renderTopBorder(innerW int) string {
	logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")
	logoW := lipgloss.Width(logo)
	prefix := "╭──"
	suffix := "╮"
	fill := innerW - logoW - 2 // -2 for the "──" before logo
	if fill < 0 {
		fill = 0
	}
	return prefix + logo + strings.Repeat("─", fill) + suffix
}

// renderGroupRow renders the group tabs: [1 Content]  2 Collections  3 Config
func (t topBarModel) renderGroupRow(innerW int) string {
	var parts []string
	for i, g := range t.groups {
		label := g.hotkey + " " + g.label
		var rendered string
		if i == t.activeGroup {
			rendered = activeTabStyle.Render(label)
		} else {
			rendered = inactiveTabStyle.Render(label)
		}
		rendered = zone.Mark("group-"+itoa(i), rendered)
		parts = append(parts, rendered)
	}
	content := strings.Join(parts, "  ")
	contentW := lipgloss.Width(content)
	totalPad := max(0, innerW-contentW)
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	return strings.Repeat(" ", leftPad) + content + strings.Repeat(" ", rightPad)
}

// renderTabRow renders sub-tabs on the left and action buttons on the right.
func (t topBarModel) renderTabRow(innerW int) string {
	g := t.groups[t.activeGroup]

	// Sub-tabs
	var tabParts []string
	for i, tab := range g.tabs {
		var rendered string
		if i == t.activeTab {
			rendered = activeTabStyle.Render(tab)
		} else {
			rendered = inactiveTabStyle.Render(tab)
		}
		rendered = zone.Mark("tab-"+itoa(t.activeGroup)+"-"+itoa(i), rendered)
		tabParts = append(tabParts, rendered)
	}
	left := strings.Join(tabParts, " ")

	// Action buttons
	var btnParts []string
	for _, action := range g.actions {
		btn := activeButtonStyle.Render(action)
		btnID := "btn-" + strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(action, "+ "), " ", "-"))
		btn = zone.Mark(btnID, btn)
		btnParts = append(btnParts, btn)
	}
	right := strings.Join(btnParts, " ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := max(1, innerW-leftW-rightW-1) // -1 for leading space

	return " " + left + strings.Repeat(" ", gap) + right
}

func (t topBarModel) tabChangedCmd() tea.Cmd {
	g := t.groups[t.activeGroup]
	label := ""
	if t.activeTab >= 0 && t.activeTab < len(g.tabs) {
		label = g.tabs[t.activeTab]
	}
	return func() tea.Msg {
		return tabChangedMsg{
			group:    t.activeGroup,
			tab:      t.activeTab,
			tabLabel: label,
		}
	}
}
