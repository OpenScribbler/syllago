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
		retryBtn := zone.Mark("add-retry", mutedStyle.Render("[r] Retry"))
		backBtn := zone.Mark("add-err-back", mutedStyle.Render("[Esc] Back"))
		lines = append(lines, pad+retryBtn+"  "+backBtn)
		return strings.Join(lines, "\n")
	}

	if len(m.discoveredItems) == 0 {
		var lines []string
		lines = append(lines, m.renderTitleRow("No content found", true, ""))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("No content found"))
		lines = append(lines, "")
		lines = append(lines, pad+zone.Mark("add-empty-back", mutedStyle.Render("[Esc] Back")))
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
	pad := "  "
	usableW := m.width - 4

	selected := m.selectedItems()
	header := fmt.Sprintf("Adding %d items to library:", len(selected))

	var lines []string
	lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(header))

	// Buttons at the top, below the header
	lines = append(lines, "")
	btnFocus := -1
	if m.reviewZone == addReviewZoneButtons {
		btnFocus = m.buttonCursor
	}
	addLabel := fmt.Sprintf("Add %d items", len(selected))
	buttons := renderModalButtons(btnFocus, usableW, pad, nil,
		buttonDef{addLabel, "add-confirm", 0},
		buttonDef{"Back", "add-back", 1},
		buttonDef{"Cancel", "add-cancel", 2},
	)
	lines = append(lines, buttons)
	lines = append(lines, "")

	// Risk banner
	if len(m.risks) > 0 {
		riskView := m.riskBanner.View()
		lines = append(lines, riskView)
		lines = append(lines, "")
	}

	// Microcopy
	lines = append(lines, pad+mutedStyle.Render("Review your selections. Press Enter on an item to inspect its files."))
	lines = append(lines, pad+mutedStyle.Render("Items marked with ! or !! contain executable code — inspect before adding."))
	lines = append(lines, "")

	// Item list with column headers
	cols := m.reviewColumns()
	reviewHdr := pad + "  " + // cursor space
		boldStyle.Render(padRight("Name", cols.name)) + " " +
		boldStyle.Render(padRight("Type", cols.ctype)) + " " +
		boldStyle.Render(padRight("Status", cols.status)) + " " +
		boldStyle.Render(padRight("Risk", cols.risk))
	lines = append(lines, truncateLine(reviewHdr, m.width))

	for i, item := range selected {
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
			statusColor = successColor
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

		nameCol := padRight(truncate(sanitizeLine(name), cols.name), cols.name)
		typeCol := padRight(truncate(typeLbl, cols.ctype), cols.ctype)

		var statusCol string
		if statusColor != nil {
			statusCol = lipgloss.NewStyle().Foreground(statusColor).Render(padRight(truncate(statusLbl, cols.status), cols.status))
		} else {
			statusCol = padRight(truncate(statusLbl, cols.status), cols.status)
		}

		var riskCol string
		switch riskLbl {
		case "!!":
			riskCol = lipgloss.NewStyle().Foreground(dangerColor).Render(padRight(riskLbl, cols.risk))
		case "!":
			riskCol = lipgloss.NewStyle().Foreground(warningColor).Render(padRight(riskLbl, cols.risk))
		default:
			riskCol = padRight("", cols.risk)
		}

		isCursor := m.reviewZone == addReviewZoneItems && i == m.reviewItemCursor

		// Build the full row with colored columns, then apply background for cursor
		line := pad + cursor + nameCol + " " + typeCol + " " + statusCol + " " + riskCol
		line = truncateLine(line, m.width)

		if isCursor {
			// Pad to full width then apply selectedRowStyle background
			if gap := m.width - len([]rune(line)); gap > 0 {
				line += strings.Repeat(" ", gap)
			}
			line = selectedRowStyle.MaxWidth(m.width).Render(line)
		}

		lines = append(lines, zone.Mark(fmt.Sprintf("add-rev-item-%d", i), line))
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

	// Per-item progress
	for i, item := range selected {
		var icon, statusText string
		if i < len(m.executeResults) {
			r := m.executeResults[i]
			switch r.status {
			case "added":
				icon = lipgloss.NewStyle().Foreground(successColor).Render("✓")
				statusText = "Added"
			case "updated":
				icon = lipgloss.NewStyle().Foreground(successColor).Render("✓")
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
				icon = mutedStyle.Render("-")
				statusText = "Skipped"
			default:
				if i == m.executeCurrent && m.executing {
					icon = lipgloss.NewStyle().Foreground(primaryColor).Render("◐")
					statusText = "Adding..."
				} else {
					icon = mutedStyle.Render(" ")
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

	if m.executeDone {
		lines = append(lines, "")
		lines = append(lines, pad+zone.Mark("add-exec-done", mutedStyle.Render("[Enter] Go to Library")))
	} else if m.executeCancelled {
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("Cancelling..."))
	} else {
		lines = append(lines, "")
		lines = append(lines, pad+zone.Mark("add-exec-cancel", mutedStyle.Render("[Esc] Cancel remaining")))
	}

	return strings.Join(lines, "\n")
}
