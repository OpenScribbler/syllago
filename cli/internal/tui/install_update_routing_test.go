package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestInstallWizard_RoutesCleanThroughUpdateModal verifies D17 Case A:
// when the rule is already installed at the Clean state, pressing Install on
// the review step opens installUpdateModal instead of emitting
// installResultMsg (no file mutation until the modal's decision is made).
func TestInstallWizard_RoutesCleanThroughUpdateModal(t *testing.T) {
	// Stub scan to report Clean state for any wizard instance. Restore on
	// cleanup to avoid cross-test bleed.
	orig := installWizardScanFn
	installWizardScanFn = func(projectRoot string, item catalog.ContentItem) wizardScanResult {
		return wizardScanResult{
			state:        installcheck.StateClean,
			reason:       installcheck.ReasonNone,
			targetFile:   "/tmp/CLAUDE.md",
			recordedHash: "sha256:abc123def4",
			newHash:      "sha256:ffffeeee12",
		}
	}
	t.Cleanup(func() { installWizardScanFn = orig })

	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	if w.step != installStepReview {
		t.Fatalf("expected installStepReview, got %d", w.step)
	}

	// Tab to buttons, right-arrow to Install.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight})
	if w.buttonCursor != 2 {
		t.Fatalf("expected buttonCursor=2 (Install), got %d", w.buttonCursor)
	}

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// D17 Case A: modal must open first. installResultMsg MUST NOT be
	// emitted by this keypress (no silent mutation before the decision).
	if cmd != nil {
		if _, ok := cmd().(installResultMsg); ok {
			t.Fatal("Clean state: Install press must route through update modal, not emit installResultMsg directly")
		}
	}
	if !w.updateModal.IsActive() {
		t.Fatal("expected installUpdateModal to be active after Install press in Clean state")
	}
	if w.modifiedModal.IsActive() {
		t.Error("modifiedModal should NOT be active in Clean state")
	}

	// Modal Enter produces installUpdateDecisionMsg — route it back through
	// the wizard Update (as the runtime would) to get the final
	// installResultMsg carrying decisionAction="replace".
	_, decisionCmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if decisionCmd == nil {
		t.Fatal("expected cmd from modal Enter")
	}
	decisionMsg := decisionCmd()
	if _, ok := decisionMsg.(installUpdateDecisionMsg); !ok {
		t.Fatalf("expected installUpdateDecisionMsg from modal, got %T", decisionMsg)
	}
	_, resultCmd := w.Update(decisionMsg)
	if resultCmd == nil {
		t.Fatal("expected installResultMsg cmd after decision dispatch")
	}
	final := resultCmd()
	result, ok := final.(installResultMsg)
	if !ok {
		t.Fatalf("expected installResultMsg, got %T", final)
	}
	if result.decisionAction != "replace" {
		t.Errorf("expected decisionAction=replace, got %q", result.decisionAction)
	}
}

// TestInstallWizard_RoutesModifiedEditedThroughModifiedModal verifies D17
// Case B: Modified state routes to installModifiedModal.
func TestInstallWizard_RoutesModifiedEditedThroughModifiedModal(t *testing.T) {
	orig := installWizardScanFn
	installWizardScanFn = func(projectRoot string, item catalog.ContentItem) wizardScanResult {
		return wizardScanResult{
			state:      installcheck.StateModified,
			reason:     installcheck.ReasonEdited,
			targetFile: "/tmp/CLAUDE.md",
		}
	}
	t.Cleanup(func() { installWizardScanFn = orig })

	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight})

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		if _, ok := cmd().(installResultMsg); ok {
			t.Fatal("Modified state: must route through modified modal, not emit installResultMsg")
		}
	}
	if !w.modifiedModal.IsActive() {
		t.Fatal("expected installModifiedModal active in Modified/edited state")
	}
	if w.updateModal.IsActive() {
		t.Error("updateModal should NOT be active in Modified state")
	}
}

// TestInstallWizard_FreshEmitsInstallResultDirectly verifies the Fresh path
// still emits installResultMsg without opening any modal (no regression).
func TestInstallWizard_FreshEmitsInstallResultDirectly(t *testing.T) {
	orig := installWizardScanFn
	installWizardScanFn = func(projectRoot string, item catalog.ContentItem) wizardScanResult {
		return wizardScanResult{state: installcheck.StateFresh}
	}
	t.Cleanup(func() { installWizardScanFn = orig })

	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight})

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected installResultMsg cmd for Fresh state")
	}
	msg := cmd()
	result, ok := msg.(installResultMsg)
	if !ok {
		t.Fatalf("expected installResultMsg for Fresh, got %T", msg)
	}
	// Fresh path should record decisionAction="proceed" for downstream
	// telemetry consumers even when no modal was shown.
	if result.decisionAction != "proceed" {
		t.Errorf("Fresh path: expected decisionAction=proceed, got %q", result.decisionAction)
	}
	if w.updateModal.IsActive() || w.modifiedModal.IsActive() {
		t.Error("no modal should be active for Fresh state")
	}
}
