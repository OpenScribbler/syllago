package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestInstallWizard_MethodStep_OffersFileAndAppendForRules verifies that when
// installing a rule to a provider whose slug has a monolithic filename (e.g.
// claude-code → CLAUDE.md), the method step offers three options: Symlink,
// Copy, and an "Append to <filename>" option. Pressing Enter on the default
// selection (cursor=0, Symlink) must advance to the review step just like
// the pre-D5 flow — that's the D5 guarantee that the monolithic option is
// first-class but not forced onto users.
func TestInstallWizard_MethodStep_OffersFileAndAppendForRules(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Single provider auto-skips to location. Enter advances to method.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w.width = 80
	w.height = 30

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	view := w.viewMethod()

	// Symlink + Copy options are the baseline.
	if !strings.Contains(view, "Symlink") {
		t.Error("method view should contain Symlink option")
	}
	if !strings.Contains(view, "Copy") {
		t.Error("method view should contain Copy option")
	}

	// D5: the append option must be visible for rules when the provider has
	// a monolithic filename. MonolithicFilenames("claude-code") returns
	// ["CLAUDE.md"], so the picker must render "Append to CLAUDE.md".
	if !strings.Contains(view, "Append to CLAUDE.md") {
		t.Error("method view should contain 'Append to CLAUDE.md' option for claude-code + rules")
	}

	// Default cursor stays at 0 (Symlink). Enter advances to review; the
	// baseline method behavior must not regress.
	if w.methodCursor != 0 {
		t.Fatalf("expected default methodCursor=0, got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepReview {
		t.Errorf("Enter from method step should advance to review, got step=%d", w.step)
	}
}

// TestInstallWizard_MethodStep_HidesAppendForNonRules verifies the append
// option is only shown for the Rules content type. Skills at claude-code
// must keep the original 2-option picker.
func TestInstallWizard_MethodStep_HidesAppendForNonRules(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w.width = 80
	w.height = 30

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	view := w.viewMethod()
	if strings.Contains(view, "Append to") {
		t.Error("method view should NOT contain append option for non-rule content")
	}
}

// TestInstallWizard_MethodStep_HidesAppendForProviderWithoutMonolithic verifies
// the append option is suppressed for providers that do not author a
// monolithic rule file. This is a rare case for rules but the picker must
// stay clean rather than rendering an empty "Append to " row.
func TestInstallWizard_MethodStep_HidesAppendForProviderWithoutMonolithic(t *testing.T) {
	t.Parallel()
	// "roo-code" has no monolithic filename per provider.MonolithicFilenames.
	prov := testInstallProvider("Roo Code", "roo-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w.width = 80
	w.height = 30

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	view := w.viewMethod()
	if strings.Contains(view, "Append to") {
		t.Error("method view should NOT contain append option for provider without monolithic filename")
	}
}

// TestInstallWizard_MethodStep_AppendNavigation verifies that when the
// append option is present, Down from Copy (index 1) advances to Append
// (index 2), Down from Append clamps, and Up from Append returns to Copy.
func TestInstallWizard_MethodStep_AppendNavigation(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w.width = 80
	w.height = 30

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	// 0 -> 1 -> 2 -> 2 (clamped)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 1 {
		t.Errorf("after Down from 0, expected cursor=1, got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 2 {
		t.Errorf("after Down from 1, expected cursor=2 (Append), got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 2 {
		t.Errorf("after Down from 2, expected cursor=2 (clamped), got %d", w.methodCursor)
	}

	// Up: 2 -> 1 -> 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.methodCursor != 1 {
		t.Errorf("after Up from 2, expected cursor=1, got %d", w.methodCursor)
	}
}

// TestInstallWizard_MethodStep_AppendEmitsMethodAppend verifies that
// selecting the append option and confirming install emits an
// installResultMsg with method=MethodAppend. This is the test that pins
// the downstream wiring — without the install-time branch, the executor
// would try to symlink, which is incorrect for a monolithic file target.
func TestInstallWizard_MethodStep_AppendEmitsMethodAppend(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))
	item.Files = []string{"rule.md"}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 100
	w.height = 40
	// provider auto-skipped -> location. Location Enter -> method.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}
	// Pick Append (cursor=2) via two Down presses.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 2 {
		t.Fatalf("expected methodCursor=2 before advance, got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	// Move focus to the buttons zone and click Install.
	w.setReviewZone(reviewZoneButtons)
	w.buttonCursor = 2 // Install button
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected installResultMsg cmd from Enter on Install button")
	}
	msg, ok := cmd().(installResultMsg)
	if !ok {
		t.Fatalf("expected installResultMsg, got %T", cmd())
	}
	if msg.method != installer.MethodAppend {
		t.Errorf("expected method=MethodAppend, got %q", msg.method)
	}
}
