package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// tierBadge returns a colored "● Label" string for a confidence tier.
// Returns empty string for items that are not content-signal detected.
func tierBadge(tier analyzer.ConfidenceTier) string {
	dot := "●"
	switch tier {
	case analyzer.TierLow:
		return lipgloss.NewStyle().Foreground(warningColor).Render(dot + " Low confidence (content-signal)")
	case analyzer.TierMedium:
		return lipgloss.NewStyle().Foreground(primaryColor).Render(dot + " Medium confidence (content-signal)")
	case analyzer.TierHigh:
		return lipgloss.NewStyle().Foreground(successColor).Render(dot + " High confidence (content-signal)")
	case analyzer.TierUser:
		return lipgloss.NewStyle().Foreground(accentColor).Render(dot + " User-asserted (content-signal)")
	}
	return ""
}

// View renders the install wizard.
func (m *installWizardModel) View() string {
	if m == nil {
		return ""
	}

	// Wizard shell (step breadcrumbs)
	header := m.shell.View()

	// Per-step content
	var content string
	switch m.step {
	case installStepProvider:
		content = m.viewProvider()
	case installStepLocation:
		content = m.viewLocation()
	case installStepMethod:
		content = m.viewMethod()
	case installStepReview:
		content = m.viewReview()
	case installStepConflict:
		content = m.viewConflict()
	}

	// Pad to fill content area so helpbar stays at the bottom
	output := header + "\n" + content
	outputLines := strings.Count(output, "\n") + 1
	if outputLines < m.height {
		output += strings.Repeat("\n", m.height-outputLines)
	}
	return output
}

// viewProvider renders the provider picker step.
func (m *installWizardModel) viewProvider() string {
	pad := "  "
	usableW := m.width - 4 // rough usable width for button alignment

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Install %q to which provider?", m.itemName))

	var lines []string
	lines = append(lines, title, "")

	for i, prov := range m.providers {
		var row string
		switch {
		case m.providerInstalled[i]:
			// Already installed: muted
			row = pad + "  " + lipgloss.NewStyle().Foreground(mutedColor).Render(
				prov.Name+" (already installed)")
		case i == m.providerCursor:
			// Selected (cursor): bold accent with arrow
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(
				"> "+prov.Name) + " " + lipgloss.NewStyle().Foreground(mutedColor).Render("(detected)")
		default:
			// Normal: primary text
			row = pad + "  " + lipgloss.NewStyle().Foreground(primaryText).Render(
				prov.Name) + " " + lipgloss.NewStyle().Foreground(mutedColor).Render("(detected)")
		}
		lines = append(lines, zone.Mark(fmt.Sprintf("inst-prov-%d", i), row))
	}

	// "All providers" option — shown when 2+ providers exist.
	if m.showAllOption() {
		// Divider
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(mutedColor).Render("─────"))
		var allRow string
		if m.selectAll {
			allRow = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("> All providers")
		} else {
			allRow = pad + lipgloss.NewStyle().Foreground(primaryText).Render("  All providers") +
				"  " + lipgloss.NewStyle().Foreground(mutedColor).Render("([a] to select)")
		}
		lines = append(lines, zone.Mark("inst-all", allRow))
	}

	// Buttons: Cancel and Next. Next is always visually focused (focusAt=1).
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Cancel", "inst-cancel", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

// viewConflict renders the conflict resolution step for "install to all providers".
func (m *installWizardModel) viewConflict() string {
	pad := "  "
	usableW := m.width - 4

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		"Install-path conflict detected")

	var lines []string
	lines = append(lines, title, "")

	// Describe each conflict
	for _, c := range m.conflicts {
		readers := make([]string, len(c.AlsoReadBy))
		for i, r := range c.AlsoReadBy {
			readers[i] = r.Name
		}
		desc := fmt.Sprintf("%s and %s share a read path:", c.InstallingTo.Name, strings.Join(readers, ", "))
		lines = append(lines,
			pad+lipgloss.NewStyle().Foreground(warningColor).Render("! ")+
				lipgloss.NewStyle().Foreground(primaryText).Render(desc))
		lines = append(lines,
			pad+"  "+lipgloss.NewStyle().Foreground(mutedColor).Render(c.SharedPath))
	}
	lines = append(lines, "")
	lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render("How do you want to resolve this?"))
	lines = append(lines, "")

	// 3 resolution options
	type conflictOpt struct {
		label string
		desc  string
		id    string
	}
	opts := []conflictOpt{
		{"Shared path only", "Install once to the shared path — other providers pick it up automatically", "inst-conflict-0"},
		{"Own dirs only", "Install to each provider's separate directory — skip the shared path", "inst-conflict-1"},
		{"Install to all", "Install everywhere (may cause duplicate content warnings)", "inst-conflict-2"},
	}

	for i, opt := range opts {
		prefix := "  "
		if i == m.conflictCursor {
			prefix = "> "
		}
		var labelRow string
		if i == m.conflictCursor {
			labelRow = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+opt.label)
		} else {
			labelRow = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+opt.label)
		}
		descRow := pad + "    " + lipgloss.NewStyle().Foreground(mutedColor).Render(opt.desc)
		lines = append(lines, zone.Mark(opt.id, labelRow))
		lines = append(lines, descRow)
		lines = append(lines, "")
	}

	// Buttons: Back and Install
	lines = append(lines, renderModalButtons(1, usableW, pad, []string{"Install"},
		buttonDef{"Back", "inst-back", 0},
		buttonDef{"Install", "inst-conflict-install", 1},
	))

	return strings.Join(lines, "\n")
}

// renderLocationInput renders the custom path text input with cursor.
func (m *installWizardModel) renderLocationInput(usableW int) string {
	fieldW := usableW - 2 // padding inside field
	bg := inputInactiveBG
	if m.locationCursor == 2 {
		bg = inputActiveBG
	}

	var displayVal string
	if m.locationCursor == 2 {
		// Show cursor
		runes := []rune(m.customPath)
		if m.customCursor >= len(runes) {
			displayVal = truncate(m.customPath+"\u2588", fieldW)
		} else {
			before := string(runes[:m.customCursor])
			under := string(runes[m.customCursor : m.customCursor+1])
			after := string(runes[m.customCursor+1:])
			cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
			displayVal = truncate(before+cursorChar+after, fieldW)
		}
	} else {
		if m.customPath == "" {
			displayVal = ""
		} else {
			displayVal = truncate(m.customPath, fieldW)
		}
	}

	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		Width(usableW).
		Padding(0, 1)
	return zone.Mark("inst-custom-path", style.Render(displayVal))
}

func (m *installWizardModel) viewLocation() string {
	pad := "  "
	usableW := m.width - 4

	provName := m.providers[m.providerCursor].Name
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Install location for %s:", provName))

	var lines []string
	lines = append(lines, title, "")

	labels := []string{"Global", "Project", "Custom"}
	for i, label := range labels {
		var row string
		prefix := "  "
		if i == m.locationCursor {
			prefix = "> "
		}

		if i < 2 {
			// Global or Project: show resolved path
			path := m.resolvedInstallPath(i)
			pathStr := lipgloss.NewStyle().Foreground(mutedColor).Render("(" + path + ")")
			if i == m.locationCursor {
				row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+label) +
					" " + pathStr
			} else {
				row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+label) +
					" " + pathStr
			}
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-loc-%d", i), row))
		} else {
			// Custom: show label + text input on same line
			if i == m.locationCursor {
				row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+label) +
					"   "
			} else {
				row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+label) +
					"   "
			}
			inputField := m.renderLocationInput(usableW - lipgloss.Width(row))
			lines = append(lines, zone.Mark(fmt.Sprintf("inst-loc-%d", i), row+inputField))
		}
	}

	// Buttons: Back and Next
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Back", "inst-back", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

func (m *installWizardModel) viewMethod() string {
	pad := "  "
	usableW := m.width - 4

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Install method:")

	var lines []string
	lines = append(lines, title, "")

	// Option 0: Symlink
	{
		prefix := "  "
		if m.methodCursor == 0 {
			prefix = "> "
		}
		var row string
		if m.symlinkDisabled() {
			row = pad + lipgloss.NewStyle().Foreground(mutedColor).Render(
				prefix+"Symlink   (not supported for this provider)")
		} else if m.methodCursor == 0 {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+"Symlink") +
				"   " + lipgloss.NewStyle().Foreground(mutedColor).Render("(recommended — stays in sync with library)")
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+"Symlink") +
				"   " + lipgloss.NewStyle().Foreground(mutedColor).Render("(recommended — stays in sync with library)")
		}
		lines = append(lines, zone.Mark("inst-method-0", row))
	}

	// Option 1: Copy
	{
		prefix := "  "
		if m.methodCursor == 1 {
			prefix = "> "
		}
		var row string
		if m.methodCursor == 1 {
			row = pad + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(prefix+"Copy") +
				"      " + lipgloss.NewStyle().Foreground(mutedColor).Render("(standalone copy, won't auto-update)")
		} else {
			row = pad + lipgloss.NewStyle().Foreground(primaryText).Render(prefix+"Copy") +
				"      " + lipgloss.NewStyle().Foreground(mutedColor).Render("(standalone copy, won't auto-update)")
		}
		lines = append(lines, zone.Mark("inst-method-1", row))
	}

	// Buttons: Back and Next
	lines = append(lines, "")
	lines = append(lines, renderModalButtons(1, usableW, pad, nil,
		buttonDef{"Back", "inst-back", 0},
		buttonDef{"Next", "inst-next", 1},
	))

	return strings.Join(lines, "\n")
}

func (m *installWizardModel) viewReview() string {
	pad := "  "
	usableW := m.width - 4

	// --- Summary + buttons above the frame ---
	prov := m.providers[m.providerCursor]
	provName := prov.Name
	var summaryLines []string
	summaryLines = append(summaryLines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(
		fmt.Sprintf("Installing %q to %s", m.itemName, provName)))

	if m.isJSONMerge {
		// Show the actual settings file path for JSON merge
		targetPath := m.resolveSettingsPath(prov)
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Target:   ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(targetPath))
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Method:   ")+
			lipgloss.NewStyle().Foreground(primaryText).Render("JSON merge"))
	} else {
		locLabels := []string{"Global", "Project", "Custom"}
		locLabel := locLabels[m.locationCursor]
		locPath := m.resolvedInstallPath(m.locationCursor)
		methodLabel := "Symlink"
		if m.methodCursor == 1 {
			methodLabel = "Copy"
		}
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Target:   ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(locPath))
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Scope:    ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(locLabel))
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Method:   ")+
			lipgloss.NewStyle().Foreground(primaryText).Render(methodLabel))
	}

	// Detection line — only shown for content-signal items
	if m.item.Meta != nil && m.item.Meta.DetectionSource != "" {
		tier := analyzer.TierForMeta(m.item.Meta.Confidence, m.item.Meta.DetectionMethod)
		badge := tierBadge(tier)
		if badge != "" {
			summaryLines = append(summaryLines, pad+mutedStyle.Render("Detection: ")+badge)
		}
	}

	// Trust line — MOAT AD-7 Panel C9. Rendered only when the item carries
	// a trust surface (Verified green check / Recalled red X). Items from
	// git registries or without a computed tier produce TrustBadgeNone and
	// the row is suppressed — absence is intentional per AD-7.
	if trustBadge := catalog.UserFacingBadge(m.item.TrustTier, m.item.Recalled); trustBadge != catalog.TrustBadgeNone {
		var glyphStyle lipgloss.Style
		switch trustBadge {
		case catalog.TrustBadgeVerified:
			glyphStyle = lipgloss.NewStyle().Foreground(successColor).Bold(true)
		case catalog.TrustBadgeRecalled:
			glyphStyle = lipgloss.NewStyle().Foreground(dangerColor).Bold(true)
		}
		trustText := catalog.TrustDescription(m.item.TrustTier, m.item.Recalled, m.item.RecallReason)
		summaryLines = append(summaryLines, pad+mutedStyle.Render("Trust:     ")+
			glyphStyle.Render(trustBadge.Glyph())+" "+
			lipgloss.NewStyle().Foreground(primaryText).Render(trustText))
	}

	// Buttons on the last line of the summary area
	btnFocus := -1
	if m.reviewZone == reviewZoneButtons {
		btnFocus = m.buttonCursor
	}
	buttons := renderModalButtons(btnFocus, usableW, pad,
		[]string{"Install"},
		buttonDef{"Cancel", "inst-cancel", 0},
		buttonDef{"Back", "inst-back", 1},
		buttonDef{"Install", "inst-install", 2},
	)
	summaryLines = append(summaryLines, buttons)

	summary := strings.Join(summaryLines, "\n")

	// --- Compute frame dimensions ---
	border := sectionRuleStyle.Render
	innerW := m.width - borderSize
	summaryH := len(summaryLines) + 1 // +1 for blank line after summary
	shellH := 3                       // wizard shell header
	frameH := max(6, m.height-shellH-summaryH)
	frameInnerH := max(3, frameH-borderSize)

	// Risk section height
	riskH := 0
	if len(m.risks) > 0 {
		riskH = len(m.risks)
	}

	// Separator between risk and panes = 1 line
	sepH := 0
	if riskH > 0 {
		sepH = 1
	}

	// Pane height (tree + preview area)
	paneH := max(3, frameInnerH-riskH-sepH)

	// --- Determine layout: tree+preview or preview only ---
	showTree := m.hasMultipleFiles()
	treeInnerW := 0
	previewInnerW := innerW
	if showTree {
		treeInnerW = max(18, innerW*30/100)
		if innerW >= 100 {
			treeInnerW = 30
		}
		previewInnerW = innerW - treeInnerW - 1 // -1 for vertical divider
	}

	// Size sub-models
	m.reviewTree.SetSize(treeInnerW, paneH)
	m.reviewPreview.SetSize(previewInnerW, paneH)

	// --- Build the frame ---
	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if gap := w - lipgloss.Width(s); gap > 0 {
			s += strings.Repeat(" ", gap)
		}
		return s
	}

	var frameLines []string

	// Top border: ╭─ Risk Indicators ───...──╮ or ╭──────╮
	if riskH > 0 {
		riskTitle := " Risk Indicators "
		riskTitleStyled := lipgloss.NewStyle().Bold(true).Foreground(m.riskBanner.borderColor()).Render(riskTitle)
		titleVisualW := lipgloss.Width(riskTitle)
		// innerW = total chars between ╭ and ╮
		// Layout: ╭ + ─ + title + dashes + ╮
		//         ^   ^content^^^^^^^^^^^^   ^
		// content = 1 + titleVisualW + dashesRight = innerW
		dashesRight := max(1, innerW-1-titleVisualW)
		frameLines = append(frameLines,
			border("╭")+border("─")+riskTitleStyled+border(strings.Repeat("─", dashesRight))+border("╮"))
	} else {
		frameLines = append(frameLines, border("╭"+strings.Repeat("─", innerW)+"╮"))
	}

	// Risk items (if any)
	if riskH > 0 {
		riskView := m.riskBanner.ViewInline(innerW, m.reviewZone == reviewZoneRisks)
		for _, rl := range strings.Split(riskView, "\n") {
			frameLines = append(frameLines, border("│")+wrapLine(rl, innerW)+border("│"))
		}
	}

	// Separator between risks/top and panes
	if showTree {
		if riskH > 0 {
			frameLines = append(frameLines, border("├"+strings.Repeat("─", treeInnerW)+"┬"+strings.Repeat("─", previewInnerW)+"┤"))
		} else {
			// Top border already drawn; add separator for tree|preview split
			// Replace the generic top border with a split one
			frameLines[0] = border("╭" + strings.Repeat("─", treeInnerW) + "┬" + strings.Repeat("─", previewInnerW) + "╮")
		}
	} else {
		if riskH > 0 {
			frameLines = append(frameLines, border("├"+strings.Repeat("─", innerW)+"┤"))
		}
		// If no risks and no tree, we just have the top border already
	}

	// Build tree and preview content
	treeContent := strings.Split(m.reviewTree.View(), "\n")
	for len(treeContent) < paneH {
		treeContent = append(treeContent, strings.Repeat(" ", treeInnerW))
	}

	// Preview: render header + body
	previewHeader := renderSectionTitle(m.reviewPreview.fileName, previewInnerW)
	previewBodyH := max(0, paneH-1)
	m.reviewPreview.SetSize(previewInnerW, previewBodyH+1) // +1 for header line consumed by View
	previewContent := []string{previewHeader}
	if previewBodyH > 0 {
		previewBody := m.renderReviewPreviewBody(previewBodyH, previewInnerW)
		previewContent = append(previewContent, strings.Split(previewBody, "\n")...)
	}
	for len(previewContent) < paneH {
		previewContent = append(previewContent, strings.Repeat(" ", previewInnerW))
	}

	// Pane rows
	for i := 0; i < paneH; i++ {
		if showTree {
			tl := ""
			if i < len(treeContent) {
				tl = treeContent[i]
			}
			pl := ""
			if i < len(previewContent) {
				pl = previewContent[i]
			}
			frameLines = append(frameLines, border("│")+wrapLine(tl, treeInnerW)+border("│")+wrapLine(pl, previewInnerW)+border("│"))
		} else {
			pl := ""
			if i < len(previewContent) {
				pl = previewContent[i]
			}
			frameLines = append(frameLines, border("│")+wrapLine(pl, innerW)+border("│"))
		}
	}

	// Bottom border
	if showTree {
		frameLines = append(frameLines, border("╰"+strings.Repeat("─", treeInnerW)+"┴"+strings.Repeat("─", previewInnerW)+"╯"))
	} else {
		frameLines = append(frameLines, border("╰"+strings.Repeat("─", innerW)+"╯"))
	}

	// --- Assemble ---
	var result []string
	result = append(result, summary)
	result = append(result, "") // blank line between summary/buttons and frame
	result = append(result, frameLines...)

	return strings.Join(result, "\n")
}

// renderReviewPreviewBody renders just the preview content lines (no header),
// similar to libraryModel.renderPreviewBody. This is needed because the preview
// model's View() includes its own header which we render separately.
func (m *installWizardModel) renderReviewPreviewBody(height, width int) string {
	p := &m.reviewPreview
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
