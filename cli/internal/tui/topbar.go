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

// actionPressedMsg is fired when an action button is activated (keyboard or mouse).
type actionPressedMsg struct {
	action string // "add" or "create"
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
				label:   "Collections",
				hotkey:  "1",
				tabs:    []string{"Library", "Registries", "Loadouts"},
				actions: []string{"[a] Add", "[n] Create"},
			},
			{
				label:   "Content",
				hotkey:  "2",
				tabs:    []string{"Skills", "Agents", "MCP", "Rules", "Hooks", "Commands"},
				actions: []string{"[a] Add", "[n] Create"},
			},
			{
				label:   "Config",
				hotkey:  "3",
				tabs:    []string{"Settings", "Sandbox"},
				actions: nil,
			},
		},
		activeGroup: 0, // Collections
		activeTab:   0, // Library
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

	// Check action button clicks
	if zone.Get("btn-add").InBounds(mouseMsg) {
		return t, t.actionCmd("add")
	}
	if zone.Get("btn-create").InBounds(mouseMsg) {
		return t, t.actionCmd("create")
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
	btnZones := []string{"btn-add", "btn-create"}
	for i, action := range g.actions {
		btn := activeButtonStyle.Render(action)
		if i < len(btnZones) {
			btn = zone.Mark(btnZones[i], btn)
		}
		btnParts = append(btnParts, btn)
	}
	right := strings.Join(btnParts, " ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := max(1, innerW-leftW-rightW-1) // -1 for leading space

	return " " + left + strings.Repeat(" ", gap) + right
}

// renderBreadcrumbRow renders the breadcrumb trail aligned under the active tab.
// Returns an empty padded line when no breadcrumbs are set.
func (t topBarModel) renderBreadcrumbRow(innerW int) string {
	if len(t.breadcrumbs) == 0 {
		return strings.Repeat(" ", innerW)
	}

	// Calculate the x-offset so ">" aligns with the first letter of the active tab.
	// Tab row: " " (1) + for each preceding tab: Padding(0,2) adds 2+label+2 + 1 space gap.
	// The active tab's first letter is at offset + 2 (left padding).
	g := t.groups[t.activeGroup]
	offset := 1 // leading " " in the tab row
	for i := 0; i < t.activeTab && i < len(g.tabs); i++ {
		offset += len(g.tabs[i]) + 4 // label + Padding(0,2) = +4
		offset++                     // space between tabs
	}
	offset += 2 // skip the active tab's left padding so ">" aligns with first letter

	// Build the breadcrumb string with clickable zones
	var parts []string
	for i, crumb := range t.breadcrumbs {
		seg := zone.Mark("crumb-"+itoa(i), sectionTitleStyle.Render("> ")+boldStyle.Render(truncate(crumb, 30)))
		parts = append(parts, seg)
	}
	trail := strings.Join(parts, " ")
	trailW := lipgloss.Width(trail)

	// Clamp offset so trail fits
	if offset+trailW > innerW {
		offset = max(1, innerW-trailW)
	}

	line := strings.Repeat(" ", offset) + trail
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
