package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// addSplitRuleItem writes a splittable rule by running the splitter and
// persisting each section through rulestore. Called from addSingleItem when
// item.splittable && item.splitChosen are both true.
//
// The returned addExecResult summarizes the whole operation: status "added"
// with name "<base> (N sections)" on success; "error" with the first error
// otherwise. Sections that fail to write are counted in the error text; no
// rollback is performed (partial installs surface naturally in the library).
func addSplitRuleItem(item addDiscoveryItem, contentRoot, provSlug string) addExecResult {
	if item.path == "" {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("split rule has empty path")}
	}
	raw, err := os.ReadFile(item.path)
	if err != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("read %s: %w", item.path, err)}
	}
	canonBody := canonical.Normalize(raw)
	candidates, skip := splitter.Split(canonBody, splitter.Options{Heuristic: splitter.HeuristicH2})
	if skip != nil {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("splitter skipped: %s", skip.Reason)}
	}
	if len(candidates) == 0 {
		return addExecResult{name: item.name, status: "error", err: fmt.Errorf("splitter produced no candidates")}
	}

	filename := filepath.Base(item.path)
	hash := rulestore.HashBody(canonBody)
	sourceProvider := provSlug
	if sourceProvider == "" {
		sourceProvider = "local"
	}
	format := filenameToFormat(filename)

	written := 0
	var firstErr error
	for _, c := range candidates {
		slug := c.Name
		if slug == "" {
			slug = fallbackSlugFromFilename(filename)
		}
		meta := metadata.RuleMetadata{
			FormatVersion: metadata.CurrentFormatVersion,
			Name:          slug,
			Description:   c.Description,
			Type:          "rule",
			Source: metadata.RuleSource{
				Provider:         sourceProvider,
				Scope:            "project",
				Path:             item.path,
				Format:           format,
				Filename:         filename,
				Hash:             hash,
				SplitMethod:      "h2",
				SplitFromSection: c.Description,
			},
		}
		if werr := rulestore.WriteRuleWithSource(
			filepath.Join(contentRoot, string(catalog.Rules)),
			sourceProvider,
			slug,
			meta,
			[]byte(c.Body),
			filename,
			canonBody,
		); werr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("writing %s: %w", slug, werr)
			}
			continue
		}
		written++
	}

	if firstErr != nil {
		return addExecResult{
			name:   fmt.Sprintf("%s (%d/%d sections)", filename, written, len(candidates)),
			status: "error",
			err:    firstErr,
		}
	}
	return addExecResult{
		name:   fmt.Sprintf("%s (%d sections)", filename, written),
		status: "added",
	}
}

// splittableSelectionIndices returns the indices (into selectedItems()) of
// items flagged splittable=true, preserving selection order.
func (m *addWizardModel) splittableSelectionIndices() []int {
	var out []int
	for i, it := range m.selectedItems() {
		if it.splittable {
			out = append(out, i)
		}
	}
	return out
}

// advanceAfterTriage is the dispatcher called when the user confirms the
// Discovery triage step. It merges confirm choices, then routes to the
// Heuristic step (Provider flow, when any splittable rule is selected) or
// directly to Review. Rebuilds shell labels since the permutation may change.
func (m *addWizardModel) advanceAfterTriage() {
	m.mergeConfirmIntoDiscovery()
	if len(m.selectedItems()) == 0 {
		return
	}
	// Shell-label permutation depends on hasSplittableSelection(), which is
	// only meaningful after merge. Rebuild now so the breadcrumb reflects
	// the correct path before we SetActive().
	m.shell.SetSteps(m.buildShellLabels())
	if m.source != addSourceMonolithic && m.hasSplittableSelection() {
		m.enterProviderHeuristic()
		return
	}
	m.enterReview()
}

// enterProviderHeuristic transitions to the Heuristic step for the Provider
// flow (per-rule split/whole toggle). Cursor starts on the first splittable
// item.
func (m *addWizardModel) enterProviderHeuristic() {
	m.step = addStepHeuristic
	m.shell.SetActive(m.shellIndexForStep(addStepHeuristic))
	m.heuristicCursor = 0
	m.updateMaxStep()
}

// viewProviderHeuristic renders the Provider-flow Heuristic step: one row per
// splittable selected rule with both Split and Whole choices visible
// side-by-side. The active choice is marked ◉; the inactive one ◯. Clicking
// either pill sets that choice explicitly; space/left/right toggle via keyboard.
func (m *addWizardModel) viewProviderHeuristic() string {
	pad := "  "
	var lines []string
	lines = append(lines, m.renderTitleRow("Split monolithic rule files?", true, ""))
	lines = append(lines, "")

	selected := m.selectedItems()
	splitIdx := m.splittableSelectionIndices()

	if len(splitIdx) == 0 {
		// Defensive — shouldn't reach here without splittable items.
		lines = append(lines, pad+mutedStyle.Render("No splittable rules selected."))
		lines = append(lines, "")
		lines = append(lines, pad+mutedStyle.Render("[enter] next  [esc] back"))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, pad+mutedStyle.Render("Detected monolithic rule files. Choose per rule:"))
	lines = append(lines, "")

	// Align the name column so the pill pair lines up across rows.
	nameColW := 0
	for _, selIdx := range splitIdx {
		it := selected[selIdx]
		n := it.displayName
		if n == "" {
			n = it.name
		}
		if w := lipgloss.Width(n); w > nameColW {
			nameColW = w
		}
	}

	for rowIdx, selIdx := range splitIdx {
		it := selected[selIdx]
		cursor := "  "
		if rowIdx == m.heuristicCursor {
			cursor = "> "
		}
		name := it.displayName
		if name == "" {
			name = it.name
		}
		nameStyle := lipgloss.NewStyle().Foreground(primaryText)
		if rowIdx == m.heuristicCursor {
			nameStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
		}
		namePart := cursor + nameStyle.Render(padRight(name, nameColW))

		splitLabel := fmt.Sprintf("Split into %d sections (H2)", it.splitSectionCount)
		wholeLabel := "Import as single rule"
		splitPill := renderSplitPill(splitLabel, it.splitChosen)
		wholePill := renderSplitPill(wholeLabel, !it.splitChosen)

		splitZoned := zone.Mark(fmt.Sprintf("add-psplit-row-%d-split", rowIdx), splitPill)
		wholeZoned := zone.Mark(fmt.Sprintf("add-psplit-row-%d-whole", rowIdx), wholePill)

		rowContent := namePart + "  " + splitZoned + " " + wholeZoned
		// Row-level zone stays so full-row clicks still register (they fall
		// through to updateMouseProviderHeuristic as a cursor-move).
		row := pad + rowContent
		lines = append(lines, zone.Mark(fmt.Sprintf("add-psplit-row-%d", rowIdx), row))
	}

	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[↑/↓] navigate  [←/→] set choice  [space] toggle  [enter] next  [esc] back"))
	return strings.Join(lines, "\n")
}

// renderSplitPill renders a single bracketed choice pill for the Provider-flow
// Heuristic step. Active pills are bold accentColor with ◉; inactive pills are
// muted with ◯. Callers wrap with zone.Mark so the pill is individually clickable.
func renderSplitPill(label string, active bool) string {
	mark := "◯"
	style := mutedStyle
	if active {
		mark = "◉"
		style = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	}
	return style.Render(fmt.Sprintf("[%s %s]", mark, label))
}

// updateKeyProviderHeuristic handles keyboard input on the Provider-flow
// Heuristic step.
func (m *addWizardModel) updateKeyProviderHeuristic(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	splitIdx := m.splittableSelectionIndices()
	maxCursor := len(splitIdx) - 1

	switch msg.Type {
	case tea.KeyEsc:
		m.step = addStepDiscovery
		m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
		return m, nil

	case tea.KeyUp:
		if m.heuristicCursor > 0 {
			m.heuristicCursor--
		}
		return m, nil

	case tea.KeyDown:
		if m.heuristicCursor < maxCursor {
			m.heuristicCursor++
		}
		return m, nil

	case tea.KeyLeft:
		// Left arrow pins the choice to Split (matches pill-order left-to-right).
		m.setProviderSplitChoice(m.heuristicCursor, true)
		return m, nil

	case tea.KeyRight:
		// Right arrow pins the choice to Whole.
		m.setProviderSplitChoice(m.heuristicCursor, false)
		return m, nil

	case tea.KeySpace:
		m.toggleProviderSplitChoice(m.heuristicCursor)
		return m, nil

	case tea.KeyEnter:
		m.enterReview()
		return m, nil
	}
	return m, nil
}

// updateMouseProviderHeuristic routes clicks on per-rule rows. The per-pill
// zones (split/whole) set the choice explicitly; the surrounding row zone moves
// the cursor without toggling (the user is aiming at a specific choice, so a
// full-row click shouldn't ambiguously toggle).
func (m *addWizardModel) updateMouseProviderHeuristic(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	splitIdx := m.splittableSelectionIndices()
	for rowIdx := range splitIdx {
		if zone.Get(fmt.Sprintf("add-psplit-row-%d-split", rowIdx)).InBounds(msg) {
			m.heuristicCursor = rowIdx
			m.setProviderSplitChoice(rowIdx, true)
			return m, nil
		}
		if zone.Get(fmt.Sprintf("add-psplit-row-%d-whole", rowIdx)).InBounds(msg) {
			m.heuristicCursor = rowIdx
			m.setProviderSplitChoice(rowIdx, false)
			return m, nil
		}
		if zone.Get(fmt.Sprintf("add-psplit-row-%d", rowIdx)).InBounds(msg) {
			m.heuristicCursor = rowIdx
			return m, nil
		}
	}
	return m, nil
}

// setProviderSplitChoice sets splitChosen to the explicit value for the
// splittable-selection slot at rowIdx. The write has to land on the underlying
// discoveredItems entry since selectedItems() returns copies.
func (m *addWizardModel) setProviderSplitChoice(rowIdx int, split bool) {
	splitIdx := m.splittableSelectionIndices()
	if rowIdx < 0 || rowIdx >= len(splitIdx) {
		return
	}
	selIdx := splitIdx[rowIdx]
	selectedIdxs := m.discoveryList.SelectedIndices()
	visible := m.visibleDiscoveryItems()
	if selIdx >= len(selectedIdxs) {
		return
	}
	visibleIdx := selectedIdxs[selIdx]
	if visibleIdx >= len(visible) {
		return
	}
	target := visible[visibleIdx]
	for i := range m.discoveredItems {
		d := &m.discoveredItems[i]
		if d.name == target.name && d.path == target.path && d.itemType == target.itemType {
			d.splitChosen = split
			return
		}
	}
}

// toggleProviderSplitChoice flips splitChosen for the splittable-selection
// slot at rowIdx.
func (m *addWizardModel) toggleProviderSplitChoice(rowIdx int) {
	splitIdx := m.splittableSelectionIndices()
	if rowIdx < 0 || rowIdx >= len(splitIdx) {
		return
	}
	selIdx := splitIdx[rowIdx]
	selectedIdxs := m.discoveryList.SelectedIndices()
	visible := m.visibleDiscoveryItems()
	if selIdx >= len(selectedIdxs) {
		return
	}
	visibleIdx := selectedIdxs[selIdx]
	if visibleIdx >= len(visible) {
		return
	}
	target := visible[visibleIdx]
	for i := range m.discoveredItems {
		d := &m.discoveredItems[i]
		if d.name == target.name && d.path == target.path && d.itemType == target.itemType {
			d.splitChosen = !d.splitChosen
			return
		}
	}
}
