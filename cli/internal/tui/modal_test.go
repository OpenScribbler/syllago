package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestConfirmModalViewContainsTitle(t *testing.T) {
	m := newConfirmModal("Delete item?", "This cannot be undone.")
	view := m.View()
	if !strings.Contains(view, "Delete item?") {
		t.Error("confirmModal.View() should contain the title text")
	}
	if !strings.Contains(view, "This cannot be undone.") {
		t.Error("confirmModal.View() should contain the body text")
	}
}

func TestConfirmModalEnterConfirms(t *testing.T) {
	m := newConfirmModal("Confirm?", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.confirmed {
		t.Error("Enter should set confirmed=true")
	}
	if updated.active {
		t.Error("Enter should set active=false")
	}
}

func TestConfirmModalEscCancels(t *testing.T) {
	m := newConfirmModal("Confirm?", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.confirmed {
		t.Error("Esc should leave confirmed=false")
	}
	if updated.active {
		t.Error("Esc should set active=false")
	}
}

func TestAppModalCapturesInput(t *testing.T) {
	// When modal is active, key input should not reach the screen handler
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
		focus:  focusModal,
		modal:  newConfirmModal("Test?", ""),
	}
	// Pressing 'n' (cancel) should be intercepted by the modal, not quit the app
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	result, _ := a.Update(qMsg)
	updated := result.(App)
	// Modal should have closed (n = cancel) but app should not have quit
	if updated.modal.active {
		t.Error("modal should be inactive after pressing 'n'")
	}
}

func TestSaveModalViewContainsInput(t *testing.T) {
	m := newSaveModal("filename.md")
	view := m.View()
	if !strings.Contains(view, "Save prompt as:") {
		t.Error("saveModal.View() should contain 'Save prompt as:' label")
	}
}

func TestSaveModalEnterWithValueConfirms(t *testing.T) {
	m := newSaveModal("filename.md")
	// Type a filename into the input
	m.input.SetValue("my-prompt.md")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.confirmed {
		t.Error("Enter with non-empty input should set confirmed=true")
	}
	if updated.value != "my-prompt.md" {
		t.Errorf("value should be 'my-prompt.md', got %q", updated.value)
	}
}

func TestOpenModalMsgOpensModal(t *testing.T) {
	// Sending openModalMsg to App should open the confirmModal
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
	}
	msg := openModalMsg{
		purpose: modalInstall,
		title:   "Install test-tool?",
		body:    "Install using symlink or copy.",
	}
	result, _ := a.Update(msg)
	updated := result.(App)
	if !updated.modal.active {
		t.Error("openModalMsg should set modal.active=true")
	}
	if updated.modal.purpose != modalInstall {
		t.Errorf("modal purpose should be modalInstall, got %d", updated.modal.purpose)
	}
}

func TestEnvSetupModalChooseNavigation(t *testing.T) {
	m := newEnvSetupModal([]string{"API_KEY", "AUTH_TOKEN"})
	if m.step != envStepChoose {
		t.Errorf("initial step should be envStepChoose, got %d", m.step)
	}
	// Down arrow should move method cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.methodCursor != 1 {
		t.Errorf("Down should move methodCursor to 1, got %d", updated.methodCursor)
	}
}

func TestEnvSetupModalChooseEnterNewValue(t *testing.T) {
	m := newEnvSetupModal([]string{"API_KEY"})
	// methodCursor 0 = "Set up new value" → envStepValue
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepValue {
		t.Errorf("Enter on 'Set up new value' should advance to envStepValue, got %d", updated.step)
	}
}

func TestEnvSetupModalChooseEnterAlreadyConfigured(t *testing.T) {
	m := newEnvSetupModal([]string{"API_KEY"})
	// Move to "Already configured"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Enter → envStepSource
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepSource {
		t.Errorf("Enter on 'Already configured' should advance to envStepSource, got %d", updated.step)
	}
}

func TestEnvSetupModalEscSkips(t *testing.T) {
	m := newEnvSetupModal([]string{"API_KEY", "AUTH_TOKEN"})
	// Esc on choose step skips to next var
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.varIdx != 1 {
		t.Errorf("Esc should advance to next var, got varIdx %d", updated.varIdx)
	}
	if updated.step != envStepChoose {
		t.Errorf("should be back at envStepChoose for next var, got %d", updated.step)
	}
}

func TestEnvSetupModalEscOnLastVarCloses(t *testing.T) {
	m := newEnvSetupModal([]string{"API_KEY"})
	// Esc on the only var should close the modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.active {
		t.Error("Esc on last var should close the modal")
	}
}

// TestInstallMethodDescriptionNoOrphanedWords verifies that the install method
// descriptions fit within the modal width without wrapping. A wrapped description
// produces a continuation line that starts with a lowercase word — a visual defect
// where the text appears flush-left instead of indented under the description column.
func TestInstallMethodDescriptionNoOrphanedWords(t *testing.T) {
	p := provider.Provider{
		Name:     "Claude Code",
		Slug:     "claude-code",
		Detected: true,
		InstallDir: func(_ string, _ catalog.ContentType) string {
			return "/tmp/test"
		},
		SupportsType: func(_ catalog.ContentType) bool { return true },
	}
	item := catalog.ContentItem{
		Name: "test-skill",
		Type: catalog.Skills,
	}
	m := newInstallModal(item, []provider.Provider{p}, "/tmp/repo")
	// Advance to the method selection step
	m.step = installStepMethod

	view := m.View()
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		// Strip border characters and spaces to get the visible content
		trimmed := strings.TrimLeft(line, " │")
		if len(trimmed) == 0 {
			continue
		}
		// A wrapped description continuation starts with a lowercase letter after
		// the border and padding have been stripped. These lines indicate description
		// text wrapped and the continuation landed flush-left, unindented.
		if trimmed[0] >= 'a' && trimmed[0] <= 'z' {
			t.Errorf("install method modal has an orphaned lowercase continuation line: %q", line)
		}
	}
}

func TestModalPurposesAreDefined(t *testing.T) {
	// All required modal purposes must be defined and distinct
	purposes := []modalPurpose{
		modalNone, modalInstall, modalUninstall, modalSave, modalPromote, modalAppScript,
	}
	seen := map[modalPurpose]bool{}
	for _, p := range purposes {
		if seen[p] {
			t.Errorf("modalPurpose %d is duplicated", p)
		}
		seen[p] = true
	}
	if len(seen) != 6 {
		t.Errorf("expected 6 distinct modal purposes, got %d", len(seen))
	}
}

// makeDetailModel creates a minimal detailModel for testing key handling.
// tab is set to tabInstall so install-related keys are active.
func makeDetailModel(itemType catalog.ContentType, local bool, installed bool) detailModel {
	item := catalog.ContentItem{
		Name:  "test-item",
		Type:  itemType,
		Path:  "/tmp/test-item",
		Local: local,
	}
	var providers []provider.Provider
	if installed {
		p := provider.Provider{
			Name:     "Claude",
			Detected: true,
			Slug:     "claude",
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == itemType
			},
			InstallDir: func(_ string, _ catalog.ContentType) string {
				return "/tmp/claude"
			},
		}
		providers = []provider.Provider{p}
	}
	m := newDetailModel(item, providers, "/tmp/repo")
	m.activeTab = tabInstall
	// If installed, mark the provider checkbox true
	if installed && len(m.provCheck.checks) > 0 {
		m.provCheck.checks[0] = true
	}
	return m
}

// extractCmd runs a tea.Cmd and returns the resulting tea.Msg (or nil).
func extractCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestUninstallKeyEmitsOpenModalMsg(t *testing.T) {
	// 'u' on a non-Apps, non-Prompts item with installed providers should
	// emit openModalMsg{purpose: modalUninstall} instead of setting confirmAction.
	//
	// We need installedProviders() to return something. For Agents (IsUniversal=true),
	// resolveTarget uses item.Name, so we can create a predictable target file.
	installDir := t.TempDir()
	itemDir := t.TempDir()
	item := catalog.ContentItem{
		Name: "test-agent",
		Type: catalog.Agents,
		Path: itemDir,
	}
	// resolveTarget for Agents uses installDir/item.Name+".md"
	targetFile := installDir + "/test-agent.md"
	if f, err := os.Create(targetFile); err == nil {
		f.Close()
	}
	p := provider.Provider{
		Name:     "TestProvider",
		Detected: true,
		Slug:     "test",
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Agents
		},
		InstallDir: func(_ string, _ catalog.ContentType) string {
			return installDir
		},
	}
	m := newDetailModel(item, []provider.Provider{p}, itemDir)
	m.activeTab = tabInstall
	uMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")}
	_, cmd := m.Update(uMsg)
	msg := extractCmd(cmd)
	oMsg, ok := msg.(openModalMsg)
	if !ok {
		t.Fatalf("pressing u should return openModalMsg, got %T", msg)
	}
	if oMsg.purpose != modalUninstall {
		t.Errorf("openModalMsg.purpose should be modalUninstall, got %d", oMsg.purpose)
	}
}

func TestPromoteKeyEmitsOpenModalMsg(t *testing.T) {
	// 'p' on a local item should emit openModalMsg{purpose: modalPromote}
	// instead of setting confirmAction=actionPromoteConfirm.
	m := makeDetailModel(catalog.MCP, true, false)
	pMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	_, cmd := m.Update(pMsg)
	msg := extractCmd(cmd)
	oMsg, ok := msg.(openModalMsg)
	if !ok {
		t.Fatalf("pressing p should return openModalMsg, got %T", msg)
	}
	if oMsg.purpose != modalPromote {
		t.Errorf("openModalMsg.purpose should be modalPromote, got %d", oMsg.purpose)
	}
}

func TestAppScriptKeyEmitsOpenModalMsg(t *testing.T) {
	// 'i' on an Apps item should emit openModalMsg{purpose: modalAppScript}
	// instead of setting confirmAction=actionAppScriptConfirm.
	m := makeDetailModel(catalog.Apps, false, false)
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	_, cmd := m.Update(iMsg)
	msg := extractCmd(cmd)
	oMsg, ok := msg.(openModalMsg)
	if !ok {
		t.Fatalf("pressing i on Apps should return openModalMsg, got %T", msg)
	}
	if oMsg.purpose != modalAppScript {
		t.Errorf("openModalMsg.purpose should be modalAppScript, got %d", oMsg.purpose)
	}
}

func TestInstallModalEscapeResetsState(t *testing.T) {
	// Open the install modal, advance to step 2 (method selection), then cancel
	// via Escape. The App's instModal must be fully zeroed — not just inactive —
	// so stale location/method choices cannot influence a future install.
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Agents,
		Path: t.TempDir(),
	}
	p := provider.Provider{
		Name:     "TestProvider",
		Detected: true,
		Slug:     "test",
		SupportsType: func(ct catalog.ContentType) bool { return true },
		InstallDir:   func(_ string, _ catalog.ContentType) string { return t.TempDir() },
	}

	// Start with an install modal already open (as if openInstallModalMsg was received)
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
		focus:  focusModal,
		instModal: newInstallModal(item, []provider.Provider{p}, "/tmp/repo"),
	}

	// Advance to step 2: move cursor to "Project" then press Enter to reach method step
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyDown}) // locationCursor → 1 (project)
	a = m.(App)
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to installStepMethod
	a = m.(App)

	if a.instModal.step != installStepMethod {
		t.Fatalf("expected installStepMethod after Enter, got step %d", a.instModal.step)
	}
	if a.instModal.locationCursor != 1 {
		t.Fatalf("expected locationCursor=1 (project), got %d", a.instModal.locationCursor)
	}

	// Escape at method step goes back to location step (one step back, not cancel)
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.instModal.step != installStepLocation {
		t.Fatalf("expected back at installStepLocation, got step %d", a.instModal.step)
	}

	// Escape at location step closes and cancels the modal
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)

	// The modal must be fully reset — zero value, not just active=false
	if a.instModal.active {
		t.Error("instModal.active should be false after cancel")
	}
	if a.instModal.confirmed {
		t.Error("instModal.confirmed should be false after cancel")
	}
	if a.instModal.locationCursor != 0 {
		t.Errorf("instModal.locationCursor should be reset to 0, got %d", a.instModal.locationCursor)
	}
	if a.instModal.methodCursor != 0 {
		t.Errorf("instModal.methodCursor should be reset to 0, got %d", a.instModal.methodCursor)
	}
	if a.instModal.step != installStepLocation {
		t.Errorf("instModal.step should be reset to installStepLocation (0), got %d", a.instModal.step)
	}
}

func TestInstallModalNavigateAwayResetsState(t *testing.T) {
	// Simulate navigating away from screenDetail while instModal is active.
	// The resetInstallModal helper (called by mouse navigation paths) must zero
	// out the modal so a later re-entry to the detail screen starts clean.
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Agents,
		Path: t.TempDir(),
	}
	p := provider.Provider{
		Name:     "TestProvider",
		Detected: true,
		Slug:     "test",
		SupportsType: func(ct catalog.ContentType) bool { return true },
		InstallDir:   func(_ string, _ catalog.ContentType) string { return t.TempDir() },
	}

	// Build an App with a mid-flow install modal: step 2 with a non-default cursor.
	modal := newInstallModal(item, []provider.Provider{p}, "/tmp/repo")
	modal.step = installStepMethod
	modal.locationCursor = 1 // project-level selected
	modal.methodCursor = 1   // copy selected
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: modal,
	}

	// Call resetInstallModal directly — this is what mouse navigation paths invoke
	a.resetInstallModal()

	if a.instModal.active {
		t.Error("instModal.active should be false after reset")
	}
	if a.instModal.confirmed {
		t.Error("instModal.confirmed should be false after reset")
	}
	if a.instModal.step != installStepLocation {
		t.Errorf("instModal.step should be reset to 0 (installStepLocation), got %d", a.instModal.step)
	}
	if a.instModal.locationCursor != 0 {
		t.Errorf("instModal.locationCursor should be reset to 0, got %d", a.instModal.locationCursor)
	}
	if a.instModal.methodCursor != 0 {
		t.Errorf("instModal.methodCursor should be reset to 0, got %d", a.instModal.methodCursor)
	}
}

// ---------------------------------------------------------------------------
// Regression tests: modal escape behavior at the App routing level
// ---------------------------------------------------------------------------

// TestConfirmModalClosesOnEscape verifies that pressing Escape on the App when
// a confirmModal is active closes the modal, returns focus to content, and does
// not set confirmed (no action taken).
func TestConfirmModalClosesOnEscape(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
		focus:  focusModal,
		modal:  newConfirmModal("Delete?", "This cannot be undone."),
	}
	a.modal.purpose = modalUninstall

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if updated.modal.active {
		t.Error("confirmModal should be inactive after Escape")
	}
	if updated.modal.confirmed {
		t.Error("Escape should not confirm the modal (confirmed must stay false)")
	}
	if updated.focus == focusModal {
		t.Errorf("focus should no longer be focusModal after modal closes, got %d", updated.focus)
	}
}

// TestSaveModalClosesOnEscape verifies that pressing Escape when the saveModal
// is active closes it without saving: confirmed stays false and detail.savePath
// is not set.
func TestSaveModalClosesOnEscape(t *testing.T) {
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		saveModal: newSaveModal("filename.md"),
	}
	// Type some text into the save input before pressing Escape.
	a.saveModal.input.SetValue("my-unsaved-prompt.md")

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if updated.saveModal.active {
		t.Error("saveModal should be inactive after Escape")
	}
	if updated.saveModal.confirmed {
		t.Error("Escape should not confirm the save modal")
	}
	if updated.detail.savePath != "" {
		t.Errorf("detail.savePath should be empty after Escape, got %q", updated.detail.savePath)
	}
	if updated.focus == focusModal {
		t.Errorf("focus should no longer be focusModal after saveModal closes, got %d", updated.focus)
	}
}

// TestInstallModalClosesOnEscapeAtLocationStep verifies that pressing Escape
// while the installModal is on its first step (location selection) closes the
// modal entirely and does not trigger an install.
func TestInstallModalClosesOnEscapeAtLocationStep(t *testing.T) {
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Skills,
		Path: "/tmp/test-item",
	}
	p := provider.Provider{
		Name:     "Claude Code",
		Slug:     "claude-code",
		Detected: true,
		SupportsType: func(ct catalog.ContentType) bool { return true },
		InstallDir:   func(_ string, _ catalog.ContentType) string { return "/tmp/claude" },
	}
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: newInstallModal(item, []provider.Provider{p}, "/tmp/repo"),
	}
	// Sanity check: modal starts at the location step.
	if a.instModal.step != installStepLocation {
		t.Fatalf("expected installStepLocation at start, got %d", a.instModal.step)
	}

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if updated.instModal.active {
		t.Error("installModal should be inactive after Escape at location step")
	}
	if updated.instModal.confirmed {
		t.Error("Escape should not confirm the install modal")
	}
	if updated.focus == focusModal {
		t.Errorf("focus should no longer be focusModal after installModal closes, got %d", updated.focus)
	}
}

// TestInstallModalEscapeAtMethodStepNavigatesBack verifies that pressing Escape
// on the method step navigates back to the location step rather than closing
// the modal.
func TestInstallModalEscapeAtMethodStepNavigatesBack(t *testing.T) {
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Skills,
		Path: "/tmp/test-item",
	}
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: newInstallModal(item, nil, "/tmp/repo"),
	}
	// Advance to method step.
	a.instModal.step = installStepMethod

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if !updated.instModal.active {
		t.Error("installModal should remain active after Escape at method step (navigates back)")
	}
	if updated.instModal.step != installStepLocation {
		t.Errorf("Escape at method step should return to installStepLocation, got %d", updated.instModal.step)
	}
	if updated.instModal.confirmed {
		t.Error("Escape should not confirm the install modal")
	}
}

// TestEnvSetupModalClosesOnEscapeWhenLastVar verifies that pressing Escape while
// the envSetupModal is on its only variable (choose step) closes the modal and
// returns focus to content — no env writes occur.
func TestEnvSetupModalClosesOnEscapeWhenLastVar(t *testing.T) {
	a := App{
		width:    80,
		height:   24,
		screen:   screenDetail,
		focus:    focusModal,
		envModal: newEnvSetupModal([]string{"API_KEY"}),
	}
	// Sanity check: modal starts active on the choose step.
	if !a.envModal.active {
		t.Fatal("envModal should be active at start")
	}
	if a.envModal.step != envStepChoose {
		t.Fatalf("expected envStepChoose at start, got %d", a.envModal.step)
	}

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if updated.envModal.active {
		t.Error("envModal should be inactive after Escape on last variable")
	}
	if updated.focus == focusModal {
		t.Errorf("focus should no longer be focusModal after envModal closes, got %d", updated.focus)
	}
}

// TestEnvSetupModalEscapeOnValueStepNavigatesBack verifies that pressing Escape
// on the value-entry step returns to the choose step without closing the modal.
func TestEnvSetupModalEscapeOnValueStepNavigatesBack(t *testing.T) {
	a := App{
		width:    80,
		height:   24,
		screen:   screenDetail,
		focus:    focusModal,
		envModal: newEnvSetupModal([]string{"API_KEY"}),
	}
	// Advance to value step.
	a.envModal.step = envStepValue

	result, _ := a.Update(keyEsc)
	updated := result.(App)

	if !updated.envModal.active {
		t.Error("envModal should remain active after Escape at value step (navigates back to choose)")
	}
	if updated.envModal.step != envStepChoose {
		t.Errorf("Escape at value step should return to envStepChoose, got %d", updated.envModal.step)
	}
	// Focus should still be on modal since it's still active.
	if updated.focus != focusModal {
		t.Errorf("focus should remain focusModal while envModal is still active, got %d", updated.focus)
	}
}

func TestClickAway_ClosesConfirmModal(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
		focus:  focusModal,
		modal:  newConfirmModal("Test", "body"),
	}
	a.modal.purpose = modalInstall

	clickMsg := tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      0,
		Y:      0,
	}
	m, _ := a.Update(clickMsg)
	updated := m.(App)
	if updated.modal.active {
		t.Error("clicking outside modal should close it")
	}
	if updated.modal.confirmed {
		t.Error("click-away should not confirm modal")
	}
	if updated.focus != focusContent {
		t.Errorf("focus should return to focusContent, got %d", updated.focus)
	}
}

func TestClickAway_ClosesSaveModal(t *testing.T) {
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		saveModal: newSaveModal("filename.md"),
	}
	a.saveModal.input.SetValue("my-file.md")

	clickMsg := tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      0,
		Y:      0,
	}
	m, _ := a.Update(clickMsg)
	updated := m.(App)
	if updated.saveModal.active {
		t.Error("clicking outside save modal should close it")
	}
	if updated.saveModal.confirmed {
		t.Error("click-away should not confirm save modal")
	}
}

func TestClickAway_ClosesInstallModal(t *testing.T) {
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Skills,
		Path: "/tmp/test",
	}
	a := App{
		width:     80,
		height:    24,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: newInstallModal(item, nil, ""),
	}

	clickMsg := tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      0,
		Y:      0,
	}
	m, _ := a.Update(clickMsg)
	updated := m.(App)
	if updated.instModal.active {
		t.Error("clicking outside install modal should close it")
	}
	if updated.instModal.confirmed {
		t.Error("click-away should not confirm install modal")
	}
}
