package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
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
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Wizard shell breadcrumb clicks (completed steps are clickable)
	if step, ok := m.shell.HandleClick(msg); ok {
		target := addStep(step)
		if m.preFilterType != "" {
			// 4-step shell: map shell index back to addStep
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
		if target < m.step {
			m.step = target
			m.shell.SetActive(step)
			m.reviewAcknowledged = false
		}
		return m, nil
	}

	// Review step button clicks
	if m.step == addStepReview {
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
	switch msg.Type {
	case tea.KeyEsc:
		m.goBackFromDiscovery()
		return m, nil

	case tea.KeyRight, tea.KeyEnter:
		if len(m.discoveryList.SelectedIndices()) > 0 {
			m.enterReview()
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
	m.discoveredItems = msg.items
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

func (m *addWizardModel) updateKeyReviewRisks(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	m.riskBanner, _ = m.riskBanner.Update(msg)
	return m, nil
}

func (m *addWizardModel) updateKeyReviewItems(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.reviewItemCursor > 0 {
			m.reviewItemCursor--
		}
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.reviewItemCursor < len(m.selectedItems())-1 {
			m.reviewItemCursor++
		}
	}
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
		case 0: // Cancel
			return m, func() tea.Msg { return addCloseMsg{} }
		case 1: // Back
			m.step = addStepDiscovery
			m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
			m.reviewAcknowledged = false
			return m, nil
		case 2: // Add
			if !m.reviewAcknowledged {
				m.reviewAcknowledged = true
				m.enterExecute()
				return m, m.addItemCmd(0)
			}
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
