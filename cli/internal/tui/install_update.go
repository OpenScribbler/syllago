package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
)

// Update handles messages for the install wizard.
func (m *installWizardModel) Update(msg tea.Msg) (*installWizardModel, tea.Cmd) {
	m.validateStep()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	}
	return m, nil
}

func (m *installWizardModel) updateKey(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch m.step {
	case installStepProvider:
		return m.updateKeyProvider(msg)
	case installStepLocation:
		return m.updateKeyLocation(msg)
	case installStepMethod:
		return m.updateKeyMethod(msg)
	case installStepReview:
		return m.updateKeyReview(msg)
	case installStepConflict:
		return m.updateKeyConflict(msg)
	}
	// Fallback Esc
	if msg.Type == tea.KeyEsc {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	return m, nil
}

func (m *installWizardModel) updateKeyProvider(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m, func() tea.Msg { return installCloseMsg{} }

	case msg.Type == tea.KeyDown || msg.Type == tea.KeyTab ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		m.selectAll = false
		if next := m.nextSelectableProvider(m.providerCursor, +1); next >= 0 {
			m.providerCursor = next
		}

	case msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftTab ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		m.selectAll = false
		if next := m.nextSelectableProvider(m.providerCursor, -1); next >= 0 {
			m.providerCursor = next
		}

	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'a':
		// Toggle "All providers" selection (only when option is visible)
		if m.showAllOption() {
			m.selectAll = !m.selectAll
		}

	case msg.Type == tea.KeyEnter:
		if m.selectAll && m.showAllOption() {
			return m.providerEnterAll()
		}
		// Only advance if current provider is not installed and there's at least one selectable
		if m.providerCursor >= 0 && m.providerCursor < len(m.providers) &&
			!m.providerInstalled[m.providerCursor] {
			if m.isJSONMerge {
				// JSON merge skips location+method, goes straight to review
				m.enterReview(1) // Shell step index: Provider=0, Review=1 (2-step shell)
			} else {
				m.step = installStepLocation
				m.shell.SetActive(1)
			}
		}
	}
	return m, nil
}

// providerEnterAll handles Enter when "All providers" is selected.
// Detects conflicts and either emits installAllResultMsg (no conflicts) or
// enters installStepConflict (conflicts found).
func (m *installWizardModel) providerEnterAll() (*installWizardModel, tea.Cmd) {
	home, _ := os.UserHomeDir()
	conflicts := installer.DetectConflicts(m.providers, m.item.Type, home)
	if len(conflicts) == 0 {
		result := m.installAllResult(m.providers)
		return m, func() tea.Msg { return result }
	}
	m.conflicts = conflicts
	m.conflictCursor = 0
	m.shell.SetSteps([]string{"Provider", "Conflicts"})
	m.shell.SetActive(1)
	m.step = installStepConflict
	return m, nil
}

// --- Conflict step ---

func (m *installWizardModel) updateKeyConflict(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.conflictGoBack()

	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.conflictCursor < 2 {
			m.conflictCursor++
		}

	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.conflictCursor > 0 {
			m.conflictCursor--
		}

	case msg.Type == tea.KeyEnter:
		return m.conflictAdvance()
	}
	return m, nil
}

// conflictGoBack returns from conflict step to provider step, restoring shell.
func (m *installWizardModel) conflictGoBack() (*installWizardModel, tea.Cmd) {
	m.conflicts = nil
	// Restore the original shell labels
	m.shell.SetSteps(m.originalShellLabels())
	m.shell.SetActive(0)
	m.step = installStepProvider
	// Keep selectAll=true so the "All providers" row stays highlighted
	return m, nil
}

// conflictAdvance applies the chosen resolution and emits installAllResultMsg.
func (m *installWizardModel) conflictAdvance() (*installWizardModel, tea.Cmd) {
	var resolution installer.ConflictResolution
	switch m.conflictCursor {
	case 0:
		resolution = installer.ResolutionSharedOnly
	case 1:
		resolution = installer.ResolutionOwnDirsOnly
	default:
		resolution = installer.ResolutionAll
	}
	filtered := installer.ApplyConflictResolution(m.providers, m.conflicts, resolution)
	result := m.installAllResult(filtered)
	return m, func() tea.Msg { return result }
}

func (m *installWizardModel) updateMouse(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Check wizard shell breadcrumb clicks (completed steps are clickable)
	if step, ok := m.shell.HandleClick(msg); ok {
		// Map shell step index back to install step.
		target := installStep(step)
		if m.isJSONMerge && step == 1 {
			target = installStepReview
		}
		if target < m.step {
			m.navigateToStep(target)
		}
		return m, nil
	}

	switch m.step {
	case installStepProvider:
		return m.updateMouseProvider(msg)
	case installStepLocation:
		return m.updateMouseLocation(msg)
	case installStepMethod:
		return m.updateMouseMethod(msg)
	case installStepReview:
		return m.updateMouseReview(msg)
	case installStepConflict:
		return m.updateMouseConflict(msg)
	}
	return m, nil
}

func (m *installWizardModel) updateMouseProvider(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	// Provider row clicks
	for i := range m.providers {
		if zone.Get(fmt.Sprintf("inst-prov-%d", i)).InBounds(msg) {
			if !m.providerInstalled[i] {
				m.selectAll = false
				m.providerCursor = i
			}
			return m, nil
		}
	}
	// "All providers" row click
	if zone.Get("inst-all").InBounds(msg) && m.showAllOption() {
		m.selectAll = true
		return m, nil
	}
	// Button clicks
	if zone.Get("inst-cancel").InBounds(msg) {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if zone.Get("inst-next").InBounds(msg) {
		if m.selectAll && m.showAllOption() {
			return m.providerEnterAll()
		}
		// Same as Enter for single-provider
		if m.providerCursor >= 0 && m.providerCursor < len(m.providers) &&
			!m.providerInstalled[m.providerCursor] {
			if m.isJSONMerge {
				m.enterReview(1)
			} else {
				m.step = installStepLocation
				m.shell.SetActive(1)
			}
		}
	}
	return m, nil
}

func (m *installWizardModel) updateMouseConflict(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	for i := 0; i < 3; i++ {
		if zone.Get(fmt.Sprintf("inst-conflict-%d", i)).InBounds(msg) {
			m.conflictCursor = i
			return m, nil
		}
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.conflictGoBack()
	}
	if zone.Get("inst-conflict-install").InBounds(msg) {
		return m.conflictAdvance()
	}
	return m, nil
}

// --- Location step ---

func (m *installWizardModel) locationGoBack() (*installWizardModel, tea.Cmd) {
	if m.autoSkippedProvider {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	m.step = installStepProvider
	m.shell.SetActive(0)
	return m, nil
}

func (m *installWizardModel) locationAdvance() (*installWizardModel, tea.Cmd) {
	// Custom path must be non-empty to advance.
	if m.locationCursor == 2 && m.customPath == "" {
		return m, nil
	}
	m.methodCursor = m.defaultMethodCursor()
	m.step = installStepMethod
	m.shell.SetActive(2)
	return m, nil
}

func (m *installWizardModel) updateKeyLocation(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	// When on Custom (cursor==2), text input is active.
	if m.locationCursor == 2 {
		switch msg.Type {
		case tea.KeyEsc:
			return m.locationGoBack()
		case tea.KeyUp:
			m.locationCursor = 1
			return m, nil
		case tea.KeyEnter:
			return m.locationAdvance()
		case tea.KeyBackspace:
			if m.customCursor > 0 {
				runes := []rune(m.customPath)
				m.customPath = string(runes[:m.customCursor-1]) + string(runes[m.customCursor:])
				m.customCursor--
			}
			return m, nil
		case tea.KeyLeft:
			if m.customCursor > 0 {
				m.customCursor--
			}
			return m, nil
		case tea.KeyRight:
			if m.customCursor < len([]rune(m.customPath)) {
				m.customCursor++
			}
			return m, nil
		case tea.KeyHome, tea.KeyCtrlA:
			m.customCursor = 0
			return m, nil
		case tea.KeyEnd, tea.KeyCtrlE:
			m.customCursor = len([]rune(m.customPath))
			return m, nil
		case tea.KeySpace:
			runes := []rune(m.customPath)
			newRunes := make([]rune, 0, len(runes)+1)
			newRunes = append(newRunes, runes[:m.customCursor]...)
			newRunes = append(newRunes, ' ')
			newRunes = append(newRunes, runes[m.customCursor:]...)
			m.customPath = string(newRunes)
			m.customCursor++
			return m, nil
		case tea.KeyRunes:
			runes := []rune(m.customPath)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.customCursor]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.customCursor:]...)
			m.customPath = string(newRunes)
			m.customCursor += len(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	// Global (0) or Project (1) — radio-button mode, no text input.
	switch {
	case msg.Type == tea.KeyEsc:
		return m.locationGoBack()
	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.locationCursor < 2 {
			m.locationCursor++
		}
	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.locationCursor > 0 {
			m.locationCursor--
		}
	case msg.Type == tea.KeyEnter:
		return m.locationAdvance()
	}
	return m, nil
}

func (m *installWizardModel) updateMouseLocation(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	for i := 0; i < 3; i++ {
		if zone.Get(fmt.Sprintf("inst-loc-%d", i)).InBounds(msg) {
			m.locationCursor = i
			return m, nil
		}
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.locationGoBack()
	}
	if zone.Get("inst-next").InBounds(msg) {
		return m.locationAdvance()
	}
	return m, nil
}

// --- Method step ---

func (m *installWizardModel) methodGoBack() (*installWizardModel, tea.Cmd) {
	m.step = installStepLocation
	m.shell.SetActive(1)
	return m, nil
}

func (m *installWizardModel) methodAdvance() (*installWizardModel, tea.Cmd) {
	m.enterReview(3)
	return m, nil
}

func (m *installWizardModel) updateKeyMethod(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.methodGoBack()

	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.methodCursor == 0 {
			m.methodCursor = 1
		}
		// If at 1 already, stay (no wrap — only 2 options).

	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.methodCursor == 1 && !m.symlinkDisabled() {
			m.methodCursor = 0
		}
		// If symlink disabled or already at 0, stay.

	case msg.Type == tea.KeyEnter:
		return m.methodAdvance()
	}
	return m, nil
}

func (m *installWizardModel) updateMouseMethod(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if zone.Get("inst-method-0").InBounds(msg) && !m.symlinkDisabled() {
		m.methodCursor = 0
		return m, nil
	}
	if zone.Get("inst-method-1").InBounds(msg) {
		m.methodCursor = 1
		return m, nil
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.methodGoBack()
	}
	if zone.Get("inst-next").InBounds(msg) {
		return m.methodAdvance()
	}
	return m, nil
}

// --- Review step ---

// reviewGoBack navigates back from the review step, accounting for JSON merge
// and auto-skipped provider states.
func (m *installWizardModel) reviewGoBack() (*installWizardModel, tea.Cmd) {
	if m.isJSONMerge && m.autoSkippedProvider {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if m.isJSONMerge {
		m.step = installStepProvider
		m.shell.SetActive(0)
		return m, nil
	}
	// Filesystem: back to method
	m.step = installStepMethod
	m.shell.SetActive(2)
	return m, nil
}

func (m *installWizardModel) updateKeyReview(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.reviewGoBack()

	case tea.KeyTab:
		m.reviewTabForward()

	case tea.KeyShiftTab:
		m.reviewTabBackward()

	default:
		// Delegate to zone-specific handler
		switch m.reviewZone {
		case reviewZoneRisks:
			return m.updateKeyReviewRisks(msg)
		case reviewZoneTree:
			return m.updateKeyReviewTree(msg)
		case reviewZonePreview:
			return m.updateKeyReviewPreview(msg)
		case reviewZoneButtons:
			return m.updateKeyReviewButtons(msg)
		}
	}
	return m, nil
}

// reviewTabForward cycles focus: risks -> tree -> preview -> buttons -> risks.
// Zones are skipped when not applicable (no risks, single file).
func (m *installWizardModel) reviewTabForward() {
	zones := m.reviewZoneOrder()
	for i, z := range zones {
		if z == m.reviewZone {
			next := zones[(i+1)%len(zones)]
			m.setReviewZone(next)
			return
		}
	}
	// Fallback: jump to first zone
	m.setReviewZone(zones[0])
}

// reviewTabBackward cycles focus in reverse.
func (m *installWizardModel) reviewTabBackward() {
	zones := m.reviewZoneOrder()
	for i, z := range zones {
		if z == m.reviewZone {
			prev := zones[(i-1+len(zones))%len(zones)]
			m.setReviewZone(prev)
			return
		}
	}
	m.setReviewZone(zones[len(zones)-1])
}

func (m *installWizardModel) updateKeyReviewRisks(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.riskBanner.cursor > 0 {
			m.riskBanner.cursor--
			m.syncPreviewToRisk()
		}
	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.riskBanner.cursor < len(m.risks)-1 {
			m.riskBanner.cursor++
			m.syncPreviewToRisk()
		}
	}
	return m, nil
}

func (m *installWizardModel) updateKeyReviewTree(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		m.reviewTree.CursorUp()
	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		m.reviewTree.CursorDown()
	case msg.Type == tea.KeyEnter:
		if m.reviewTree.cursor >= 0 && m.reviewTree.cursor < len(m.reviewTree.nodes) {
			if m.reviewTree.nodes[m.reviewTree.cursor].isDir {
				m.reviewTree.ToggleDir()
			} else {
				m.loadReviewTreeFile()
			}
		}
	}
	return m, nil
}

func (m *installWizardModel) updateKeyReviewPreview(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyUp ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		m.reviewPreview.ScrollUp()
	case msg.Type == tea.KeyDown ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		m.reviewPreview.ScrollDown()
	case msg.Type == tea.KeyPgUp:
		m.reviewPreview.PageUp()
	case msg.Type == tea.KeyPgDown:
		m.reviewPreview.PageDown()
	}
	return m, nil
}

func (m *installWizardModel) updateKeyReviewButtons(msg tea.KeyMsg) (*installWizardModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyLeft ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h'):
		if m.buttonCursor > 0 {
			m.buttonCursor--
		}
	case msg.Type == tea.KeyRight ||
		(msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		if m.buttonCursor < 2 {
			m.buttonCursor++
		}
	case msg.Type == tea.KeyEnter:
		switch m.buttonCursor {
		case 0: // Cancel
			return m, func() tea.Msg { return installCloseMsg{} }
		case 1: // Back
			return m.reviewGoBack()
		case 2: // Install
			if !m.confirmed {
				m.confirmed = true
				result := m.installResult()
				return m, func() tea.Msg { return result }
			}
		}
	}
	return m, nil
}

func (m *installWizardModel) updateMouseReview(msg tea.MouseMsg) (*installWizardModel, tea.Cmd) {
	if zone.Get("inst-cancel").InBounds(msg) {
		return m, func() tea.Msg { return installCloseMsg{} }
	}
	if zone.Get("inst-back").InBounds(msg) {
		return m.reviewGoBack()
	}
	if zone.Get("inst-install").InBounds(msg) {
		if !m.confirmed {
			m.confirmed = true
			result := m.installResult()
			return m, func() tea.Msg { return result }
		}
	}
	// Risk item clicks
	for i := range m.risks {
		if zone.Get(fmt.Sprintf("risk-%d", i)).InBounds(msg) {
			m.riskBanner.cursor = i
			m.setReviewZone(reviewZoneRisks)
			m.syncPreviewToRisk()
			return m, nil
		}
	}
	// File tree node clicks
	for i := range m.reviewTree.nodes {
		if zone.Get("ftnode-" + itoa(i)).InBounds(msg) {
			m.reviewTree.cursor = i
			m.setReviewZone(reviewZoneTree)
			if m.reviewTree.nodes[i].isDir {
				m.reviewTree.ToggleDir()
			} else {
				m.loadReviewTreeFile()
			}
			return m, nil
		}
	}
	return m, nil
}
