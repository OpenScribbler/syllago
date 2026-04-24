package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestInstallWizard_ReviewShowsMonolithicHint verifies that when the user
// picks method=append in the method step and advances to review, the review
// step renders the provider-specific monolithic hint (D10). For codex the
// hint is "Codex prefers per-directory AGENTS.md files...".
func TestInstallWizard_ReviewShowsMonolithicHint(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Codex", "codex", true)
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	item := testInstallItem("my-rule", catalog.Rules, itemDir)
	item.Files = []string{"rule.md"}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 100
	w.height = 40
	// Single provider auto-skipped → location. Enter → method.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}
	// Pick Append (cursor=2)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 2 {
		t.Fatalf("expected methodCursor=2 before advance, got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()
	if !strings.Contains(view, "Codex prefers per-directory AGENTS.md files") {
		t.Errorf("review view should contain codex monolithic hint; got view:\n%s", view)
	}
}

// TestInstallWizard_ReviewNoHintWhenMethodNotAppend verifies the hint is
// only rendered for method=append. Picking Symlink on the same codex+rule
// must NOT show the hint — it only applies to the monolithic-append flow.
func TestInstallWizard_ReviewNoHintWhenMethodNotAppend(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Codex", "codex", true)
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	item := testInstallItem("my-rule", catalog.Rules, itemDir)
	item.Files = []string{"rule.md"}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 100
	w.height = 40
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location
	// Leave methodCursor at 0 (Symlink)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // review
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()
	if strings.Contains(view, "Codex prefers per-directory AGENTS.md files") {
		t.Errorf("review view should NOT contain monolithic hint for method=symlink; got view:\n%s", view)
	}
}

// TestInstallWizard_ReviewNoHintWhenProviderHasNoHint verifies that
// providers without a MonolithicHint produce no extra line in the review
// view even when method=append. claude-code has a monolithic filename
// (CLAUDE.md) but no hint string — the review must not render an empty
// "note" row for it.
func TestInstallWizard_ReviewNoHintWhenProviderHasNoHint(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	item := testInstallItem("my-rule", catalog.Rules, itemDir)
	item.Files = []string{"rule.md"}

	if provider.MonolithicHint("claude-code") != "" {
		t.Fatalf("test precondition: claude-code should have empty MonolithicHint")
	}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 100
	w.height = 40
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})  // Append
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // review
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()
	// Be strict: the "Note:" label is our hint marker. If no hint, no label.
	if strings.Contains(view, "Note:") {
		t.Errorf("review view should not render Note: label when provider has no hint; got view:\n%s", view)
	}
}
