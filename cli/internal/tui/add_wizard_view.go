package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/add"
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

// --- Source step view ---

func (m *addWizardModel) viewSource() string {
	pad := "  "

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Where is the content?")

	var lines []string
	lines = append(lines, title, "")

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
		lines = append(lines, row)

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
		if i == m.providerCursor {
			lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(cursor+name))
		} else {
			lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render(cursor+name))
		}
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
		if i == m.registryCursor {
			lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(cursor+name))
		} else {
			lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render(cursor+name))
		}
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

	lines = append(lines, pad+"    "+style.Render(displayVal))

	// Inline error
	if m.sourceErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(dangerColor)
		lines = append(lines, pad+"    "+errStyle.Render(m.sourceErr))
	}

	return lines
}

// --- Type step view ---

func (m *addWizardModel) viewType() string {
	pad := "  "

	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("What type of content?")

	var lines []string
	lines = append(lines, title, "")
	lines = append(lines, m.typeChecks.View())
	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[space] toggle  [a] all  [n] none  [enter] next  [esc] back"))

	return strings.Join(lines, "\n")
}

// --- Discovery step view ---

func (m *addWizardModel) viewDiscovery() string {
	pad := "  "

	if m.discovering {
		return pad + lipgloss.NewStyle().Foreground(primaryColor).Render("Discovering content...")
	}

	if m.discoveryErr != "" {
		var lines []string
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
		lines = append(lines, pad+mutedStyle.Render("[r] Retry  [Esc] Back"))
		return strings.Join(lines, "\n")
	}

	if len(m.discoveredItems) == 0 {
		var lines []string
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("No content found"))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("[Esc] Back"))
		return strings.Join(lines, "\n")
	}

	var lines []string
	selected := len(m.discoveryList.SelectedIndices())
	total := len(m.discoveredItems)
	header := fmt.Sprintf("Found %d items (%d selected)", total, selected)
	lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(header))
	lines = append(lines, "")
	lines = append(lines, m.discoveryList.View())
	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[space] toggle  [a] all  [n] none  [enter/→] next  [esc] back"))

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
	lines = append(lines, "")

	// Risk banner
	if len(m.risks) > 0 {
		riskView := m.riskBanner.View()
		lines = append(lines, riskView)
		lines = append(lines, "")
	}

	// Item list
	for i, item := range selected {
		cursor := "  "
		if m.reviewZone == addReviewZoneItems && i == m.reviewItemCursor {
			cursor = "> "
		}

		name := item.displayName
		if name == "" {
			name = item.name
		}
		nameStyled := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(name)
		typeStyled := mutedStyle.Render("(" + string(item.itemType) + ")")

		var statusStyled string
		switch item.status {
		case add.StatusNew:
			statusStyled = lipgloss.NewStyle().Foreground(successColor).Render("new")
		case add.StatusOutdated:
			statusStyled = lipgloss.NewStyle().Foreground(warningColor).Render("update — content differs")
		case add.StatusInLibrary:
			statusStyled = mutedStyle.Render("in library")
		}

		line := pad + cursor + nameStyled + " " + typeStyled
		if statusStyled != "" {
			line += " " + statusStyled
		}
		lines = append(lines, line)
	}

	// Buttons
	lines = append(lines, "")
	btnFocus := -1
	if m.reviewZone == addReviewZoneButtons {
		btnFocus = m.buttonCursor
	}
	addLabel := fmt.Sprintf("Add %d items", len(selected))
	buttons := renderModalButtons(btnFocus, usableW, pad, nil,
		buttonDef{"Cancel", "add-cancel", 0},
		buttonDef{"Back", "add-back", 1},
		buttonDef{addLabel, "add-confirm", 2},
	)
	lines = append(lines, buttons)

	return strings.Join(lines, "\n")
}

// --- Execute step view ---

func (m *addWizardModel) viewExecute() string {
	pad := "  "
	selected := m.selectedItems()

	var lines []string

	if m.executeDone {
		// Count results
		added, updated, errors, cancelled := 0, 0, 0, 0
		for _, r := range m.executeResults {
			switch r.status {
			case "added":
				added++
			case "updated":
				updated++
			case "error":
				errors++
			case "cancelled":
				cancelled++
			}
		}

		total := added + updated
		header := fmt.Sprintf("Done! %d items added to library.", total)
		lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(successColor).Render(header))
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
		lines = append(lines, pad+mutedStyle.Render("[Enter] Go to Library"))
	} else if m.executeCancelled {
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("Cancelling..."))
	} else {
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("[Esc] Cancel remaining"))
	}

	return strings.Join(lines, "\n")
}
