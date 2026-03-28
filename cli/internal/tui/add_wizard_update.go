package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/add"
)

// Update handles messages for the add wizard.
func (m *addWizardModel) Update(msg tea.Msg) (*addWizardModel, tea.Cmd) {
	m.validateStep()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case addDiscoveryDoneMsg:
		return m.handleDiscoveryDone(msg)
	case addExecItemDoneMsg:
		return m.handleExecItemDone(msg)
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

	// Wizard shell breadcrumb clicks (completed and previously-visited steps are clickable)
	if step, ok := m.shell.HandleClick(msg); ok {
		target := addStep(step)
		if m.preFilterType != "" {
			switch step {
			case 0:
				target = addStepSource
			case 1:
				target = addStepDiscovery
			case 2:
				target = addStepReview
			case 3:
				target = addStepExecute
			}
		}
		if target != m.step && target <= m.maxStep {
			m.step = target
			m.shell.SetActive(step)
			m.reviewAcknowledged = false
		}
		return m, nil
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
	case addStepReview:
		return m.updateMouseReview(msg)
	case addStepExecute:
		return m.updateMouseExecute(msg)
	}

	return m, nil
}

// updateMouseWheel handles scroll wheel events on the current step.
func (m *addWizardModel) updateMouseWheel(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	up := msg.Button == tea.MouseButtonWheelUp

	switch m.step {
	case addStepDiscovery:
		if m.discovering || m.discoveryErr != "" || len(m.discoveredItems) == 0 {
			return m, nil
		}
		if up {
			if m.discoveryList.cursor > 0 {
				m.discoveryList.cursor--
				m.discoveryList.adjustOffset()
			}
		} else {
			if m.discoveryList.cursor < len(m.discoveryList.items)-1 {
				m.discoveryList.cursor++
				m.discoveryList.adjustOffset()
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
			m.advanceFromSource()
			return m, nil
		}
	}

	// Registry sub-list clicks
	for i := range m.registries {
		if zone.Get("add-reg-" + itoa(i)).InBounds(msg) {
			m.registryCursor = i
			m.source = addSourceRegistry
			m.sourceExpanded = false
			m.advanceFromSource()
			return m, nil
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

	// Installed items toggle
	if zone.Get("add-installed-toggle").InBounds(msg) {
		if m.installedCount > 0 {
			m.toggleInstalled()
		}
		return m, nil
	}

	// Next button
	if zone.Get("add-disc-next").InBounds(msg) {
		if len(m.discoveryList.SelectedIndices()) > 0 {
			m.enterReview()
		}
		return m, nil
	}

	// Checkbox list row clicks — toggle selection
	if idx, ok := m.discoveryList.HandleClick(msg); ok {
		m.discoveryList.cursor = idx
		if !m.discoveryList.items[idx].disabled {
			m.discoveryList.selected[idx] = !m.discoveryList.selected[idx]
		}
		return m, nil
	}

	return m, nil
}

// --- Review step mouse ---

func (m *addWizardModel) updateMouseReview(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
	// Drill-in: nav-back closes drill-in
	if m.reviewDrillIn {
		if zone.Get("add-nav-back").InBounds(msg) {
			m.exitReviewDrillIn()
			return m, nil
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
			return m, nil
		}
	}

	// Risk item clicks (delegated to riskBanner's zone marks)
	for i := range m.risks {
		if zone.Get("risk-" + itoa(i)).InBounds(msg) {
			m.riskBanner.cursor = i
			m.reviewZone = addReviewZoneRisks
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
	case addStepReview:
		return m.updateKeyReview(msg)
	case addStepExecute:
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
		m.advanceFromSource()
		return m, nil
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
			// Local path validation
			if !filepath.IsAbs(m.pathInput) {
				m.sourceErr = "Path must be absolute"
				return m, nil
			}
			info, err := os.Stat(m.pathInput)
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
			m.advanceFromSource()
			return m, nil
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
			m.advanceFromSource()
			return m, nil
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

	// Normal results
	switch {
	case msg.Type == tea.KeyEsc:
		m.goBackFromDiscovery()
		return m, nil

	case msg.Type == tea.KeyRight || msg.Type == tea.KeyEnter:
		if len(m.discoveryList.SelectedIndices()) > 0 {
			m.enterReview()
			return m, nil
		}

	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h':
		if m.installedCount > 0 {
			m.toggleInstalled()
			return m, nil
		}

	default:
		var cmd tea.Cmd
		m.discoveryList, cmd = m.discoveryList.Update(msg)
		return m, cmd
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

	// Sort items: actionable (New/Outdated) first, installed (InLibrary) last
	var actionable, installed []addDiscoveryItem
	for _, item := range msg.items {
		if item.status == add.StatusInLibrary {
			installed = append(installed, item)
		} else {
			actionable = append(actionable, item)
		}
	}
	m.actionableCount = len(actionable)
	m.installedCount = len(installed)
	m.discoveredItems = append(actionable, installed...)
	m.showInstalled = false

	if msg.tmpDir != "" {
		m.gitTempDir = msg.tmpDir
	}
	m.sourceRegistry = msg.sourceRegistry
	m.sourceVisibility = msg.sourceVisibility
	m.discoveryList = m.buildDiscoveryList()
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
		// Go back to Discovery (selections preserved)
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
		case addReviewZoneRisks:
			return m.updateKeyReviewRisks(msg)
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
	case tea.KeyLeft, tea.KeyRight:
		m.reviewDrillTree.focused = !m.reviewDrillTree.focused
		m.reviewDrillPreview.focused = !m.reviewDrillTree.focused
	case tea.KeyTab:
		m.reviewDrillTree.focused = !m.reviewDrillTree.focused
		m.reviewDrillPreview.focused = !m.reviewDrillTree.focused
	}
	return m, nil
}

func (m *addWizardModel) updateKeyReviewRisks(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	m.riskBanner, _ = m.riskBanner.Update(msg)
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
	}
	m.adjustReviewOffset()
	return m, nil
}

func (m *addWizardModel) updateKeyReviewButtons(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h'):
		if m.buttonCursor > 0 {
			m.buttonCursor--
		}
	case msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		if m.buttonCursor < 2 {
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
		case 1: // Back
			m.step = addStepDiscovery
			m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
			m.reviewAcknowledged = false
			return m, nil
		case 2: // Cancel
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
	var zones []addReviewZone
	if len(m.risks) > 0 {
		zones = append(zones, addReviewZoneRisks)
	}
	zones = append(zones, addReviewZoneItems)
	zones = append(zones, addReviewZoneButtons)
	return zones
}

// --- Execute step ---

func (m *addWizardModel) updateKeyExecute(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	if m.executeDone {
		if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEsc {
			return m, func() tea.Msg { return addCloseMsg{} }
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
