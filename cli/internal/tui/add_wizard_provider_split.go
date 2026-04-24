package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

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
			contentRoot,
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
// splittable selected rule with a toggleable [Split] / [Whole] choice.
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

	for rowIdx, selIdx := range splitIdx {
		it := selected[selIdx]
		mark := "◯"
		action := "import as single rule"
		if it.splitChosen {
			mark = "◉"
			action = fmt.Sprintf("split into %d sections (H2)", it.splitSectionCount)
		}
		cursor := "  "
		if rowIdx == m.heuristicCursor {
			cursor = "> "
		}
		name := it.displayName
		if name == "" {
			name = it.name
		}
		rowText := fmt.Sprintf("%s%s %s  —  %s", cursor, mark, name, action)
		style := lipgloss.NewStyle().Foreground(primaryText)
		if rowIdx == m.heuristicCursor {
			style = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
		}
		row := pad + style.Render(rowText)
		lines = append(lines, zone.Mark(fmt.Sprintf("add-psplit-row-%d", rowIdx), row))
	}

	lines = append(lines, "")
	lines = append(lines, pad+mutedStyle.Render("[↑/↓] navigate  [space] toggle  [enter] next  [esc] back"))
	return strings.Join(lines, "\n")
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

	case tea.KeySpace:
		m.toggleProviderSplitChoice(m.heuristicCursor)
		return m, nil

	case tea.KeyEnter:
		m.enterReview()
		return m, nil
	}
	return m, nil
}

// updateMouseProviderHeuristic routes clicks on per-rule rows. Clicking a row
// moves the cursor AND toggles its split choice (single-click semantics match
// checkbox lists elsewhere in the TUI).
func (m *addWizardModel) updateMouseProviderHeuristic(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	splitIdx := m.splittableSelectionIndices()
	for rowIdx := range splitIdx {
		if zone.Get(fmt.Sprintf("add-psplit-row-%d", rowIdx)).InBounds(msg) {
			if m.heuristicCursor == rowIdx {
				m.toggleProviderSplitChoice(rowIdx)
			} else {
				m.heuristicCursor = rowIdx
			}
			return m, nil
		}
	}
	return m, nil
}

// toggleProviderSplitChoice flips splitChosen for the splittable-selection
// slot at rowIdx. The flip has to land on the underlying discoveredItems entry,
// since selectedItems() returns copies.
func (m *addWizardModel) toggleProviderSplitChoice(rowIdx int) {
	splitIdx := m.splittableSelectionIndices()
	if rowIdx < 0 || rowIdx >= len(splitIdx) {
		return
	}
	// Map selectedItems index → discoveredItems index via the selection order.
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
	// visibleDiscoveryItems returns a filtered view; find the matching entry
	// in the backing slice by identity (name + path) to avoid mutating a copy.
	target := visible[visibleIdx]
	for i := range m.discoveredItems {
		d := &m.discoveredItems[i]
		if d.name == target.name && d.path == target.path && d.itemType == target.itemType {
			d.splitChosen = !d.splitChosen
			return
		}
	}
}
