package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// galleryPane tracks which pane is focused within the gallery.
type galleryPane int

const (
	paneGrid galleryPane = iota
	paneSidebar
)

// cardSelectedMsg is sent when the selected card changes.
type cardSelectedMsg struct {
	card *cardData
}

// cardDrillMsg is sent when Enter is pressed on a card.
type cardDrillMsg struct {
	card *cardData
}

// galleryModel orchestrates the card grid + contents sidebar within a bordered frame.
type galleryModel struct {
	grid     cardGridModel
	sidebar  contentsSidebarModel
	focus    galleryPane
	width    int
	height   int
	tabLabel string // "Loadouts" or "Registries"
}

func newGalleryModel() galleryModel {
	return galleryModel{
		grid:    newCardGridModel(nil),
		sidebar: newContentsSidebarModel(),
		focus:   paneGrid,
	}
}

// SetSize recalculates child dimensions.
func (g *galleryModel) SetSize(width, height int) {
	g.width = width
	g.height = height

	innerW := max(0, width-borderSize)
	paneH := max(3, height-borderSize-metaBarLines-1)

	sidebarW := max(20, innerW*30/100)
	gridW := max(20, innerW-sidebarW-1) // -1 for vertical divider

	g.grid.SetSize(gridW, paneH)
	g.sidebar.SetSize(sidebarW, paneH)
}

// SetCards replaces the card data and updates the sidebar.
func (g *galleryModel) SetCards(cards []cardData, tabLabel string) {
	g.tabLabel = tabLabel
	g.grid = newCardGridModel(cards)
	g.grid.focused = g.focus == paneGrid
	g.SetSize(g.width, g.height)
	g.updateSidebar()
}

// updateSidebar refreshes the sidebar from the selected card.
func (g *galleryModel) updateSidebar() {
	card := g.grid.Selected()
	if card != nil && len(card.items) > 0 {
		g.sidebar.SetCard(card)
	} else if card != nil {
		// Build groups from counts map (loadouts without resolved items)
		var groups []contentGroup
		for _, k := range sortedKeys(card.counts) {
			groups = append(groups, contentGroup{
				typeName: k,
			})
		}
		g.sidebar.SetGroups(card, groups)
	} else {
		g.sidebar.SetCard(nil)
	}
}

// Update handles key and mouse input.
func (g galleryModel) Update(msg tea.Msg) (galleryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return g.updateKeys(msg)
	case tea.MouseMsg:
		return g.updateMouse(msg)
	}
	return g, nil
}

func (g galleryModel) updateKeys(msg tea.KeyMsg) (galleryModel, tea.Cmd) {
	switch msg.String() {
	case "tab":
		g.toggleFocus()
		return g, nil

	case "enter":
		if g.focus == paneGrid {
			card := g.grid.Selected()
			if card != nil {
				return g, func() tea.Msg { return cardDrillMsg{card: card} }
			}
		}
		return g, nil
	}

	switch g.focus {
	case paneGrid:
		return g.updateGridKeys(msg)
	case paneSidebar:
		return g.updateSidebarKeys(msg)
	}
	return g, nil
}

func (g galleryModel) updateGridKeys(msg tea.KeyMsg) (galleryModel, tea.Cmd) {
	prevCursor := g.grid.cursor
	switch msg.String() {
	case keyUp, "up":
		g.grid.CursorUp()
	case keyDown, "down":
		g.grid.CursorDown()
	case keyLeft, "left":
		g.grid.CursorLeft()
	case keyRight, "right":
		g.grid.CursorRight()
	case "home":
		g.grid.cursor = 0
		g.grid.offset = 0
	case "end":
		if len(g.grid.cards) > 0 {
			g.grid.cursor = len(g.grid.cards) - 1
			g.grid.scrollToCursor()
		}
	}

	if g.grid.cursor != prevCursor {
		g.updateSidebar()
		card := g.grid.Selected()
		return g, func() tea.Msg { return cardSelectedMsg{card: card} }
	}
	return g, nil
}

func (g galleryModel) updateSidebarKeys(msg tea.KeyMsg) (galleryModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		g.sidebar.ScrollDown()
	case keyUp, "up":
		g.sidebar.ScrollUp()
	}
	return g, nil
}

func (g galleryModel) updateMouse(msg tea.MouseMsg) (galleryModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Edit button click
		if zone.Get("meta-edit").InBounds(msg) {
			return g, func() tea.Msg { return libraryEditMsg{} }
		}

		for i := range g.grid.cards {
			if zone.Get("card-" + itoa(i)).InBounds(msg) {
				if g.grid.cursor == i {
					// Double-click: drill in
					card := g.grid.Selected()
					if card != nil {
						return g, func() tea.Msg { return cardDrillMsg{card: card} }
					}
				}
				g.grid.cursor = i
				g.setFocus(paneGrid)
				g.updateSidebar()
				card := g.grid.Selected()
				return g, func() tea.Msg { return cardSelectedMsg{card: card} }
			}
		}
	}

	// Scroll wheel
	if msg.Action == tea.MouseActionPress {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if g.focus == paneGrid {
				g.grid.CursorUp()
				g.updateSidebar()
			} else {
				g.sidebar.ScrollUp()
			}
		case tea.MouseButtonWheelDown:
			if g.focus == paneGrid {
				g.grid.CursorDown()
				g.updateSidebar()
			} else {
				g.sidebar.ScrollDown()
			}
		}
	}

	return g, nil
}

// toggleFocus switches between grid and sidebar.
func (g *galleryModel) toggleFocus() {
	if g.focus == paneGrid {
		g.setFocus(paneSidebar)
	} else {
		g.setFocus(paneGrid)
	}
}

func (g *galleryModel) setFocus(pane galleryPane) {
	g.focus = pane
	g.grid.focused = pane == paneGrid
	g.sidebar.focused = pane == paneSidebar
}

// View renders the gallery layout: metadata + grid|sidebar inside a bordered frame.
func (g galleryModel) View() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}

	innerW := max(0, g.width-borderSize)
	paneH := max(3, g.height-borderSize-metaBarLines-1)

	sidebarW := max(20, innerW*30/100)
	gridW := max(20, innerW-sidebarW-1)

	// Metadata panel for selected card
	metaContent := g.renderMetadata(innerW)

	// Separator with T-junction
	separator := sectionRuleStyle.Render("├" + strings.Repeat("─", gridW) + "┬" + strings.Repeat("─", sidebarW) + "┤")

	// Render panes
	g.grid.SetSize(gridW, paneH)
	g.sidebar.SetSize(sidebarW, paneH)

	gridLines := strings.Split(g.grid.View(), "\n")
	sidebarLines := strings.Split(g.sidebar.View(), "\n")

	for len(gridLines) < paneH {
		gridLines = append(gridLines, strings.Repeat(" ", gridW))
	}
	for len(sidebarLines) < paneH {
		sidebarLines = append(sidebarLines, strings.Repeat(" ", sidebarW))
	}

	border := sectionRuleStyle.Render
	topBorder := border("╭" + strings.Repeat("─", innerW) + "╮")
	bottomBorder := border("╰" + strings.Repeat("─", gridW) + "┴" + strings.Repeat("─", sidebarW) + "╯")

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if gap := w - lipgloss.Width(s); gap > 0 {
			s += strings.Repeat(" ", gap)
		}
		return s
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, border("│")+wrapLine(ml, innerW)+border("│"))
	}
	lines = append(lines, separator)
	for i := 0; i < paneH; i++ {
		gl := ""
		if i < len(gridLines) {
			gl = gridLines[i]
		}
		sl := ""
		if i < len(sidebarLines) {
			sl = sidebarLines[i]
		}
		lines = append(lines, border("│")+wrapLine(gl, gridW)+border("│")+wrapLine(sl, sidebarW)+border("│"))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// renderMetadata returns 3 lines of card-level metadata.
func (g galleryModel) renderMetadata(width int) string {
	card := g.grid.Selected()
	if card == nil {
		blank := strings.Repeat(" ", width)
		return blank + "\n" + blank + "\n" + blank
	}

	gap := "  "

	// Line 1: Name + type + item count
	nameMaxW := 40
	name := truncate(sanitizeLine(card.name), nameMaxW)
	line1 := " " + boldStyle.Render(padRight(name, nameMaxW))
	line1 += gap + boldStyle.Render(g.tabLabel)

	totalItems := 0
	for _, v := range card.counts {
		totalItems += v
	}
	line1 += gap + mutedStyle.Render(itoa(totalItems)+" items")

	if card.subtitle != "" {
		line1 += gap + mutedStyle.Render(truncate(sanitizeLine(card.subtitle), max(10, width-lipgloss.Width(line1)-2)))
	}

	// Line 2: Status + count breakdown
	line2 := " " + boldStyle.Render("Status: ") + mutedStyle.Render(card.status)
	countParts := make([]string, 0, len(card.counts))
	for _, k := range sortedKeys(card.counts) {
		countParts = append(countParts, itoa(card.counts[k])+" "+k)
	}
	if len(countParts) > 0 {
		line2 += gap + mutedStyle.Render(strings.Join(countParts, ", "))
	}

	// Line 3: Description + [e] Edit button right-aligned
	editBtn := zone.Mark("meta-edit", activeButtonStyle.Render("[e] Edit"))
	editBtnW := lipgloss.Width(editBtn)

	line3 := ""
	if card.desc != "" {
		maxDescW := max(10, width-editBtnW-3) // -3 for padding
		line3 = " " + mutedStyle.Render(truncate(sanitizeLine(card.desc), maxDescW))
	}
	line3W := lipgloss.Width(line3)
	btnGap := max(1, width-line3W-editBtnW)
	line3 += strings.Repeat(" ", btnGap) + editBtn

	pad := func(s string) string {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
		if g := width - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}
	return pad(line1) + "\n" + pad(line2) + "\n" + pad(line3)
}
