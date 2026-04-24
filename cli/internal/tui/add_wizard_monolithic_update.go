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
