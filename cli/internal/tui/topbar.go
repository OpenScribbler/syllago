package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// tabGroup represents a top-level navigation group (Content, Collections, Config).
type tabGroup struct {
	label  string
	hotkey string   // display hint: "1", "2", "3"
	tabs   []string // sub-tab labels
}

// tabAction defines a clickable action button with its zone ID and action name.
type tabAction struct {
	label  string // e.g., "[a] Add"
	zone   string // bubblezone ID, e.g., "btn-add"
	action string // action name for actionPressedMsg, e.g., "add"
}

// tabChangedMsg is fired when the active group or sub-tab changes.
type tabChangedMsg struct {
	group    int
	tab      int
	tabLabel string
}

// actionPressedMsg is fired when an action button is activated (keyboard or mouse).
type actionPressedMsg struct {
	action string // "add", "remove", or "uninstall"
	group  string // which group was active (e.g., "Content", "Collections")
	tab    string // which sub-tab was active (e.g., "Skills", "Library")
}

// breadcrumbClickMsg is sent when a breadcrumb segment is clicked.
type breadcrumbClickMsg struct {
	index int // which segment was clicked (0 = first after the tab anchor)
}

// topBarModel manages a two-tier tab navigation bar with a bordered frame.
type topBarModel struct {
	groups      []tabGroup
	activeGroup int
	activeTab   int // sub-tab index within the active group
	width       int
	breadcrumbs []string // when non-empty, shown on row 3 under the active tab
}

func newTopBar() topBarModel {
	return topBarModel{
		groups: []tabGroup{
			{
				label:  "Collections",
				hotkey: "1",
				tabs:   []string{"Library", "Registries", "Loadouts"},
			},
			{
				label:  "Content",
				hotkey: "2",
				tabs:   []string{"Skills", "Agents", "MCP", "Rules", "Hooks", "Commands"},
			},
			{
				label:  "Config",
				hotkey: "3",
				tabs:   []string{"Settings", "Sandbox"},
			},
		},
		activeGroup: 0, // Collections
		activeTab:   0, // Library
	}
}

// tabActions returns the context-sensitive action buttons for the current tab.
func (t topBarModel) tabActions() []tabAction {
	switch t.ActiveTabLabel() {
	case "Library":
		return []tabAction{
			{"[a] Add", "btn-add", "add"},
			{"[d] Remove", "btn-remove", "remove"},
			{"[x] Uninstall", "btn-uninstall", "uninstall"},
		}
	case "Registries":
		return []tabAction{
			{"[a] Add", "btn-add", "add"},
			{"[d] Remove", "btn-remove", "remove"},
		}
	case "Loadouts":
		return []tabAction{
			{"[d] Remove", "btn-remove", "remove"},
		}
	case "Skills", "Agents", "MCP", "Rules", "Hooks", "Commands":
		return []tabAction{
			{"[a] Add", "btn-add", "add"},
			{"[d] Remove", "btn-remove", "remove"},
			{"[x] Uninstall", "btn-uninstall", "uninstall"},
		}
	default: // Config tabs
		return nil
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

// SetBreadcrumbs sets the breadcrumb trail shown under the active tab.
func (t *topBarModel) SetBreadcrumbs(crumbs []string) {
	t.breadcrumbs = crumbs
}

// ClearBreadcrumbs removes the breadcrumb trail.
func (t *topBarModel) ClearBreadcrumbs() {
	t.breadcrumbs = nil
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

	// Check breadcrumb clicks
	for i := range t.breadcrumbs {
		if zone.Get("crumb-" + itoa(i)).InBounds(mouseMsg) {
			idx := i
			return t, func() tea.Msg { return breadcrumbClickMsg{index: idx} }
		}
	}

	// Check action button clicks (context-sensitive per tab)
	for _, a := range t.tabActions() {
		if zone.Get(a.zone).InBounds(mouseMsg) {
			return t, t.actionCmd(a.action)
		}
	}
	// Help button click
	if zone.Get("btn-help").InBounds(mouseMsg) {
		return t, func() tea.Msg { return helpToggleMsg{} }
	}

	return t, nil
}

// ActionCmd creates an actionPressedMsg for the given action name.
func (t topBarModel) actionCmd(action string) tea.Cmd {
	return func() tea.Msg {
		return actionPressedMsg{
			action: action,
			group:  t.ActiveGroupLabel(),
			tab:    t.ActiveTabLabel(),
		}
	}
}

// Height returns the rendered height of the topbar (always 6: top border + groups + separator + tabs + breadcrumbs + bottom border).
func (t topBarModel) Height() int {
	return 6
}

// View renders the full bordered topbar with two-tier tabs and breadcrumb row.
func (t topBarModel) View() string {
	innerW := t.width - 2 // subtract left+right border chars

	// Row 1: group tabs
	groupRow := t.renderGroupRow(innerW)

	// Separator
	sep := "├" + strings.Repeat("─", innerW) + "┤"

	// Row 2: sub-tabs + action buttons
	tabRow := t.renderTabRow(innerW)

	// Row 3: breadcrumbs (blank when no drill-in)
	crumbRow := t.renderBreadcrumbRow(innerW)

	// Top border with ──syllago── inline
	topBorder := t.renderTopBorder(innerW)

	// Bottom border
	botBorder := "╰" + strings.Repeat("─", innerW) + "╯"

	return strings.Join([]string{
		topBorder,
		"│" + groupRow + "│",
		sep,
		"│" + tabRow + "│",
		"│" + crumbRow + "│",
		botBorder,
	}, "\n")
}

// helpToggleMsg is sent when the help button in the topbar corner is clicked.
type helpToggleMsg struct{}

// renderTopBorder renders ╭──syllago────...──[?]──╮ with colored logo and help button.
// The [?] is inset 2 dashes from the right edge, mirroring the logo's 2-dash inset on the left.
func (t topBarModel) renderTopBorder(innerW int) string {
	logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")
	logoW := lipgloss.Width(logo)
	prefix := "╭──"
	suffix := "──╮"

	helpBtn := zone.Mark("btn-help", mutedStyle.Render("[?]"))
	helpW := lipgloss.Width(helpBtn)

	// -2 for "──" before logo, -2 for "──" after [?]
	fill := innerW - logoW - 2 - helpW - 2
	if fill < 0 {
		fill = 0
	}
	return prefix + logo + strings.Repeat("─", fill) + helpBtn + suffix
}

// renderGroupRow renders the group tabs: [1 Content]  2 Collections  3 Config
func (t topBarModel) renderGroupRow(innerW int) string {
	var parts []string
	for i, g := range t.groups {
		label := "[" + g.hotkey + "] " + g.label
		var rendered string
		if i == t.activeGroup {
			rendered = activeGroupStyle.Render(label)
		} else {
			rendered = inactiveGroupStyle.Render(label)
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

// renderTabRow renders sub-tabs only (buttons moved to breadcrumb row).
func (t topBarModel) renderTabRow(innerW int) string {
	g := t.groups[t.activeGroup]

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
	leftW := lipgloss.Width(left)
	pad := max(0, innerW-leftW-1)
	return " " + left + strings.Repeat(" ", pad)
}

// renderBreadcrumbRow renders breadcrumbs (left) and action buttons (right).
func (t topBarModel) renderBreadcrumbRow(innerW int) string {
	// Right side: action buttons
	actions := t.tabActions()
	var btnParts []string
	for _, a := range actions {
		btn := activeButtonStyle.Render(a.label)
		btn = zone.Mark(a.zone, btn)
		btnParts = append(btnParts, btn)
	}
	right := strings.Join(btnParts, " ")
	rightW := lipgloss.Width(right)

	// Left side: breadcrumbs (if any)
	left := ""
	if len(t.breadcrumbs) > 0 {
		// Calculate the x-offset so ">" aligns with the first letter of the active tab.
		g := t.groups[t.activeGroup]
		offset := 1
		for i := 0; i < t.activeTab && i < len(g.tabs); i++ {
			offset += len(g.tabs[i]) + 4
			offset++
		}
		offset += 2

		var parts []string
		for i, crumb := range t.breadcrumbs {
			seg := zone.Mark("crumb-"+itoa(i), sectionTitleStyle.Render("> ")+boldStyle.Render(truncate(crumb, 30)))
			parts = append(parts, seg)
		}
		trail := strings.Join(parts, " ")
		trailW := lipgloss.Width(trail)

		// Clamp offset so trail + buttons fit
		maxTrailSpace := innerW - rightW - 1
		if offset+trailW > maxTrailSpace {
			offset = max(1, maxTrailSpace-trailW)
		}
		left = strings.Repeat(" ", offset) + trail
	}

	leftW := lipgloss.Width(left)
	gap := max(1, innerW-leftW-rightW)
	if left == "" {
		gap = max(0, innerW-rightW)
	}

	line := left + strings.Repeat(" ", gap) + right
	lineW := lipgloss.Width(line)
	if lineW < innerW {
		line += strings.Repeat(" ", innerW-lineW)
	}
	return lipgloss.NewStyle().MaxWidth(innerW).Render(line)
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
