package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// addMonolithicDiscoveryDoneMsg delivers the result of scanning for
// monolithic rule files. Wired through m.seq for stale-message detection
// consistent with the legacy discovery path.
type addMonolithicDiscoveryDoneMsg struct {
	seq        int
	candidates []monolithicCandidate
	err        error
}

// addMonolithicExecDoneMsg delivers the result of writing all accepted
// review candidates to the rule library (Task 4.7).
type addMonolithicExecDoneMsg struct {
	seq     int
	results []addExecResult
}

// addCompletedMsg is emitted after a monolithic-rule add wizard finishes
// writing accepted candidates so the App can rescan the catalog and refresh
// content (D18, Task 4.10).
type addCompletedMsg struct {
	count int
}

// startMonolithicDiscoveryCmd scans projectRoot + homeDir for monolithic
// rule files and returns the list as addMonolithicDiscoveryDoneMsg.
func (m *addWizardModel) startMonolithicDiscoveryCmd() tea.Cmd {
	seq := m.seq
	projectRoot := m.projectRoot
	contentRoot := m.contentRoot
	return func() tea.Msg {
		cands, err := discoverMonolithicCandidates(projectRoot, contentRoot)
		return addMonolithicDiscoveryDoneMsg{seq: seq, candidates: cands, err: err}
	}
}

// updateKeyMonolithicDiscovery handles keyboard input on the monolithic-rule
// discovery step (D2, D18). Up/Down moves the cursor, space toggles the
// focused row into selectedCandidates, Enter advances to the Heuristic step
// (blocked until at least one row is selected), Esc backs out to Source.
func (m *addWizardModel) updateKeyMonolithicDiscovery(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	if m.discovering {
		if msg.Type == tea.KeyEsc {
			m.seq++
			m.discovering = false
			m.step = addStepSource
			m.shell.SetActive(m.shellIndexForStep(addStepSource))
		}
		return m, nil
	}
	if m.discoveryErr != "" {
		if msg.Type == tea.KeyEsc {
			m.discoveryErr = ""
			m.step = addStepSource
			m.shell.SetActive(m.shellIndexForStep(addStepSource))
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		// Back to Source
		m.source = addSourceNone
		m.discoveryCandidates = nil
		m.selectedCandidates = nil
		m.discoveryCandidateCurs = 0
		m.step = addStepSource
		m.shell.SetSteps(m.buildShellLabels())
		m.shell.SetActive(0)
		return m, nil

	case tea.KeyUp:
		if m.discoveryCandidateCurs > 0 {
			m.discoveryCandidateCurs--
		}
		return m, nil

	case tea.KeyDown:
		if m.discoveryCandidateCurs < len(m.discoveryCandidates)-1 {
			m.discoveryCandidateCurs++
		}
		return m, nil

	case tea.KeySpace:
		m.toggleMonolithicSelection(m.discoveryCandidateCurs)
		return m, nil

	case tea.KeyEnter:
		if len(m.selectedCandidates) == 0 {
			return m, nil
		}
		m.step = addStepHeuristic
		m.heuristicCursor = 0
		m.chosenHeuristic = int(splitter.HeuristicH2)
		m.shell.SetActive(m.shellIndexForStep(addStepHeuristic))
		m.updateMaxStep()
		return m, nil

	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}
		switch msg.Runes[0] {
		case 'a':
			m.selectedCandidates = m.selectedCandidates[:0]
			for i := range m.discoveryCandidates {
				m.selectedCandidates = append(m.selectedCandidates, i)
			}
		case 'n':
			m.selectedCandidates = nil
		}
		return m, nil
	}
	return m, nil
}

// toggleMonolithicSelection adds or removes idx from selectedCandidates.
// Membership is preserved in insertion order so the Review step groups
// candidates by their selection order.
func (m *addWizardModel) toggleMonolithicSelection(idx int) {
	if idx < 0 || idx >= len(m.discoveryCandidates) {
		return
	}
	for i, sel := range m.selectedCandidates {
		if sel == idx {
			// Remove
			m.selectedCandidates = append(m.selectedCandidates[:i], m.selectedCandidates[i+1:]...)
			return
		}
	}
	m.selectedCandidates = append(m.selectedCandidates, idx)
}

// updateMouseMonolithicDiscovery handles clicks on the discovery rows and
// toggles the clicked row into selectedCandidates. Keeps mouse parity with
// the keyboard spacebar toggle per .claude/rules/tui-wizard-patterns.md.
func (m *addWizardModel) updateMouseMonolithicDiscovery(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	for i := range m.discoveryCandidates {
		if zone.Get(fmt.Sprintf("add-mono-disc-row-%d", i)).InBounds(msg) {
			m.discoveryCandidateCurs = i
			m.toggleMonolithicSelection(i)
			return m, nil
		}
	}
	return m, nil
}

// handleMonolithicDiscoveryDone stores the candidate list into the wizard
// model and clears the scanning spinner.
func (m *addWizardModel) handleMonolithicDiscoveryDone(msg addMonolithicDiscoveryDoneMsg) (*addWizardModel, tea.Cmd) {
	if msg.seq != m.seq {
		return m, nil
	}
	m.discovering = false
	if msg.err != nil {
		m.discoveryErr = msg.err.Error()
		return m, nil
	}
	m.discoveryCandidates = msg.candidates
	m.discoveryCandidateCurs = 0
	return m, nil
}

// handleMonolithicExecDone stores the per-candidate execute results and
// produces an addCompletedMsg so the App can refresh the catalog (Task 4.7).
func (m *addWizardModel) handleMonolithicExecDone(msg addMonolithicExecDoneMsg) (*addWizardModel, tea.Cmd) {
	if msg.seq != m.seq {
		return m, nil
	}
	m.executeMonolithicResult = msg.results
	m.executeDone = true
	m.executing = false
	count := 0
	for _, r := range msg.results {
		if r.status == "added" {
			count++
		}
	}
	return m, func() tea.Msg { return addCompletedMsg{count: count} }
}

// updateKeyHeuristic handles keyboard input on the Heuristic step. Up/Down
// moves the cursor, Enter advances to Review (building candidates from the
// chosen heuristic), Esc returns to Discovery. When the Marker option is
// highlighted, printable runes edit the marker literal in place.
func (m *addWizardModel) updateKeyHeuristic(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	const maxCursor = 4

	// Marker-input mode: while on the marker row, printable runes type into
	// the marker literal. Backspace deletes. Other keys fall through.
	if m.heuristicCursor == 3 {
		switch msg.Type {
		case tea.KeyRunes:
			m.markerLiteral += string(msg.Runes)
			return m, nil
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.markerLiteral) > 0 {
				m.markerLiteral = m.markerLiteral[:len(m.markerLiteral)-1]
			}
			return m, nil
		}
	}

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

	case tea.KeyEnter:
		m.chosenHeuristic = heuristicForCursor(m.heuristicCursor)
		m.reviewCandidates = buildReviewCandidates(m.discoveryCandidates, m.selectedCandidates, m.chosenHeuristic, m.markerLiteral)
		m.reviewAccepted = make([]bool, len(m.reviewCandidates))
		m.reviewRenames = make([]string, len(m.reviewCandidates))
		for i, rc := range m.reviewCandidates {
			m.reviewAccepted[i] = rc.Accept
		}
		m.reviewCandidateCursor = 0
		m.step = addStepReview
		m.shell.SetActive(m.shellIndexForStep(addStepReview))
		m.updateMaxStep()
		return m, nil
	}
	return m, nil
}

// heuristicForCursor maps the Heuristic step's cursor position to the
// splitter heuristic int value.
func heuristicForCursor(cursor int) int {
	switch cursor {
	case 0:
		return int(splitter.HeuristicH2)
	case 1:
		return int(splitter.HeuristicH3)
	case 2:
		return int(splitter.HeuristicH4)
	case 3:
		return int(splitter.HeuristicMarker)
	case 4:
		return int(splitter.HeuristicSingle)
	}
	return int(splitter.HeuristicH2)
}

// updateMouseHeuristic routes clicks on heuristic radio options + the
// marker-input row. Clicking an option moves the cursor and selects it.
// Clicking the marker row sets the cursor to the marker option.
func (m *addWizardModel) updateMouseHeuristic(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	optIDs := []string{"add-heur-opt-h2", "add-heur-opt-h3", "add-heur-opt-h4", "add-heur-opt-marker", "add-heur-opt-single"}
	for i, id := range optIDs {
		if zone.Get(id).InBounds(msg) {
			m.heuristicCursor = i
			return m, nil
		}
	}
	if zone.Get("add-heur-marker-input").InBounds(msg) {
		m.heuristicCursor = 3
		return m, nil
	}
	return m, nil
}

// updateKeyMonolithicReview handles the Review step in the monolithic path.
// Up/Down moves the cursor, space toggles accept, r opens the rename modal,
// Enter advances to Execute (blocked if no accepted candidates), Esc returns
// to Heuristic.
func (m *addWizardModel) updateKeyMonolithicReview(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	if m.renameModal.active {
		// Rename modal handled in the top-level Update before reaching here.
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.step = addStepHeuristic
		m.shell.SetActive(m.shellIndexForStep(addStepHeuristic))
		return m, nil

	case tea.KeyUp:
		if m.reviewCandidateCursor > 0 {
			m.reviewCandidateCursor--
		}
		return m, nil

	case tea.KeyDown:
		if m.reviewCandidateCursor < len(m.reviewCandidates)-1 {
			m.reviewCandidateCursor++
		}
		return m, nil

	case tea.KeySpace:
		if m.reviewCandidateCursor < len(m.reviewAccepted) {
			m.reviewAccepted[m.reviewCandidateCursor] = !m.reviewAccepted[m.reviewCandidateCursor]
		}
		return m, nil

	case tea.KeyEnter:
		any := false
		for _, acc := range m.reviewAccepted {
			if acc {
				any = true
				break
			}
		}
		if !any {
			return m, nil
		}
		m.step = addStepExecute
		m.shell.SetActive(m.shellIndexForStep(addStepExecute))
		m.updateMaxStep()
		m.executing = true
		m.executeDone = false
		return m, m.startMonolithicExecuteCmd()

	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == 'r' {
			return m.openMonolithicRenameModal()
		}
	}
	return m, nil
}

// updateMouseMonolithicReview routes clicks on review candidate rows.
// Clicking a row moves the cursor and toggles the accept state.
func (m *addWizardModel) updateMouseMonolithicReview(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	for i := range m.reviewCandidates {
		if zone.Get(fmt.Sprintf("add-mono-review-cand-%d", i)).InBounds(msg) {
			if m.reviewCandidateCursor == i {
				if i < len(m.reviewAccepted) {
					m.reviewAccepted[i] = !m.reviewAccepted[i]
				}
			} else {
				m.reviewCandidateCursor = i
			}
			return m, nil
		}
	}
	return m, nil
}

// openMonolithicRenameModal pops a one-field edit modal pre-populated with
// the focused candidate's current slug. On save, the new value overrides
// the candidate's Name via reviewRenames.
func (m *addWizardModel) openMonolithicRenameModal() (*addWizardModel, tea.Cmd) {
	if m.reviewCandidateCursor < 0 || m.reviewCandidateCursor >= len(m.reviewCandidates) {
		return m, nil
	}
	cur := m.reviewCandidates[m.reviewCandidateCursor].Candidate.Name
	if m.reviewCandidateCursor < len(m.reviewRenames) && m.reviewRenames[m.reviewCandidateCursor] != "" {
		cur = m.reviewRenames[m.reviewCandidateCursor]
	}
	m.renameModal = newEditModal()
	m.renameModal.SetWidth(m.width)
	m.renameModal.OpenWithContext("Rename rule", cur, "", "", "wizard_rename")
	m.renameDiscoveryIdx = m.reviewCandidateCursor
	return m, nil
}

// updateKeyMonolithicExecute handles the Execute step keyboard input. Enter
// closes the wizard once execution is done; before completion, only Esc is
// accepted to cancel.
func (m *addWizardModel) updateKeyMonolithicExecute(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	if m.executeDone {
		if msg.Type == tea.KeyEnter {
			return m, func() tea.Msg { return addCloseMsg{} }
		}
	}
	return m, nil
}

// updateMouseMonolithicExecute handles clicks on the Execute step nav row.
func (m *addWizardModel) updateMouseMonolithicExecute(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	if m.executeDone {
		if zone.Get("add-nav-next").InBounds(msg) {
			return m, func() tea.Msg { return addCloseMsg{} }
		}
	}
	return m, nil
}

// startMonolithicExecuteCmd writes accepted review candidates via rulestore
// and returns an addMonolithicExecDoneMsg with the per-candidate results.
func (m *addWizardModel) startMonolithicExecuteCmd() tea.Cmd {
	seq := m.seq
	results := m.writeAcceptedCandidates()
	return func() tea.Msg {
		return addMonolithicExecDoneMsg{seq: seq, results: results}
	}
}
