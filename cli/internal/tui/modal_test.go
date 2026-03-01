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
