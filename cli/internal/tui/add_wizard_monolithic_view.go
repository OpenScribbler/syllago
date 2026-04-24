package tui

import (
	"fmt"
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// viewMonolithicDiscovery renders the discovery step when addSourceMonolithic
// is active (D2, D18). One row per discovered file with path, line count,
// H2 count, scope, in-library badge, and a checkbox mark for the selection
// state. Zone marks per row + a title-row nav band for mouse parity.
func (m *addWizardModel) viewMonolithicDiscovery() string {
	pad := "  "
	var lines []string
	lines = append(lines, m.renderTitleRow("Select monolithic rule files to import", true, ""))
	lines = append(lines, "")

	if m.discovering {
		lines = append(lines, pad+mutedStyle.Render("Scanning project and home directory for monolithic rule files..."))
		return strings.Join(lines, "\n")
	}
	if m.discoveryErr != "" {
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(dangerColor).Render("Error: "+m.discoveryErr))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("[esc] back"))
		return strings.Join(lines, "\n")
	}
	if len(m.discoveryCandidates) == 0 {
		lines = append(lines, pad+mutedStyle.Render("No monolithic rule files found under project or home directory."))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("[esc] back"))
		return strings.Join(lines, "\n")
	}

	selected := make(map[int]bool, len(m.selectedCandidates))
	for _, idx := range m.selectedCandidates {
		selected[idx] = true
	}

	for i, c := range m.discoveryCandidates {
		cursor := "  "
		if i == m.discoveryCandidateCurs {
			cursor = "> "
		}
		mark := "◯"
		if selected[i] {
			mark = "◉"
		}
		scope := "[" + c.Scope + "]"
		libTag := ""
		if c.InLibrary {
			libTag = "  " + lipgloss.NewStyle().Foreground(successColor).Render("✓ in library")
		}
		sizeErr := ""
		if c.SizeErr != "" {
			sizeErr = "  " + lipgloss.NewStyle().Foreground(dangerColor).Render("(unreadable)")
		}
		rowText := fmt.Sprintf("%s%s %s   %dL  %dH2  %s%s%s",
			cursor, mark, c.RelPath, c.Lines, c.H2Count, scope, libTag, sizeErr)
		style := lipgloss.NewStyle().Foreground(primaryText)
		if i == m.discoveryCandidateCurs {
			style = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
		}
		row := pad + style.Render(rowText)
		lines = append(lines, zone.Mark(fmt.Sprintf("add-mono-disc-row-%d", i), row))
	}

	lines = append(lines, "")
	selCount := len(m.selectedCandidates)
	lines = append(lines, pad+mutedStyle.Render(fmt.Sprintf("%d of %d selected — [space] toggle  [a] all  [n] none  [enter] next  [esc] back",
		selCount, len(m.discoveryCandidates))))

	return strings.Join(lines, "\n")
}

// viewHeuristic renders the Heuristic step radio list (Task 4.4).
func (m *addWizardModel) viewHeuristic() string {
	pad := "  "
	var lines []string
	lines = append(lines, m.renderTitleRow("How should we split these files?", true, ""))
	lines = append(lines, "")

	type heurOption struct {
		id    string
		label string
	}
	opts := []heurOption{
		{"h2", "By H2 (default)"},
		{"h3", "By H3"},
		{"h4", "By H4"},
		{"marker", "By literal marker"},
		{"single", "Import as single rule"},
	}

	for i, opt := range opts {
		cursor := "  "
		if i == m.heuristicCursor {
			cursor = "> "
		}
		mark := "◯"
		if i == m.heuristicCursor {
			mark = "◉"
		}
		style := lipgloss.NewStyle().Foreground(primaryText)
		if i == m.heuristicCursor {
			style = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
		}
		row := pad + style.Render(fmt.Sprintf("%s%s %s", cursor, mark, opt.label))
		lines = append(lines, zone.Mark("add-heur-opt-"+opt.id, row))
	}

	if m.heuristicCursor == 3 {
		// Marker literal input
		lines = append(lines, "")
		input := m.markerLiteral
		if input == "" {
			input = mutedStyle.Render("(enter literal marker string)")
		}
		row := pad + lipgloss.NewStyle().Foreground(primaryText).Render("  Marker: "+input)
		lines = append(lines, zone.Mark("add-heur-marker-input", row))
	}

	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[↑/↓] navigate  [enter] next  [esc] back"))
	return strings.Join(lines, "\n")
}

// viewMonolithicReview renders the Review step when addSourceMonolithic
// is active (Task 4.5). Groups by source file; within each group, one row
// per review candidate.
func (m *addWizardModel) viewMonolithicReview() string {
	pad := "  "
	var lines []string
	lines = append(lines, m.renderTitleRow("Review split candidates", true, ""))
	lines = append(lines, "")

	if len(m.reviewCandidates) == 0 {
		lines = append(lines, pad+mutedStyle.Render("No candidates to review."))
		return strings.Join(lines, "\n")
	}

	// Group candidates by SourceIdx while preserving order.
	type group struct {
		src     monolithicCandidate
		indices []int
	}
	var groups []group
	var currentIdx = -1
	for i, rc := range m.reviewCandidates {
		if rc.SourceIdx != currentIdx {
			if rc.SourceIdx >= 0 && rc.SourceIdx < len(m.discoveryCandidates) {
				groups = append(groups, group{src: m.discoveryCandidates[rc.SourceIdx], indices: []int{i}})
			}
			currentIdx = rc.SourceIdx
		} else if len(groups) > 0 {
			groups[len(groups)-1].indices = append(groups[len(groups)-1].indices, i)
		}
	}

	for _, g := range groups {
		// Group header
		skipSplit := false
		skipReason := ""
		if len(g.indices) == 1 {
			if rc := m.reviewCandidates[g.indices[0]]; rc.SkipSplit {
				skipSplit = true
				skipReason = rc.SkipReason
			}
		}
		var header string
		if skipSplit {
			header = fmt.Sprintf("── %s ─── will import as single rule (%s) ──", g.src.RelPath, skipReason)
		} else {
			header = fmt.Sprintf("── %s ── %d candidates ──", g.src.RelPath, len(g.indices))
		}
		lines = append(lines, pad+mutedStyle.Render(header))

		// Rows
		for _, i := range g.indices {
			rc := m.reviewCandidates[i]
			cursor := "  "
			if i == m.reviewCandidateCursor {
				cursor = "> "
			}
			mark := "◯"
			accepted := i < len(m.reviewAccepted) && m.reviewAccepted[i]
			if accepted {
				mark = "◉"
			}
			slug := rc.Candidate.Name
			if i < len(m.reviewRenames) && m.reviewRenames[i] != "" {
				slug = m.reviewRenames[i]
			}
			desc := rc.Candidate.Description
			if desc == "" {
				desc = "(whole file)"
			}
			text := fmt.Sprintf("%s%s %s  —  %s", cursor, mark, slug, desc)
			style := lipgloss.NewStyle().Foreground(primaryText)
			if i == m.reviewCandidateCursor {
				style = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
			}
			row := pad + style.Render(text)
			lines = append(lines, zone.Mark(fmt.Sprintf("add-mono-review-cand-%d", i), row))
		}
		lines = append(lines, "")
	}

	lines = append(lines, pad+mutedStyle.Render("[↑/↓] navigate  [space] toggle  [r] rename  [enter] next  [esc] back"))
	return strings.Join(lines, "\n")
}

// viewMonolithicExecute renders the Execute step result for the monolithic
// path. Success results are tagged green, failures red.
func (m *addWizardModel) viewMonolithicExecute() string {
	pad := "  "
	var lines []string
	lines = append(lines, m.renderTitleRow("Importing rules", false, "Close"))
	lines = append(lines, "")

	if !m.executeDone {
		lines = append(lines, pad+mutedStyle.Render("Writing accepted candidates to the rule library..."))
		return strings.Join(lines, "\n")
	}

	for _, r := range m.executeMonolithicResult {
		style := lipgloss.NewStyle().Foreground(successColor)
		tag := "✓"
		if r.status == "error" {
			style = lipgloss.NewStyle().Foreground(dangerColor)
			tag = "✗"
		}
		msg := r.name
		if r.err != nil {
			msg = msg + "  " + r.err.Error()
		}
		lines = append(lines, pad+style.Render(tag+" "+msg))
	}
	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[enter] close"))
	return strings.Join(lines, "\n")
}
