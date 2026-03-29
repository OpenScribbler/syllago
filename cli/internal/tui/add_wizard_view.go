package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// View renders the add wizard.
func (m *addWizardModel) View() string {
	if m == nil {
		return ""
	}

	header := m.shell.View()

	var content string
	switch m.step {
	case addStepSource:
		content = m.viewSource()
	case addStepType:
		content = m.viewType()
	case addStepDiscovery:
		content = m.viewDiscovery()
	case addStepReview:
		content = m.viewReview()
	case addStepExecute:
		content = m.viewExecute()
	}

	output := header + "\n" + content
	outputLines := strings.Count(output, "\n") + 1
	if outputLines < m.height {
		output += strings.Repeat("\n", m.height-outputLines)
	}
	return output
}

// renderTitleRow renders a step title on the left and nav buttons on the right, on the same line.
// showBack=false hides the Back button (e.g., Source step has no prior step).
// nextLabel overrides the default "Next" label (e.g., "Close" for Execute).
func (m *addWizardModel) renderTitleRow(title string, showBack bool, nextLabel string) string {
	pad := "  "
	if nextLabel == "" {
		nextLabel = "Next"
	}

	titleRendered := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(title)
	titleW := lipgloss.Width(titleRendered)

	var buttons []buttonDef
	if showBack {
		buttons = append(buttons, buttonDef{"Back", "add-nav-back", 0})
		buttons = append(buttons, buttonDef{nextLabel, "add-nav-next", 1})
	} else {
		buttons = append(buttons, buttonDef{nextLabel, "add-nav-next", 0})
	}

	// Render buttons with no focus (pass -1 so none is highlighted)
	var parts []string
	for _, b := range buttons {
		style := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		parts = append(parts, zone.Mark(b.zoneID, style.Render(b.label)))
	}
	btns := strings.Join(parts, "  ")
	btnsW := lipgloss.Width(btns)

	gap := max(1, m.width-titleW-btnsW-4)
	return titleRendered + strings.Repeat(" ", gap) + btns
}

// --- Source step view ---

func (m *addWizardModel) viewSource() string {
	pad := "  "

	var lines []string
	lines = append(lines, m.renderTitleRow("Where is the content?", false, ""))
	lines = append(lines, "")

	type sourceOption struct {
		label    string
		desc     string
		disabled bool
	}

	options := []sourceOption{
		{"Provider", "Import from a detected provider", len(m.providers) == 0},
		{"Registry", "Import from a configured registry", len(m.registries) == 0},
		{"Local Path", "Import from a local directory", false},
		{"Git URL", "Clone a git repository", false},
	}

	for i, opt := range options {
		var row string
		cursor := "  "
		if i == m.sourceCursor {
			cursor = "> "
		}

		if opt.disabled {
			var reason string
			if i == 0 {
				reason = "(no providers detected)"
			} else {
				reason = "(no registries configured)"
			}
			row = pad + mutedStyle.Render(cursor+opt.label+"  "+reason)
		} else if i == m.sourceCursor {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(cursor+opt.label) +
				"  " + mutedStyle.Render(opt.desc)
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(cursor+opt.label) +
				"  " + mutedStyle.Render(opt.desc)
		}
		lines = append(lines, zone.Mark(fmt.Sprintf("add-src-%d", i), row))

		// Render expanded sub-list or text input inline
		if m.sourceExpanded && i == m.sourceCursor {
			switch i {
			case 0:
				lines = append(lines, m.viewProviderSubList(pad)...)
			case 1:
				lines = append(lines, m.viewRegistrySubList(pad)...)
			}
		}
		if m.inputActive && i == m.sourceCursor && (i == 2 || i == 3) {
			lines = append(lines, m.viewPathInput(pad, i == 3)...)
		}
	}

	return strings.Join(lines, "\n")
}

func (m *addWizardModel) viewProviderSubList(pad string) []string {
	var lines []string
	for i, prov := range m.providers {
		cursor := "    "
		if i == m.providerCursor {
			cursor = "  > "
		}
		name := prov.Name
		var row string
		if i == m.providerCursor {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(cursor+name)
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(cursor+name)
		}
		lines = append(lines, zone.Mark(fmt.Sprintf("add-prov-%d", i), row))
	}
	return lines
}

func (m *addWizardModel) viewRegistrySubList(pad string) []string {
	var lines []string
	for i, reg := range m.registries {
		cursor := "    "
		if i == m.registryCursor {
			cursor = "  > "
		}
		name := reg.Name
		var row string
		if i == m.registryCursor {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(cursor+name)
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(cursor+name)
		}
		lines = append(lines, zone.Mark(fmt.Sprintf("add-reg-%d", i), row))
	}
	return lines
}

func (m *addWizardModel) viewPathInput(pad string, isGit bool) []string {
	var lines []string

	placeholder := "Enter absolute path..."
	if isGit {
		placeholder = "Enter git URL (https://, git@, ssh://)..."
	}

	// Text input field
	fieldW := m.width - 8
	if fieldW < 20 {
		fieldW = 20
	}

	bg := inputActiveBG
	var displayVal string
	runes := []rune(m.pathInput)
	if len(runes) == 0 {
		displayVal = mutedStyle.Render(placeholder)
	} else if m.pathCursor >= len(runes) {
		displayVal = m.pathInput + "\u2588"
	} else {
		before := string(runes[:m.pathCursor])
		under := string(runes[m.pathCursor : m.pathCursor+1])
		after := string(runes[m.pathCursor+1:])
		cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
		displayVal = before + cursorChar + after
	}

	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		MaxWidth(fieldW).
		Padding(0, 1)

	lines = append(lines, zone.Mark("add-path-input", pad+"    "+style.Render(displayVal)))

	// Inline error
	if m.sourceErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(dangerColor)
		lines = append(lines, pad+"    "+errStyle.Render(m.sourceErr))
	}

	return lines
}

// --- Type step view ---

func (m *addWizardModel) viewType() string {
	var lines []string
	lines = append(lines, m.renderTitleRow("What type of content?", true, ""))
	lines = append(lines, "")
	lines = append(lines, m.typeChecks.View())

	return strings.Join(lines, "\n")
}

// --- Discovery step view ---

func (m *addWizardModel) viewDiscovery() string {
	pad := "  "

	if m.discovering {
		return m.renderTitleRow("Discovering content...", true, "") + "\n" +
			pad + lipgloss.NewStyle().Foreground(primaryColor).Render("Scanning...")
	}

	if m.discoveryErr != "" {
		var lines []string
		lines = append(lines, m.renderTitleRow("Discovery Error", true, ""))
		lines = append(lines, "")
		errBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dangerColor).
			Foreground(dangerColor).
			Padding(1, 2).
			MaxWidth(m.width - 4).
			Render("Error: " + m.discoveryErr)
		lines = append(lines, errBox)
		lines = append(lines, "")
		btnStyle := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		retryBtn := zone.Mark("add-retry", btnStyle.Render("Retry"))
		backBtn := zone.Mark("add-err-back", btnStyle.Render("Back"))
		lines = append(lines, pad+retryBtn+"  "+backBtn)
		return strings.Join(lines, "\n")
	}

	if len(m.discoveredItems) == 0 {
		var lines []string
		lines = append(lines, m.renderTitleRow("No content found", true, ""))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("No content found"))
		lines = append(lines, "")
		emptyBtnStyle := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		lines = append(lines, pad+zone.Mark("add-empty-back", emptyBtnStyle.Render("Back")))
		return strings.Join(lines, "\n")
	}

	selected := len(m.discoveryList.SelectedIndices())
	actionable := m.actionableCount
	header := fmt.Sprintf("Found %d items (%d selected)", len(m.discoveredItems), selected)
	if m.installedCount > 0 && !m.showInstalled {
		header = fmt.Sprintf("Found %d new items (%d selected)", actionable, selected)
	}

	var lines []string
	lines = append(lines, m.renderTitleRow(header, true, ""))
	lines = append(lines, "")
	lines = append(lines, m.discoveryHeader())
	lines = append(lines, m.discoveryList.View())

	// Installed items toggle
	if m.installedCount > 0 {
		lines = append(lines, "")
		if m.showInstalled {
			toggle := zone.Mark("add-installed-toggle",
				mutedStyle.Render(fmt.Sprintf("  [h] Hide %d already-installed items", m.installedCount)))
			lines = append(lines, toggle)
		} else {
			toggle := zone.Mark("add-installed-toggle",
				mutedStyle.Render(fmt.Sprintf("  [h] Show %d already-installed items", m.installedCount)))
			lines = append(lines, toggle)
		}
	}

	return strings.Join(lines, "\n")
}

// --- Review step view ---

func (m *addWizardModel) viewReview() string {
	if m.reviewDrillIn {
		return m.viewReviewDrillIn()
	}

	pad := "  "

	selected := m.selectedItems()
	header := fmt.Sprintf("Adding %d items to library:", len(selected))

	// Title on left, buttons right-aligned on the same line
	titleRendered := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(header)

	btnFocus := -1
	if m.reviewZone == addReviewZoneButtons {
		btnFocus = m.buttonCursor
	}
	addLabel := fmt.Sprintf("Add %d items", len(selected))

	// Build button string (right-aligned)
	var btnParts []string
	for _, b := range []buttonDef{
		{addLabel, "add-confirm", 0},
		{"Back", "add-back", 1},
		{"Cancel", "add-cancel", 2},
	} {
		style := lipgloss.NewStyle().Padding(0, 2)
		if btnFocus == b.focusAt {
			style = style.Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
				Background(accentColor)
		} else {
			style = style.
				Foreground(primaryText).
				Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		}
		btnParts = append(btnParts, zone.Mark(b.zoneID, style.Render(b.label)))
	}
	btnsStr := strings.Join(btnParts, "  ")
	btnsW := lipgloss.Width(btnsStr)
	titleW := lipgloss.Width(titleRendered)
	btnGap := max(2, m.width-titleW-btnsW-6)

	var lines []string
	lines = append(lines, pad+titleRendered+strings.Repeat(" ", btnGap)+btnsStr)

	// Microcopy
	lines = append(lines, pad+mutedStyle.Render("Enter to inspect files. Items with ! or !! contain executable code."))

	// Item list with column headers
	cols := m.reviewColumns()
	reviewHdr := pad + "  " + // cursor space
		boldStyle.Render(padRight("Name", cols.name)) + " " +
		boldStyle.Render(padRight("Risk", cols.risk)) + " " +
		boldStyle.Render(padRight("Type", cols.ctype)) + " " +
		boldStyle.Render(padRight("Status", cols.status))
	lines = append(lines, truncateLine(reviewHdr, m.width))

	// Window the item list to the visible range using reviewItemOffset
	vh := m.reviewVisibleHeight()
	endIdx := m.reviewItemOffset + vh
	if endIdx > len(selected) {
		endIdx = len(selected)
	}
	startIdx := m.reviewItemOffset
	if startIdx > len(selected) {
		startIdx = len(selected)
	}

	for i := startIdx; i < endIdx; i++ {
		item := selected[i]
		cursor := "  "
		if m.reviewZone == addReviewZoneItems && i == m.reviewItemCursor {
			cursor = "> "
		}

		name := item.displayName
		if name == "" {
			name = item.name
		}

		typeLbl := typeLabel(item.itemType)

		var statusLbl string
		var statusColor lipgloss.TerminalColor
		switch item.status {
		case add.StatusNew:
			statusLbl = "new"
			statusColor = primaryColor
		case add.StatusOutdated:
			statusLbl = "update"
			statusColor = warningColor
		case add.StatusInLibrary:
			statusLbl = "in library"
			statusColor = mutedColor
		}

		var riskLbl string
		if len(item.risks) > 0 {
			hasHigh := false
			for _, r := range item.risks {
				if r.Level == catalog.RiskHigh {
					hasHigh = true
					break
				}
			}
			if hasHigh {
				riskLbl = "!!"
			} else {
				riskLbl = "!"
			}
		}

		isCursor := m.reviewZone == addReviewZoneItems && i == m.reviewItemCursor

		// Build columns with appropriate styling
		nameText := padRight(truncate(sanitizeLine(name), cols.name), cols.name)
		typeText := padRight(truncate(typeLbl, cols.ctype), cols.ctype)
		statusText := padRight(truncate(statusLbl, cols.status), cols.status)

		var nameCol, typeCol, statusCol, riskCol string
		if isCursor {
			// Cursor row: bold name + background on each column, preserve status/risk fg colors
			bg := selectedBG
			nameCol = lipgloss.NewStyle().Bold(true).Foreground(primaryText).Background(bg).Render(nameText)
			typeCol = lipgloss.NewStyle().Foreground(primaryText).Background(bg).Render(typeText)
			if statusColor != nil {
				statusCol = lipgloss.NewStyle().Foreground(statusColor).Background(bg).Render(statusText)
			} else {
				statusCol = lipgloss.NewStyle().Background(bg).Render(statusText)
			}
			switch riskLbl {
			case "!!":
				riskCol = lipgloss.NewStyle().Foreground(dangerColor).Background(bg).Render(padRight(riskLbl, cols.risk))
			case "!":
				riskCol = lipgloss.NewStyle().Foreground(warningColor).Background(bg).Render(padRight(riskLbl, cols.risk))
			default:
				riskCol = lipgloss.NewStyle().Background(bg).Render(padRight("", cols.risk))
			}
			bgSpace := lipgloss.NewStyle().Background(bg).Render(" ")
			cursorStyled := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Background(bg).Render(cursor)
			row := pad + cursorStyled + nameCol + bgSpace + riskCol + bgSpace + typeCol + bgSpace + statusCol
			// Pad to full width with background, then hard-clip
			rowW := lipgloss.Width(row)
			if rowW < m.width {
				row += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", m.width-rowW))
			}
			line := lipgloss.NewStyle().MaxWidth(m.width).Render(row)
			lines = append(lines, zone.Mark(fmt.Sprintf("add-rev-item-%d", i), line))
		} else {
			// Normal row — explicit foreground to prevent background bleed
			nameCol = lipgloss.NewStyle().Foreground(primaryText).Render(nameText)
			typeCol = lipgloss.NewStyle().Foreground(primaryText).Render(typeText)
			if statusColor != nil {
				statusCol = lipgloss.NewStyle().Foreground(statusColor).Render(statusText)
			} else {
				statusCol = lipgloss.NewStyle().Foreground(primaryText).Render(statusText)
			}
			switch riskLbl {
			case "!!":
				riskCol = lipgloss.NewStyle().Foreground(dangerColor).Render(padRight(riskLbl, cols.risk))
			case "!":
				riskCol = lipgloss.NewStyle().Foreground(warningColor).Render(padRight(riskLbl, cols.risk))
			default:
				riskCol = padRight("", cols.risk)
			}
			line := pad + cursor + nameCol + " " + riskCol + " " + typeCol + " " + statusCol
			line = truncateLine(line, m.width)
			// Pad to full width to reset any lingering background
			if lineW := lipgloss.Width(line); lineW < m.width {
				line += strings.Repeat(" ", m.width-lineW)
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("add-rev-item-%d", i), line))
		}
	}

	// Per-item risk info at the bottom — fixed height area
	innerLines := reviewRiskBoxLines - 2
	var itemRisks []catalog.RiskIndicator
	if m.reviewItemCursor < len(selected) {
		itemRisks = selected[m.reviewItemCursor].risks
	}
	if len(itemRisks) > 0 {
		hasHigh := false
		var riskContent []string
		maxShow := innerLines
		for i, r := range itemRisks {
			if i >= maxShow {
				remaining := len(itemRisks) - maxShow
				riskContent = append(riskContent, mutedStyle.Render(fmt.Sprintf("+%d more — drill in to see all", remaining)))
				break
			}
			prefix := "! "
			color := warningColor
			if r.Level == catalog.RiskHigh {
				prefix = "!! "
				color = dangerColor
				hasHigh = true
			}
			riskContent = append(riskContent, lipgloss.NewStyle().Foreground(color).Render(prefix+r.Label)+
				" — "+mutedStyle.Render(truncate(r.Description, m.width-20)))
		}
		for len(riskContent) < innerLines {
			riskContent = append(riskContent, "")
		}
		borderColor := warningColor
		if hasHigh {
			borderColor = dangerColor
		}
		riskBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			MaxWidth(m.width-4).
			Padding(0, 1).
			Render(strings.Join(riskContent, "\n"))
		lines = append(lines, riskBox)
	} else {
		for i := 0; i < reviewRiskBoxLines; i++ {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// viewReviewDrillIn renders the file tree + preview for a selected review item.
func (m *addWizardModel) viewReviewDrillIn() string {
	selected := m.selectedItems()
	name := ""
	if m.reviewItemCursor < len(selected) {
		name = selected[m.reviewItemCursor].displayName
		if name == "" {
			name = selected[m.reviewItemCursor].name
		}
	}

	var lines []string
	lines = append(lines, m.renderTitleRow("Inspecting: "+name, true, ""))

	// Compute pane dimensions
	innerW := m.width - borderSize
	paneH := max(5, m.height-7) // shell(3) + title(1) + blank(1) + border(2)

	// Always show the file tree pane for consistent drill-in experience
	treeW := max(18, innerW*30/100)
	previewW := innerW - treeW - 1

	m.reviewDrillTree.SetSize(treeW, paneH)
	m.reviewDrillPreview.SetSize(previewW, paneH)

	mBorder := mutedStyle.Render                                   // inactive border
	fBorder := lipgloss.NewStyle().Foreground(primaryColor).Render // focused border

	// Pick border style per pane
	treeBorder := mBorder
	prevBorder := mBorder
	if m.reviewDrillTree.focused {
		treeBorder = fBorder
	} else {
		prevBorder = fBorder
	}

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if gap := w - lipgloss.Width(s); gap > 0 {
			s += strings.Repeat(" ", gap)
		}
		return s
	}

	// Top border with focus coloring
	lines = append(lines,
		treeBorder("╭")+treeBorder(strings.Repeat("─", treeW))+
			mBorder("┬")+
			prevBorder(strings.Repeat("─", previewW))+prevBorder("╮"))

	// Build tree + preview content
	treeContent := strings.Split(m.reviewDrillTree.View(), "\n")
	for len(treeContent) < paneH {
		treeContent = append(treeContent, strings.Repeat(" ", treeW))
	}

	previewHeader := renderSectionTitle(m.reviewDrillPreview.fileName, previewW)
	previewContent := []string{previewHeader}
	bodyH := max(0, paneH-1)
	if bodyH > 0 {
		body := m.renderDrillInPreviewBody(bodyH, previewW)
		previewContent = append(previewContent, strings.Split(body, "\n")...)
	}
	for len(previewContent) < paneH {
		previewContent = append(previewContent, strings.Repeat(" ", previewW))
	}

	// Pane rows
	for i := 0; i < paneH; i++ {
		tl := ""
		if i < len(treeContent) {
			tl = treeContent[i]
		}
		pl := ""
		if i < len(previewContent) {
			pl = previewContent[i]
		}
		lines = append(lines,
			treeBorder("│")+wrapLine(tl, treeW)+
				mBorder("│")+
				wrapLine(pl, previewW)+prevBorder("│"))
	}

	// Bottom border
	lines = append(lines,
		treeBorder("╰")+treeBorder(strings.Repeat("─", treeW))+
			mBorder("┴")+
			prevBorder(strings.Repeat("─", previewW))+prevBorder("╯"))

	return strings.Join(lines, "\n")
}

// renderDrillInPreviewBody renders the preview content for the drill-in view,
// with support for highlighted risk lines (same pattern as install wizard).
func (m *addWizardModel) renderDrillInPreviewBody(height, width int) string {
	p := &m.reviewDrillPreview
	if len(p.lines) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("No preview available")
	}

	linesAbove := p.offset
	lastVisible := min(p.offset+height, len(p.lines))
	linesBelow := max(0, len(p.lines)-lastVisible)
	showAbove := linesAbove > 0
	showBelow := linesBelow > 0

	contentStart := p.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++
	}
	if showBelow && contentEnd > contentStart {
		contentEnd--
	}

	lineNumW := len(fmt.Sprintf("%d", len(p.lines)))
	if lineNumW < 2 {
		lineNumW = 2
	}

	lines := make([]string, 0, height)

	if showAbove {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more above)", linesAbove)))
	}

	for i := contentStart; i < contentEnd; i++ {
		lineNum := i + 1
		if p.highlightLines != nil && p.highlightLines[lineNum] {
			// Highlighted line: danger gutter marker + tinted background
			num := lipgloss.NewStyle().Foreground(dangerColor).Render(fmt.Sprintf("%*d", lineNumW, lineNum))
			gutterChar := lipgloss.NewStyle().Foreground(dangerColor).Render("\u258c")
			lineW := width - lipgloss.Width(num) - 1
			lineContent := truncateLine(p.lines[i], lineW)
			padded := lineContent + strings.Repeat(" ", max(0, lineW-lipgloss.Width(lineContent)))
			styledLine := lipgloss.NewStyle().Background(highlightBG).Foreground(primaryText).Render(padded)
			lines = append(lines, num+gutterChar+styledLine)
		} else {
			num := mutedStyle.Render(fmt.Sprintf("%*d ", lineNumW, lineNum))
			numW := lipgloss.Width(num)
			lineW := width - numW
			line := truncateLine(p.lines[i], lineW)
			lines = append(lines, num+line)
		}
	}

	if showBelow {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more below)", linesBelow)))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

// reviewColumns computes column widths for the review table.
func (m *addWizardModel) reviewColumns() discoveryColLayout {
	// Reuse the same layout as discovery for visual consistency
	return m.discoveryColumns()
}

// --- Execute step view ---

func (m *addWizardModel) viewExecute() string {
	pad := "  "
	selected := m.selectedItems()

	var lines []string

	if m.executeDone {
		// Count results
		added, updated := 0, 0
		for _, r := range m.executeResults {
			switch r.status {
			case "added":
				added++
			case "updated":
				updated++
			}
		}

		total := added + updated
		header := fmt.Sprintf("Done! %d items added to library.", total)
		// Use success color for the done header, with Close button on the right
		titleRendered := pad + lipgloss.NewStyle().Bold(true).Foreground(successColor).Render(header)
		closeStyle := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		closeBtn := zone.Mark("add-nav-next", closeStyle.Render("Close"))
		titleW := lipgloss.Width(titleRendered)
		btnW := lipgloss.Width(closeBtn)
		gap := max(1, m.width-titleW-btnW-4)
		lines = append(lines, titleRendered+strings.Repeat(" ", gap)+closeBtn)
	} else {
		lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Adding items..."))
	}
	lines = append(lines, "")

	// Per-item progress (windowed to available height)
	execH := max(3, m.height-8) // shell(3) + header(1) + blank(1) + footer(2) + pad(1)

	// Auto-scroll to keep current item visible
	if m.executeCurrent >= m.executeOffset+execH {
		m.executeOffset = m.executeCurrent - execH + 1
	}
	endIdx := min(m.executeOffset+execH, len(selected))

	for i := m.executeOffset; i < endIdx; i++ {
		item := selected[i]
		var icon, statusText string
		if i < len(m.executeResults) {
			r := m.executeResults[i]
			switch r.status {
			case "added":
				icon = lipgloss.NewStyle().Foreground(successColor).Render("●")
				statusText = "Added"
			case "updated":
				icon = lipgloss.NewStyle().Foreground(successColor).Render("●")
				statusText = "Updated"
			case "error":
				icon = lipgloss.NewStyle().Foreground(dangerColor).Render("✗")
				errMsg := "Error"
				if r.err != nil {
					errMsg = "Error: " + r.err.Error()
				}
				statusText = errMsg
			case "cancelled":
				icon = mutedStyle.Render("-")
				statusText = "Cancelled"
			case "skipped":
				icon = mutedStyle.Render("○")
				statusText = "Skipped — same version already in library"
			default:
				if i == m.executeCurrent && m.executing {
					icon = lipgloss.NewStyle().Foreground(primaryColor).Render("◐")
					statusText = "Adding..."
				} else {
					icon = mutedStyle.Render("○")
					statusText = "Pending"
				}
			}
		}

		name := item.displayName
		if name == "" {
			name = item.name
		}
		line := pad + icon + " " + name + "  " + mutedStyle.Render(statusText)
		lines = append(lines, line)
	}

	// Scroll indicator
	if m.executeOffset > 0 {
		lines = append(lines, pad+mutedStyle.Render(fmt.Sprintf("(%d more above)", m.executeOffset)))
	}
	if endIdx < len(selected) {
		lines = append(lines, pad+mutedStyle.Render(fmt.Sprintf("(%d more below)", len(selected)-endIdx)))
	}

	if m.executeDone {
		lines = append(lines, "")
		// Styled action buttons
		btnStyle := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		closeBtn := zone.Mark("add-exec-done", btnStyle.Render("Go to Library"))
		addMoreBtn := zone.Mark("add-exec-restart", btnStyle.Render("Add More"))
		lines = append(lines, pad+closeBtn+"  "+addMoreBtn)
	} else if m.executeCancelled {
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("Cancelling..."))
	} else {
		lines = append(lines, "")
		cancelBtnStyle := lipgloss.NewStyle().Padding(0, 2).
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		lines = append(lines, pad+zone.Mark("add-exec-cancel", cancelBtnStyle.Render("Cancel remaining")))
	}

	return strings.Join(lines, "\n")
}
