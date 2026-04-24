package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
)

// Update handles messages for the add wizard.
func (m *addWizardModel) Update(msg tea.Msg) (*addWizardModel, tea.Cmd) {
	m.validateStep()

	// Rename modal (review step) captures all key + mouse input when active.
	if m.renameModal.active {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			m.renameModal, cmd = m.renameModal.Update(msg)
			return m, cmd
		case tea.MouseMsg:
			var cmd tea.Cmd
			m.renameModal, cmd = m.renameModal.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case addDiscoveryDoneMsg:
		return m.handleDiscoveryDone(msg)
	case addMonolithicDiscoveryDoneMsg:
		return m.handleMonolithicDiscoveryDone(msg)
	case addMonolithicExecDoneMsg:
		return m.handleMonolithicExecDone(msg)
	case addExecItemDoneMsg:
		return m.handleExecItemDone(msg)
	case editCancelledMsg:
		// Modal already closed itself; no state to reset.
		return m, nil
	}
	return m, nil
}

func (m *addWizardModel) updateMouse(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	// Mouse wheel scrolling
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return m.updateMouseWheel(msg)
	}

	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Wizard shell breadcrumb clicks — disabled during Execute step (can't undo adds)
	if m.step != addStepExecute {
		if step, ok := m.shell.HandleClick(msg); ok {
			target := m.stepForShellIndex(step)
			// Don't allow navigating to Execute via breadcrumb
			if target == addStepExecute {
				return m, nil
			}
			if target != m.step && target <= m.maxStep {
				m.step = target
				m.shell.SetActive(step)
				m.reviewAcknowledged = false
			}
			return m, nil
		}
	}

	// Nav button clicks (shared across steps)
	if zone.Get("add-nav-back").InBounds(msg) {
		return m.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	}
	if zone.Get("add-nav-next").InBounds(msg) {
		return m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	}

	// Per-step mouse routing
	switch m.step {
	case addStepSource:
		return m.updateMouseSource(msg)
	case addStepType:
		return m.updateMouseType(msg)
	case addStepDiscovery:
		return m.updateMouseDiscovery(msg)
	case addStepHeuristic:
		return m.updateMouseHeuristic(msg)
	case addStepReview:
		if m.source == addSourceMonolithic {
			return m.updateMouseMonolithicReview(msg)
		}
		return m.updateMouseReview(msg)
	case addStepExecute:
		if m.source == addSourceMonolithic {
			return m.updateMouseMonolithicExecute(msg)
		}
		return m.updateMouseExecute(msg)
	}

	return m, nil
}

// updateMouseWheel handles scroll wheel events on the current step.
func (m *addWizardModel) updateMouseWheel(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	up := msg.Button == tea.MouseButtonWheelUp

	switch m.step {
	case addStepDiscovery:
		if m.discovering || m.discoveryErr != "" || len(m.confirmItems) == 0 {
			return m, nil
		}
		if up {
			if m.confirmCursor > 0 {
				m.confirmCursor--
				m.adjustTriageOffset()
				m.loadTriagePreview()
			}
		} else {
			if m.confirmCursor < len(m.confirmItems)-1 {
				m.confirmCursor++
				m.adjustTriageOffset()
				m.loadTriagePreview()
			}
		}
	case addStepReview:
		if m.reviewDrillIn {
			if m.reviewDrillTree.focused {
				if up {
					m.reviewDrillTree.CursorUp()
				} else {
					m.reviewDrillTree.CursorDown()
				}
				m.loadDrillInFile()
			} else {
				if up {
					m.reviewDrillPreview.ScrollUp()
				} else {
					m.reviewDrillPreview.ScrollDown()
				}
			}
			return m, nil
		}
		if m.reviewZone == addReviewZoneItems {
			items := m.selectedItems()
			if up {
				if m.reviewItemCursor > 0 {
					m.reviewItemCursor--
				}
			} else {
				if m.reviewItemCursor < len(items)-1 {
					m.reviewItemCursor++
				}
			}
			m.adjustReviewOffset()
			m.highlightRisksForItem(m.reviewItemCursor)
		}
	case addStepType:
		if up {
			if m.typeChecks.cursor > 0 {
				m.typeChecks.cursor--
				m.typeChecks.adjustOffset()
			}
		} else {
			if m.typeChecks.cursor < len(m.typeChecks.items)-1 {
				m.typeChecks.cursor++
				m.typeChecks.adjustOffset()
			}
		}
	case addStepExecute:
		if m.executeDone {
			execH := max(3, m.height-8)
			maxOff := max(0, len(m.selectedItems())-execH)
			if up {
				if m.executeOffset > 0 {
					m.executeOffset--
				}
			} else {
				if m.executeOffset < maxOff {
					m.executeOffset++
				}
			}
		}
	}
	return m, nil
}

// --- Source step mouse ---

func (m *addWizardModel) updateMouseSource(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	// Top-level source option clicks
	for i := 0; i < 4; i++ {
		if zone.Get("add-src-" + itoa(i)).InBounds(msg) {
			if m.sourceExpanded || m.inputActive {
				// Already in a sub-mode — clicking a source option resets to top-level
				m.sourceExpanded = false
				m.inputActive = false
			}
			m.sourceCursor = i

			// Activate like Enter
			switch i {
			case 0: // Provider
				if len(m.providers) > 0 {
					m.sourceExpanded = true
					m.providerCursor = 0
				}
			case 1: // Registry
				if len(m.registries) > 0 {
					m.sourceExpanded = true
					m.registryCursor = 0
				}
			case 2: // Local
				m.inputActive = true
				m.sourceErr = ""
			case 3: // Git
				m.inputActive = true
				m.sourceErr = ""
			}
			return m, nil
		}
	}

	// Provider sub-list clicks
	for i := range m.providers {
		if zone.Get("add-prov-" + itoa(i)).InBounds(msg) {
			m.providerCursor = i
			// Double-click-like: select and advance
			m.source = addSourceProvider
			m.sourceExpanded = false
			cmd := m.advanceFromSource()
			return m, cmd
		}
	}

	// Registry sub-list clicks
	for i := range m.registries {
		if zone.Get("add-reg-" + itoa(i)).InBounds(msg) {
			m.registryCursor = i
			m.source = addSourceRegistry
			m.sourceExpanded = false
			cmd := m.advanceFromSource()
			return m, cmd
		}
	}

	// Path input click — focus the input
	if zone.Get("add-path-input").InBounds(msg) {
		if !m.inputActive {
			m.inputActive = true
			m.sourceErr = ""
		}
		return m, nil
	}

	return m, nil
}

// --- Type step mouse ---

func (m *addWizardModel) updateMouseType(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	if idx, ok := m.typeChecks.HandleClick(msg); ok {
		m.typeChecks.cursor = idx
		if !m.typeChecks.items[idx].disabled {
			m.typeChecks.selected[idx] = !m.typeChecks.selected[idx]
		}
		return m, nil
	}
	return m, nil
}

// --- Discovery step mouse ---

func (m *addWizardModel) updateMouseDiscovery(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	// Monolithic-rule discovery has a single-list layout; clicks toggle
	// selection on the clicked row (mouse parity with the spacebar key).
	if m.source == addSourceMonolithic {
		return m.updateMouseMonolithicDiscovery(msg)
	}

	// Error state buttons
	if m.discoveryErr != "" {
		if zone.Get("add-retry").InBounds(msg) {
			m.discoveryErr = ""
			m.seq++
			m.discovering = true
			return m, m.startDiscoveryCmd()
		}
		if zone.Get("add-err-back").InBounds(msg) {
			m.goBackFromDiscovery()
			return m, nil
		}
		return m, nil
	}

	// Empty state back button
	if zone.Get("add-empty-back").InBounds(msg) {
		m.goBackFromDiscovery()
		return m, nil
	}

	// Normal state — triage-style split-pane buttons
	if zone.Get("triage-cancel").InBounds(msg) {
		return m, func() tea.Msg { return addCloseMsg{} }
	}
	if zone.Get("triage-back").InBounds(msg) {
		m.goBackFromDiscovery()
		return m, nil
	}
	if zone.Get("triage-next").InBounds(msg) {
		m.mergeConfirmIntoDiscovery()
		if len(m.selectedItems()) > 0 {
			m.enterReview()
		}
		return m, nil
	}

	// Item row clicks — first click selects/previews, second click toggles
	for i := range m.confirmItems {
		if zone.Get(fmt.Sprintf("triage-item-%d", i)).InBounds(msg) {
			m.confirmFocus = triageZoneItems
			if m.confirmSelected == nil {
				m.confirmSelected = make(map[int]bool)
			}
			if m.confirmCursor == i {
				m.confirmSelected[i] = !m.confirmSelected[i]
			} else {
				m.confirmCursor = i
				m.adjustTriageOffset()
				m.loadTriagePreview()
			}
			return m, nil
		}
	}

	return m, nil
}

// --- Review step mouse ---

func (m *addWizardModel) updateMouseReview(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	// Drill-in: handle clicks within the drill-in view
	if m.reviewDrillIn {
		if zone.Get("add-nav-back").InBounds(msg) {
			m.exitReviewDrillIn()
			return m, nil
		}
		if zone.Get("add-rename").InBounds(msg) {
			m.openRenameModal(m.reviewItemCursor)
			return m, nil
		}
		if zone.Get("add-nav-next").InBounds(msg) {
			// Advance to the next selected item, mirroring the keyboard
			// Enter-on-Next behavior. If this is the last item, exit drill-in.
			selected := m.selectedItems()
			if m.reviewItemCursor+1 < len(selected) {
				m.reviewItemCursor++
				m.enterReviewDrillIn()
			} else {
				m.exitReviewDrillIn()
			}
			return m, nil
		}
		// Click-to-focus: left portion = tree, right portion = preview.
		// Any pane click also drops the title-row button cursor so the
		// highlight follows the user's focus.
		innerW := m.width - borderSize
		treeW := max(18, innerW*30/100)
		if msg.X <= treeW+1 {
			// Click in tree area — focus tree
			m.drillButtonCursor = -1
			if !m.reviewDrillTree.focused {
				m.reviewDrillTree.focused = true
				m.reviewDrillPreview.focused = false
			}
			// Handle tree item clicks (zone IDs are "ftnode-N" from fileTreeModel)
			for i := range m.reviewDrillTree.nodes {
				if zone.Get("ftnode-" + itoa(i)).InBounds(msg) {
					m.reviewDrillTree.cursor = i
					if m.reviewDrillTree.nodes[i].isDir {
						m.reviewDrillTree.ToggleDir()
					}
					m.loadDrillInFile()
					return m, nil
				}
			}
		} else {
			// Click in preview area — focus preview
			m.drillButtonCursor = -1
			if m.reviewDrillTree.focused {
				m.reviewDrillTree.focused = false
				m.reviewDrillPreview.focused = true
			}
		}
		return m, nil
	}

	// Button clicks (already zone-marked by renderModalButtons)
	if zone.Get("add-cancel").InBounds(msg) {
		return m, func() tea.Msg { return addCloseMsg{} }
	}
	if zone.Get("add-back").InBounds(msg) {
		m.step = addStepDiscovery
		m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
		m.reviewAcknowledged = false
		return m, nil
	}
	if zone.Get("add-confirm").InBounds(msg) {
		if !m.reviewAcknowledged {
			m.reviewAcknowledged = true
			m.enterExecute()
			return m, m.addItemCmd(0)
		}
		return m, nil
	}
	if zone.Get("add-rename").InBounds(msg) {
		m.openRenameModal(m.reviewItemCursor)
		return m, nil
	}

	// Review item clicks — click to select, click again to drill in
	selected := m.selectedItems()
	for i := range selected {
		if zone.Get("add-rev-item-" + itoa(i)).InBounds(msg) {
			if m.reviewZone == addReviewZoneItems && m.reviewItemCursor == i {
				// Already selected — drill in
				m.enterReviewDrillIn()
				return m, nil
			}
			m.reviewZone = addReviewZoneItems
			m.reviewItemCursor = i
			m.adjustReviewOffset()
			m.highlightRisksForItem(i)
			return m, nil
		}
	}

	return m, nil
}

// --- Execute step mouse ---

func (m *addWizardModel) updateMouseExecute(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	if m.executeDone {
		if zone.Get("add-exec-done").InBounds(msg) {
			return m, func() tea.Msg { return addCloseMsg{} }
		}
		if zone.Get("add-exec-restart").InBounds(msg) {
			return m, func() tea.Msg { return addRestartMsg{} }
		}
	} else if !m.executeCancelled {
		if zone.Get("add-exec-cancel").InBounds(msg) {
			m.executeCancelled = true
		}
	}
	return m, nil
}

func (m *addWizardModel) updateKey(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch m.step {
	case addStepSource:
		return m.updateKeySource(msg)
	case addStepType:
		return m.updateKeyType(msg)
	case addStepDiscovery:
		return m.updateKeyDiscovery(msg)
	case addStepHeuristic:
		return m.updateKeyHeuristic(msg)
	case addStepReview:
		if m.source == addSourceMonolithic {
			return m.updateKeyMonolithicReview(msg)
		}
		return m.updateKeyReview(msg)
	case addStepExecute:
		if m.source == addSourceMonolithic {
			return m.updateKeyMonolithicExecute(msg)
		}
		return m.updateKeyExecute(msg)
	}
	return m, nil
}

// --- Source step ---

func (m *addWizardModel) updateKeySource(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	// Mode 3: Text input active
	if m.inputActive {
		return m.updateKeySourceInput(msg)
	}

	// Mode 2: Sub-list expanded
	if m.sourceExpanded {
		return m.updateKeySourceExpanded(msg)
	}

	// Mode 1: Top-level radio
	return m.updateKeySourceRadio(msg)
}

// Mode 1: Top-level radio selection
func (m *addWizardModel) updateKeySourceRadio(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m, func() tea.Msg { return addCloseMsg{} }

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.sourceCursor < 3 {
			m.sourceCursor++
		}

	case msg.Type == tea.KeyEnter:
		switch m.sourceCursor {
		case 0: // Provider
			if len(m.providers) > 0 {
				m.sourceExpanded = true
				m.providerCursor = 0
			}
		case 1: // Registry
			if len(m.registries) > 0 {
				m.sourceExpanded = true
				m.registryCursor = 0
			}
		case 2: // Local
			m.inputActive = true
			m.sourceErr = ""
		case 3: // Git
			m.inputActive = true
			m.sourceErr = ""
		}
	}
	return m, nil
}

// Mode 2: Sub-list expanded (provider or registry picker)
func (m *addWizardModel) updateKeySourceExpanded(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		m.sourceExpanded = false
		return m, nil

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.sourceCursor == 0 { // Provider sub-list
			if m.providerCursor > 0 {
				m.providerCursor--
			}
		} else { // Registry sub-list
			if m.registryCursor > 0 {
				m.registryCursor--
			}
		}

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.sourceCursor == 0 {
			if m.providerCursor < len(m.providers)-1 {
				m.providerCursor++
			}
		} else {
			if m.registryCursor < len(m.registries)-1 {
				m.registryCursor++
			}
		}

	case msg.Type == tea.KeyEnter:
		if m.sourceCursor == 0 {
			m.source = addSourceProvider
		} else {
			m.source = addSourceRegistry
		}
		m.sourceExpanded = false
		cmd := m.advanceFromSource()
		return m, cmd
	}
	return m, nil
}

// Mode 3: Text input active (local path or git URL)
func (m *addWizardModel) updateKeySourceInput(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.pathInput != "" {
			m.pathInput = ""
			m.pathCursor = 0
			m.sourceErr = ""
		}
		m.inputActive = false
		return m, nil

	case tea.KeyUp:
		m.inputActive = false
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
		return m, nil

	case tea.KeyDown:
		m.inputActive = false
		if m.sourceCursor < 3 {
			m.sourceCursor++
		}
		return m, nil

	case tea.KeyEnter:
		if m.pathInput == "" {
			return m, nil
		}
		if m.sourceCursor == 2 {
			// Local path validation — expand ~ to home dir
			inputPath := m.pathInput
			if strings.HasPrefix(inputPath, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					inputPath = filepath.Join(home, inputPath[2:]) // skip "~/"
				}
			} else if inputPath == "~" {
				if home, err := os.UserHomeDir(); err == nil {
					inputPath = home
				}
			}
			if !filepath.IsAbs(inputPath) {
				m.sourceErr = "Path must be absolute (use ~ for home directory)"
				return m, nil
			}
			m.pathInput = inputPath // normalize for downstream use
			info, err := os.Stat(inputPath)
			if err != nil {
				m.sourceErr = "Path does not exist"
				return m, nil
			}
			if !info.IsDir() {
				m.sourceErr = "Path must be a directory"
				return m, nil
			}
			m.sourceErr = ""
			m.source = addSourceLocal
			m.inputActive = false
			cmd := m.advanceFromSource()
			return m, cmd
		}
		if m.sourceCursor == 3 {
			// Git URL validation
			if !validGitURL(m.pathInput) {
				lower := strings.ToLower(m.pathInput)
				if strings.HasPrefix(lower, "ext::") || strings.HasPrefix(lower, "fd::") {
					m.sourceErr = "Blocked: ext:: and fd:: protocols are not allowed"
				} else if strings.HasPrefix(lower, "file://") {
					m.sourceErr = "Use Local Path for file:// URLs"
				} else {
					m.sourceErr = "Enter a valid git URL (https://, git@, ssh://)"
				}
				return m, nil
			}
			m.sourceErr = ""
			m.source = addSourceGit
			m.inputActive = false
			cmd := m.advanceFromSource()
			return m, cmd
		}
		return m, nil

	case tea.KeyBackspace:
		if m.pathCursor > 0 {
			runes := []rune(m.pathInput)
			m.pathInput = string(runes[:m.pathCursor-1]) + string(runes[m.pathCursor:])
			m.pathCursor--
			m.sourceErr = ""
		}
		return m, nil

	case tea.KeyLeft:
		if m.pathCursor > 0 {
			m.pathCursor--
		}
		return m, nil

	case tea.KeyRight:
		if m.pathCursor < len([]rune(m.pathInput)) {
			m.pathCursor++
		}
		return m, nil

	case tea.KeyHome, tea.KeyCtrlA:
		m.pathCursor = 0
		return m, nil

	case tea.KeyEnd, tea.KeyCtrlE:
		m.pathCursor = len([]rune(m.pathInput))
		return m, nil

	case tea.KeySpace:
		runes := []rune(m.pathInput)
		newRunes := make([]rune, 0, len(runes)+1)
		newRunes = append(newRunes, runes[:m.pathCursor]...)
		newRunes = append(newRunes, ' ')
		newRunes = append(newRunes, runes[m.pathCursor:]...)
		m.pathInput = string(newRunes)
		m.pathCursor++
		m.sourceErr = ""
		return m, nil

	case tea.KeyRunes:
		runes := []rune(m.pathInput)
		newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
		newRunes = append(newRunes, runes[:m.pathCursor]...)
		newRunes = append(newRunes, msg.Runes...)
		newRunes = append(newRunes, runes[m.pathCursor:]...)
		m.pathInput = string(newRunes)
		m.pathCursor += len(msg.Runes)
		m.sourceErr = ""
		return m, nil
	}
	return m, nil
}

// --- Type step ---

func (m *addWizardModel) updateKeyType(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.step = addStepSource
		m.shell.SetActive(0)
		return m, nil

	case tea.KeyEnter:
		if len(m.selectedTypes()) > 0 {
			m.step = addStepDiscovery
			m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
			m.discovering = true
			m.updateMaxStep()
			return m, m.startDiscoveryCmd()
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.typeChecks, cmd = m.typeChecks.Update(msg)
		return m, cmd
	}
}

// --- Discovery step ---

func (m *addWizardModel) updateKeyDiscovery(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	// Monolithic-rule discovery has its own single-list layout — no split
	// pane, no triage items. Each candidate is a row with a [space] toggle.
	if m.source == addSourceMonolithic {
		return m.updateKeyMonolithicDiscovery(msg)
	}

	// During scan, only Esc works
	if m.discovering {
		if msg.Type == tea.KeyEsc {
			m.seq++
			m.discovering = false
			m.goBackFromDiscovery()
			return m, nil
		}
		return m, nil
	}

	// Error state
	if m.discoveryErr != "" {
		if msg.String() == "r" {
			m.discoveryErr = ""
			m.seq++
			m.discovering = true
			return m, m.startDiscoveryCmd()
		}
		if msg.Type == tea.KeyEsc {
			m.goBackFromDiscovery()
			return m, nil
		}
		return m, nil
	}

	// Normal state — triage-style split-pane navigation
	if len(m.confirmItems) == 0 {
		if msg.Type == tea.KeyEsc {
			m.goBackFromDiscovery()
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.goBackFromDiscovery()
		return m, nil

	case tea.KeyRight:
		m.confirmFocus = triageZonePreview
		return m, nil

	case tea.KeyEnter:
		m.mergeConfirmIntoDiscovery()
		if len(m.selectedItems()) > 0 {
			m.enterReview()
		}
		return m, nil

	case tea.KeyTab:
		m.confirmFocus = (m.confirmFocus + 1) % 3
		return m, nil

	case tea.KeyShiftTab:
		m.confirmFocus = (m.confirmFocus + 2) % 3
		return m, nil

	case tea.KeyUp:
		switch m.confirmFocus {
		case triageZoneItems:
			if m.confirmCursor > 0 {
				m.confirmCursor--
				m.adjustTriageOffset()
				m.loadTriagePreview()
			}
		case triageZonePreview:
			if m.confirmPreview.offset > 0 {
				m.confirmPreview.offset--
			}
		}
		return m, nil

	case tea.KeyDown:
		switch m.confirmFocus {
		case triageZoneItems:
			if m.confirmCursor < len(m.confirmItems)-1 {
				m.confirmCursor++
				m.adjustTriageOffset()
				m.loadTriagePreview()
			}
		case triageZonePreview:
			maxOff := max(0, len(m.confirmPreview.lines)-1)
			if m.confirmPreview.offset < maxOff {
				m.confirmPreview.offset++
			}
		}
		return m, nil

	case tea.KeyLeft:
		if m.confirmFocus != triageZoneItems {
			m.confirmFocus = triageZoneItems
		}
		return m, nil

	case tea.KeyPgUp:
		if m.confirmFocus == triageZonePreview {
			pageSize := 10
			m.confirmPreview.offset = max(0, m.confirmPreview.offset-pageSize)
		}
		return m, nil

	case tea.KeyPgDown:
		if m.confirmFocus == triageZonePreview {
			pageSize := 10
			maxOff := max(0, len(m.confirmPreview.lines)-1)
			m.confirmPreview.offset = min(maxOff, m.confirmPreview.offset+pageSize)
		}
		return m, nil

	case tea.KeySpace:
		if m.confirmFocus == triageZoneItems && m.confirmCursor < len(m.confirmItems) {
			if m.confirmSelected == nil {
				m.confirmSelected = make(map[int]bool)
			}
			m.confirmSelected[m.confirmCursor] = !m.confirmSelected[m.confirmCursor]
		}
		return m, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'a':
				if m.confirmSelected == nil {
					m.confirmSelected = make(map[int]bool)
				}
				for i := range m.confirmItems {
					m.confirmSelected[i] = true
				}
			case 'n':
				m.confirmSelected = make(map[int]bool)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *addWizardModel) handleDiscoveryDone(msg addDiscoveryDoneMsg) (*addWizardModel, tea.Cmd) {
	if msg.seq != m.seq {
		return m, nil // stale
	}
	m.discovering = false
	if msg.err != nil {
		m.discoveryErr = msg.err.Error()
		return m, nil
	}

	// Separate installed items (already in library) from actionable (new/outdated)
	var installed []addDiscoveryItem
	var actionable []addDiscoveryItem
	for _, item := range msg.items {
		if item.status == add.StatusInLibrary {
			installed = append(installed, item)
		} else {
			actionable = append(actionable, item)
		}
	}

	// Installed items serve as the pre-merge baseline in discoveredItems so
	// mergeConfirmIntoDiscovery can preserve them when it rebuilds the slice.
	m.actionableCount = 0
	m.installedCount = len(installed)
	m.preMergeActionableCount = 0
	m.preMergeInstalledCount = len(installed)
	m.discoveredItems = installed
	m.showInstalled = false

	if msg.tmpDir != "" {
		m.gitTempDir = msg.tmpDir
	}
	m.sourceRegistry = msg.sourceRegistry
	m.sourceVisibility = msg.sourceVisibility
	m.discoveryList = m.buildDiscoveryList()

	// Convert auto-discovered actionable items to addConfirmItem (pre-selected).
	autoConfirm := make([]addConfirmItem, len(actionable))
	for i, di := range actionable {
		name := di.displayName
		if name == "" {
			name = di.name
		}
		// Derive sourceDir from the path when the discovery item didn't carry one
		// (file items from nativeItemsToDiscovery leave SourceDir empty).
		sourceDir := di.sourceDir
		if sourceDir == "" && di.path != "" {
			sourceDir = filepath.Dir(di.path)
		}
		// Pattern-matched items (catalog scan, native provider, hooks) don't
		// carry a tier because they were found via known layout, not the
		// content-signal analyzer.  Default to TierHigh — reliable detection.
		tier := di.tier
		if tier == "" && di.detectionSource != "content-signal" {
			tier = analyzer.TierHigh
		}
		autoConfirm[i] = addConfirmItem{
			displayName:   name,
			itemType:      di.itemType,
			path:          filepath.Base(di.path),
			sourceDir:     sourceDir,
			tier:          tier,
			status:        di.status,     // preserve StatusOutdated for conflict detection
			underlying:    di.underlying, // preserve for merge fidelity
			risks:         di.risks,      // pre-computed risks (hooks carry command/URL risks)
			hookData:      di.hookData,   // needed for triage preview + drill-in
			hookSourceDir: di.hookSourceDir,
			catalogItem:   di.catalogItem, // original item with correct Files list
		}
	}

	// Build unified list: auto items first (pre-selected), then needs-review items.
	allConfirm := append(autoConfirm, msg.confirmItems...)
	newSel := make(map[int]bool, len(allConfirm))
	for i := range autoConfirm {
		newSel[i] = true // auto-discovered items are always pre-selected
	}
	for i, ci := range msg.confirmItems {
		idx := len(autoConfirm) + i
		newSel[idx] = ci.tier == analyzer.TierHigh || ci.tier == analyzer.TierUser
	}

	m.confirmItems, m.confirmSelected = sortConfirmItemsByType(allConfirm, newSel)
	m.confirmCursor = 0
	m.confirmOffset = 0
	m.confirmFocus = triageZoneItems
	m.confirmPreview = newPreviewModel()
	m.loadTriagePreview()
	m.updateMaxStep()

	return m, nil
}

// --- Review step ---

func (m *addWizardModel) updateKeyReview(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	// Drill-in sub-view captures all input
	if m.reviewDrillIn {
		return m.updateKeyReviewDrillIn(msg)
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.step = addStepDiscovery
		m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
		m.reviewAcknowledged = false
		return m, nil

	case tea.KeyTab:
		m.reviewTabForward()
		return m, nil

	case tea.KeyShiftTab:
		m.reviewTabBackward()
		return m, nil

	default:
		switch m.reviewZone {
		case addReviewZoneItems:
			return m.updateKeyReviewItems(msg)
		case addReviewZoneButtons:
			return m.updateKeyReviewButtons(msg)
		}
	}
	return m, nil
}

// updateKeyReviewDrillIn handles keys while viewing item detail in review.
func (m *addWizardModel) updateKeyReviewDrillIn(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	// [e] opens the rename modal from inside drill-in so the user can pick a
	// meaningful display name while looking at the hook/MCP/rule's contents.
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'e' {
		m.openRenameModal(m.reviewItemCursor)
		return m, nil
	}

	// When the title-row buttons hold focus, intercept navigation and Enter
	// before it reaches the tree/preview handlers below. Tab still cycles
	// onward so users can walk through the full strip.
	if m.drillButtonCursor >= 0 {
		switch msg.Type {
		case tea.KeyEsc:
			m.exitReviewDrillIn()
			return m, nil
		case tea.KeyTab:
			// Cycle Back → Rename → Next → panes → Back.
			m.drillButtonCursor++
			if m.drillButtonCursor > 2 {
				m.drillButtonCursor = -1
				m.reviewDrillTree.focused = true
				m.reviewDrillPreview.focused = false
			}
			return m, nil
		case tea.KeyShiftTab:
			m.drillButtonCursor--
			if m.drillButtonCursor < 0 {
				// Wrap to preview pane so Shift+Tab mirrors Tab.
				m.drillButtonCursor = -1
				m.reviewDrillTree.focused = false
				m.reviewDrillPreview.focused = true
			}
			return m, nil
		case tea.KeyLeft:
			if m.drillButtonCursor > 0 {
				m.drillButtonCursor--
			}
			return m, nil
		case tea.KeyRight:
			if m.drillButtonCursor < 2 {
				m.drillButtonCursor++
			}
			return m, nil
		case tea.KeyEnter:
			switch m.drillButtonCursor {
			case 0: // Back — exit drill-in back to review list
				m.exitReviewDrillIn()
				return m, nil
			case 1: // Rename
				m.openRenameModal(m.reviewItemCursor)
				return m, nil
			case 2: // Next — advance to the next item or exit if last
				selected := m.selectedItems()
				if m.reviewItemCursor+1 < len(selected) {
					m.reviewItemCursor++
					m.enterReviewDrillIn()
				} else {
					m.exitReviewDrillIn()
				}
				return m, nil
			}
			return m, nil
		}
		// Non-navigation keys fall through and are ignored while buttons focused.
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.exitReviewDrillIn()
		return m, nil
	case tea.KeyUp:
		if m.reviewDrillTree.focused {
			m.reviewDrillTree.CursorUp()
			m.loadDrillInFile()
		} else {
			m.reviewDrillPreview.ScrollUp()
		}
	case tea.KeyDown:
		if m.reviewDrillTree.focused {
			m.reviewDrillTree.CursorDown()
			m.loadDrillInFile()
		} else {
			m.reviewDrillPreview.ScrollDown()
		}
	case tea.KeyPgUp:
		if m.reviewDrillTree.focused {
			m.reviewDrillTree.PageUp()
			m.loadDrillInFile()
		} else {
			m.reviewDrillPreview.PageUp()
		}
	case tea.KeyPgDown:
		if m.reviewDrillTree.focused {
			m.reviewDrillTree.PageDown()
			m.loadDrillInFile()
		} else {
			m.reviewDrillPreview.PageDown()
		}
	case tea.KeyLeft, tea.KeyRight:
		m.reviewDrillTree.focused = !m.reviewDrillTree.focused
		m.reviewDrillPreview.focused = !m.reviewDrillTree.focused
	case tea.KeyTab:
		// Cycle tree → preview → Back button.
		if m.reviewDrillTree.focused {
			m.reviewDrillTree.focused = false
			m.reviewDrillPreview.focused = true
		} else {
			m.reviewDrillTree.focused = false
			m.reviewDrillPreview.focused = false
			m.drillButtonCursor = 0 // enter button row at Back
		}
	case tea.KeyShiftTab:
		// Reverse cycle: preview → tree → (wraps to Next button).
		if m.reviewDrillPreview.focused {
			m.reviewDrillPreview.focused = false
			m.reviewDrillTree.focused = true
		} else {
			m.reviewDrillTree.focused = false
			m.reviewDrillPreview.focused = false
			m.drillButtonCursor = 2 // enter button row at Next
		}
	}
	return m, nil
}

func (m *addWizardModel) updateKeyReviewItems(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	itemCount := len(m.selectedItems())
	if itemCount == 0 {
		return m, nil
	}
	switch {
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.reviewItemCursor > 0 {
			m.reviewItemCursor--
		}
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.reviewItemCursor < itemCount-1 {
			m.reviewItemCursor++
		}
	case msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		// Right crosses into the button row so users don't need to discover
		// the Tab shortcut. Land on the leftmost button (Add) — from there
		// Right/Left walk through Rename, Back, Cancel.
		m.reviewZone = addReviewZoneButtons
		m.buttonCursor = 0
		return m, nil
	case msg.Type == tea.KeyPgUp:
		pageSize := max(1, m.reviewVisibleHeight())
		m.reviewItemCursor = max(0, m.reviewItemCursor-pageSize)
	case msg.Type == tea.KeyPgDown:
		pageSize := max(1, m.reviewVisibleHeight())
		m.reviewItemCursor = min(itemCount-1, m.reviewItemCursor+pageSize)
	case msg.Type == tea.KeyHome:
		m.reviewItemCursor = 0
	case msg.Type == tea.KeyEnd:
		m.reviewItemCursor = itemCount - 1
	case msg.Type == tea.KeyEnter:
		m.enterReviewDrillIn()
		return m, nil
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'e':
		m.openRenameModal(m.reviewItemCursor)
		return m, nil
	}
	m.adjustReviewOffset()
	m.highlightRisksForItem(m.reviewItemCursor)
	return m, nil
}

func (m *addWizardModel) updateKeyReviewButtons(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h'):
		if m.buttonCursor > 0 {
			m.buttonCursor--
			return m, nil
		}
		// Already at leftmost button — cross back into the items zone so
		// Left/Right navigation is symmetric with the Right-from-items jump.
		if len(m.selectedItems()) > 0 {
			m.reviewZone = addReviewZoneItems
		}
		return m, nil
	case msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		if m.buttonCursor < 3 {
			m.buttonCursor++
		}
	case msg.Type == tea.KeyEnter:
		switch m.buttonCursor {
		case 0: // Add
			if !m.reviewAcknowledged {
				m.reviewAcknowledged = true
				m.enterExecute()
				return m, m.addItemCmd(0)
			}
		case 1: // Rename
			m.openRenameModal(m.reviewItemCursor)
			return m, nil
		case 2: // Back
			m.step = addStepDiscovery
			m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
			m.reviewAcknowledged = false
			return m, nil
		case 3: // Cancel
			return m, func() tea.Msg { return addCloseMsg{} }
		}
	}
	return m, nil
}

// reviewTabForward cycles: risks -> items -> buttons -> risks.
func (m *addWizardModel) reviewTabForward() {
	zones := m.reviewZoneOrder()
	for i, z := range zones {
		if z == m.reviewZone {
			m.reviewZone = zones[(i+1)%len(zones)]
			return
		}
	}
	if len(zones) > 0 {
		m.reviewZone = zones[0]
	}
}

func (m *addWizardModel) reviewTabBackward() {
	zones := m.reviewZoneOrder()
	for i, z := range zones {
		if z == m.reviewZone {
			m.reviewZone = zones[(i-1+len(zones))%len(zones)]
			return
		}
	}
	if len(zones) > 0 {
		m.reviewZone = zones[len(zones)-1]
	}
}

func (m *addWizardModel) reviewZoneOrder() []addReviewZone {
	return []addReviewZone{addReviewZoneItems, addReviewZoneButtons}
}

// --- Execute step ---

func (m *addWizardModel) updateKeyExecute(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	if m.executeDone {
		// 'a' key = Add More
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'a' {
			return m, func() tea.Msg { return addRestartMsg{} }
		}
		switch msg.Type {
		case tea.KeyEnter, tea.KeyEsc:
			return m, func() tea.Msg { return addCloseMsg{} }
		case tea.KeyUp:
			if m.executeOffset > 0 {
				m.executeOffset--
			}
		case tea.KeyDown:
			execH := max(3, m.height-8)
			maxOff := max(0, len(m.selectedItems())-execH)
			if m.executeOffset < maxOff {
				m.executeOffset++
			}
		case tea.KeyHome:
			m.executeOffset = 0
		case tea.KeyEnd:
			execH := max(3, m.height-8)
			m.executeOffset = max(0, len(m.selectedItems())-execH)
		}
		return m, nil
	}
	if msg.Type == tea.KeyEsc {
		m.executeCancelled = true
		return m, nil
	}
	return m, nil
}

func (m *addWizardModel) handleExecItemDone(msg addExecItemDoneMsg) (*addWizardModel, tea.Cmd) {
	if msg.seq != m.seq {
		return m, nil
	}
	if msg.index < len(m.executeResults) {
		m.executeResults[msg.index] = msg.result
	}
	m.executeCurrent = msg.index + 1

	if next := m.nextPending(); next >= 0 && !m.executeCancelled {
		return m, m.addItemCmd(next)
	}

	// Mark remaining as cancelled
	for i := m.executeCurrent; i < len(m.executeResults); i++ {
		if m.executeResults[i].status == "" {
			m.executeResults[i] = addExecResult{
				name:   m.selectedItems()[i].name,
				status: "cancelled",
			}
		}
	}

	m.executeDone = true
	m.executing = false
	seq := m.seq
	return m, func() tea.Msg { return addExecAllDoneMsg{seq: seq} }
}
