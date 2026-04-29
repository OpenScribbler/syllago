package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Test helpers (install-specific) ---

// testProvider creates a provider stub for install wizard tests.
// Hooks return JSONMergeSentinel; rules use filesystem install.
func testInstallProvider(name, slug string, detected bool) provider.Provider {
	return provider.Provider{
		Name:     name,
		Slug:     slug,
		Detected: detected,
		InstallDir: func(home string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(home, "."+slug, "rules")
			case catalog.Skills:
				return filepath.Join(home, "."+slug, "skills")
			case catalog.Hooks:
				return provider.JSONMergeSentinel
			case catalog.MCP:
				return provider.JSONMergeSentinel
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules || ct == catalog.Skills ||
				ct == catalog.Hooks || ct == catalog.MCP
		},
	}
}

// testInstallItem creates a content item stub for install wizard tests.
func testInstallItem(name string, ct catalog.ContentType, path string) catalog.ContentItem {
	return catalog.ContentItem{
		Name:    name,
		Type:    ct,
		Path:    path,
		Library: true,
	}
}

// --- Open tests ---

func TestInstallWizard_Open(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())

	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider, got %d", w.step)
	}
	if w.isJSONMerge {
		t.Error("expected isJSONMerge=false for rules")
	}
	if len(w.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(w.providers))
	}
	if len(w.providerInstalled) != 2 {
		t.Errorf("expected 2 providerInstalled entries, got %d", len(w.providerInstalled))
	}
	// Neither provider should show as installed (no files on disk)
	for i, installed := range w.providerInstalled {
		if installed {
			t.Errorf("provider %d should not be installed", i)
		}
	}
	if w.autoSkippedProvider {
		t.Error("should not auto-skip with 2 providers")
	}
	if w.itemName != "my-rule" {
		t.Errorf("expected itemName=my-rule, got %s", w.itemName)
	}
	// Shell should have 4 steps for filesystem type
	if len(w.shell.steps) != 4 {
		t.Errorf("expected 4 shell steps, got %d", len(w.shell.steps))
	}
}

func TestInstallWizard_OpenDisplayName(t *testing.T) {
	t.Parallel()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))
	item.DisplayName = "My Fancy Rule"
	prov := testInstallProvider("Claude Code", "claude-code", true)

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())

	if w.itemName != "My Fancy Rule" {
		t.Errorf("expected itemName='My Fancy Rule', got %s", w.itemName)
	}
}

func TestInstallWizard_OpenSingleProvider(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())

	// Single uninstalled provider: auto-skip to location step
	if w.step != installStepLocation {
		t.Errorf("expected step=installStepLocation after auto-skip, got %d", w.step)
	}
	if !w.autoSkippedProvider {
		t.Error("expected autoSkippedProvider=true")
	}
	if w.providerCursor != 0 {
		t.Errorf("expected providerCursor=0, got %d", w.providerCursor)
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (location), got %d", w.shell.active)
	}
}

func TestInstallWizard_OpenSingleProviderInstalled(t *testing.T) {
	t.Parallel()
	// To simulate "installed", we'd need actual files. Since CheckStatus checks
	// the filesystem and nothing is on disk, it will report StatusNotInstalled.
	// Instead, verify the non-installed path (auto-skip) and trust that CheckStatus
	// is tested in the installer package. We test the installed-stays-on-provider
	// path by verifying the logic: if providerInstalled[0] were true, no skip happens.

	// Create a wizard manually to test the installed case
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Force installed state to verify the model would stay on provider step
	// (openInstallWizard already auto-skipped because disk says not installed,
	// so we construct the scenario directly)
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.autoSkippedProvider = false
	w.providerInstalled[0] = true

	// Verify validateStep doesn't panic at provider step with installed provider
	// (it should be fine — the invariant only blocks entering *location* when installed)
	w.validateStep()

	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider, got %d", w.step)
	}
}

func TestInstallWizard_OpenJSONMerge(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(t.TempDir(), "hooks", "my-hook"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())

	if !w.isJSONMerge {
		t.Error("expected isJSONMerge=true for hooks")
	}
	// Shell should have 2 steps for JSON merge type
	if len(w.shell.steps) != 2 {
		t.Errorf("expected 2 shell steps, got %d", len(w.shell.steps))
	}
	if w.shell.steps[0] != "Provider" || w.shell.steps[1] != "Review" {
		t.Errorf("expected steps [Provider, Review], got %v", w.shell.steps)
	}
	// Multi-provider: should stay on provider step
	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider, got %d", w.step)
	}
}

func TestInstallWizard_OpenJSONMergeSingleProvider(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(t.TempDir(), "hooks", "my-hook"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())

	// Single provider + JSON merge: auto-skip provider, jump straight to review
	if w.step != installStepReview {
		t.Errorf("expected step=installStepReview after auto-skip, got %d", w.step)
	}
	if !w.autoSkippedProvider {
		t.Error("expected autoSkippedProvider=true")
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (review in 2-step shell), got %d", w.shell.active)
	}
}

func TestInstallWizard_Close(t *testing.T) {
	t.Parallel()
	// A nil wizard should render empty string (safe for App to call before wizard is set)
	var w *installWizardModel
	if got := w.View(); got != "" {
		t.Errorf("expected empty view for nil wizard, got %q", got)
	}
}

func TestInstallWizard_EscProducesCloseMsg(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov, provB}, t.TempDir())
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected cmd from Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", msg)
	}
}

// --- Provider step tests ---

func TestInstallWizard_ProviderNav(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	provC := testInstallProvider("Gemini CLI", "gemini-cli", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB, provC}, t.TempDir())
	// Mark provider B (index 1) as installed
	w.providerInstalled[1] = true

	// Cursor starts at 0 (first selectable). Press Down: should skip index 1, land on 2.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.providerCursor != 2 {
		t.Errorf("after Down from 0, expected cursor=2 (skip installed 1), got %d", w.providerCursor)
	}

	// Press Up from 2: should skip installed (1), land on 0.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.providerCursor != 0 {
		t.Errorf("after Up from 2, expected cursor=0 (skip installed 1), got %d", w.providerCursor)
	}

	// Go back to 2, then Down should wrap to 0.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown}) // 0 -> 2
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown}) // 2 -> wraps to 0
	if w.providerCursor != 0 {
		t.Errorf("after Down from 2, expected cursor=0 (wrap), got %d", w.providerCursor)
	}
}

func TestInstallWizard_ProviderEnter(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Cursor is at 0, neither installed. Send Enter.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepLocation {
		t.Errorf("expected step=installStepLocation after Enter, got %d", w.step)
	}
	if w.providerCursor != 0 {
		t.Errorf("expected providerCursor=0, got %d", w.providerCursor)
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1, got %d", w.shell.active)
	}
}

func TestInstallWizard_ProviderEnterJSON(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(t.TempDir(), "hooks", "my-hook"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// JSON merge type: Enter should go to review, not location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepReview {
		t.Errorf("expected step=installStepReview for JSON merge, got %d", w.step)
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (review in 2-step shell), got %d", w.shell.active)
	}
}

func TestInstallWizard_ProviderEnterInstalled(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Force cursor to an installed provider
	w.providerInstalled[0] = true
	w.providerCursor = 0

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepProvider {
		t.Errorf("expected step to stay at installStepProvider, got %d", w.step)
	}
}

func TestInstallWizard_ProviderEsc(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected cmd from Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", msg)
	}
}

func TestInstallWizard_ProviderAllInstalled(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w.providerInstalled[0] = true
	w.providerInstalled[1] = true

	// nextSelectableProvider should return -1
	if got := w.nextSelectableProvider(0, +1); got != -1 {
		t.Errorf("expected nextSelectableProvider=-1 when all installed, got %d", got)
	}

	// Enter should be a no-op (step stays at provider)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepProvider {
		t.Errorf("expected step to stay at installStepProvider, got %d", w.step)
	}
}

// --- Location step tests ---

func TestInstallWizard_LocationNav(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	// Single provider auto-skips to location step.
	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	if w.step != installStepLocation {
		t.Fatalf("expected step=installStepLocation, got %d", w.step)
	}
	if w.locationCursor != 0 {
		t.Fatalf("expected locationCursor=0, got %d", w.locationCursor)
	}

	// Down: 0 -> 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.locationCursor != 1 {
		t.Errorf("after Down, expected cursor=1, got %d", w.locationCursor)
	}

	// Down: 1 -> 2
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.locationCursor != 2 {
		t.Errorf("after Down, expected cursor=2, got %d", w.locationCursor)
	}

	// Up from 2: 2 -> 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.locationCursor != 1 {
		t.Errorf("after Up from 2, expected cursor=1, got %d", w.locationCursor)
	}

	// Up from 1: 1 -> 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.locationCursor != 0 {
		t.Errorf("after Up from 1, expected cursor=0, got %d", w.locationCursor)
	}

	// Up from 0: stays at 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.locationCursor != 0 {
		t.Errorf("after Up from 0, expected cursor=0 (clamped), got %d", w.locationCursor)
	}
}

func TestInstallWizard_LocationGlobal(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// locationCursor=0 (Global), press Enter.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepMethod {
		t.Errorf("expected step=installStepMethod, got %d", w.step)
	}
	if w.shell.active != 2 {
		t.Errorf("expected shell.active=2, got %d", w.shell.active)
	}
}

func TestInstallWizard_LocationProject(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Move to Project (cursor 1)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.locationCursor != 1 {
		t.Fatalf("expected locationCursor=1, got %d", w.locationCursor)
	}

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepMethod {
		t.Errorf("expected step=installStepMethod, got %d", w.step)
	}
}

func TestInstallWizard_LocationCustomType(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Move to Custom
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.locationCursor != 2 {
		t.Fatalf("expected locationCursor=2, got %d", w.locationCursor)
	}

	// Type "abc"
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	if w.customPath != "abc" {
		t.Errorf("expected customPath=abc, got %q", w.customPath)
	}
	if w.customCursor != 3 {
		t.Errorf("expected customCursor=3, got %d", w.customCursor)
	}
}

func TestInstallWizard_LocationCustomBackspace(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Type "abc" then Backspace
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if w.customPath != "ab" {
		t.Errorf("expected customPath=ab, got %q", w.customPath)
	}
	if w.customCursor != 2 {
		t.Errorf("expected customCursor=2, got %d", w.customCursor)
	}
}

func TestInstallWizard_LocationCustomEmpty(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})

	// customPath is empty, press Enter — should NOT advance
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepLocation {
		t.Errorf("expected step=installStepLocation (no advance on empty custom), got %d", w.step)
	}
}

func TestInstallWizard_LocationCustomAdvance(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/test")})

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepMethod {
		t.Errorf("expected step=installStepMethod, got %d", w.step)
	}
}

func TestInstallWizard_LocationEscBack(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Advance to location step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepLocation {
		t.Fatalf("expected step=installStepLocation, got %d", w.step)
	}

	// Esc goes back to provider
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider after Esc, got %d", w.step)
	}
	if w.shell.active != 0 {
		t.Errorf("expected shell.active=0, got %d", w.shell.active)
	}
}

func TestInstallWizard_LocationEscAutoSkip(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	if !w.autoSkippedProvider {
		t.Fatalf("expected autoSkippedProvider=true")
	}

	// Esc should close wizard (not go back to provider)
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd from Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", msg)
	}
}

func TestInstallWizard_LocationResolvedPaths(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	projectRoot := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(projectRoot, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, projectRoot)

	// Global path should start with ~
	globalPath := w.resolvedInstallPath(0)
	if !strings.HasPrefix(globalPath, "~") {
		t.Errorf("expected global path to start with '~', got %q", globalPath)
	}

	// Project path should start with .
	projectPath := w.resolvedInstallPath(1)
	if !strings.HasPrefix(projectPath, ".") {
		t.Errorf("expected project path to start with '.', got %q", projectPath)
	}
}

// --- Method step tests ---

func TestInstallWizard_MethodNav(t *testing.T) {
	t.Parallel()
	// Use Skills here rather than Rules so the method picker stays at the
	// baseline 2 options (Symlink + Copy). The D5 append option only shows
	// for Rules — dedicated coverage lives in install_method_test.go.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-skill", catalog.Skills, "/fake/skills/my-skill")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Advance: provider -> location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}
	if w.methodCursor != 0 {
		t.Fatalf("expected methodCursor=0 (Symlink default), got %d", w.methodCursor)
	}

	// Down: 0 -> 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 1 {
		t.Errorf("after Down, expected cursor=1, got %d", w.methodCursor)
	}

	// Down: stays at 1 (only 2 options, no wrap)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 1 {
		t.Errorf("after Down from 1, expected cursor=1 (clamped), got %d", w.methodCursor)
	}

	// Up: 1 -> 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.methodCursor != 0 {
		t.Errorf("after Up, expected cursor=0, got %d", w.methodCursor)
	}

	// Up: stays at 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.methodCursor != 0 {
		t.Errorf("after Up from 0, expected cursor=0 (clamped), got %d", w.methodCursor)
	}
}

func TestInstallWizard_MethodSymlinkDisabled(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Test", "test", true)
	prov.SymlinkSupport = map[catalog.ContentType]bool{
		catalog.Rules: false,
	}
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Single provider auto-skips to location. Enter advances to method.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}
	// Should default to Copy (1) when symlink disabled
	if w.methodCursor != 1 {
		t.Errorf("expected methodCursor=1 (Copy) when symlink disabled, got %d", w.methodCursor)
	}

	// Up from 1 should stay at 1 (symlink is disabled)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.methodCursor != 1 {
		t.Errorf("after Up, expected cursor=1 (symlink disabled), got %d", w.methodCursor)
	}

	// Down from 1 should stay at 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.methodCursor != 1 {
		t.Errorf("after Down, expected cursor=1 (clamped), got %d", w.methodCursor)
	}
}

func TestInstallWizard_MethodEnter(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Advance to method step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	// Enter advances to review
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepReview {
		t.Errorf("expected step=installStepReview after Enter, got %d", w.step)
	}
	if w.shell.active != 3 {
		t.Errorf("expected shell.active=3 (review), got %d", w.shell.active)
	}
}

func TestInstallWizard_MethodEsc(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Advance to method step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method

	if w.step != installStepMethod {
		t.Fatalf("expected step=installStepMethod, got %d", w.step)
	}

	// Esc goes back to location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if w.step != installStepLocation {
		t.Errorf("expected step=installStepLocation after Esc, got %d", w.step)
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (location), got %d", w.shell.active)
	}
}

func TestInstallWizard_JSONMergeSkip(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-hook", catalog.Hooks, "/fake/hooks/my-hook")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// JSON merge with 2 providers: starts at provider step
	if w.step != installStepProvider {
		t.Fatalf("expected step=installStepProvider, got %d", w.step)
	}

	// Enter from provider should skip location+method, go to review
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepReview {
		t.Errorf("expected step=installStepReview (JSON merge skips location+method), got %d", w.step)
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (review in 2-step shell), got %d", w.shell.active)
	}
}

func TestInstallWizard_JSONMergeSingleAuto(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-hook", catalog.Hooks, "/fake/hooks/my-hook")

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())

	// Single provider + hook = auto-skip directly to review
	if w.step != installStepReview {
		t.Errorf("expected step=installStepReview (single provider + JSON merge), got %d", w.step)
	}
	if !w.autoSkippedProvider {
		t.Error("expected autoSkippedProvider=true")
	}
}

func TestInstallWizard_MethodView(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w.width = 80

	view := w.viewMethod()

	if !strings.Contains(view, "Install method") {
		t.Error("view should contain title")
	}
	if !strings.Contains(view, "Symlink") {
		t.Error("view should contain Symlink option")
	}
	if !strings.Contains(view, "Copy") {
		t.Error("view should contain Copy option")
	}
	if !strings.Contains(view, "recommended") {
		t.Error("view should show recommended hint for Symlink")
	}
	if !strings.Contains(view, "Back") {
		t.Error("view should contain Back button")
	}
	if !strings.Contains(view, "Next") {
		t.Error("view should contain Next button")
	}
}

func TestInstallWizard_MethodViewSymlinkDisabled(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Test", "test", true)
	prov.SymlinkSupport = map[catalog.ContentType]bool{
		catalog.Rules: false,
	}
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w.width = 80

	view := w.viewMethod()

	if !strings.Contains(view, "not supported for this provider") {
		t.Error("view should show 'not supported for this provider' for disabled symlink")
	}
}

func TestInstallWizard_ProviderView(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	provC := testInstallProvider("Gemini CLI", "gemini-cli", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB, provC}, t.TempDir())
	w.providerInstalled[1] = true
	w.width = 80

	view := w.viewProvider()

	// Title should mention the item name
	if !strings.Contains(view, "my-rule") {
		t.Error("view should contain item name")
	}

	// Provider names should appear
	for _, name := range []string{"Claude Code", "Cursor", "Gemini CLI"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain provider name %q", name)
		}
	}

	// Installed provider should show status text
	if !strings.Contains(view, "already installed") {
		t.Error("view should show 'already installed' for Cursor")
	}

	// Detected providers should show detected label
	if !strings.Contains(view, "detected") {
		t.Error("view should show 'detected' for uninstalled providers")
	}

	// Cursor indicator should appear
	if !strings.Contains(view, ">") {
		t.Error("view should show '>' cursor indicator")
	}

	// Buttons should render
	if !strings.Contains(view, "Cancel") {
		t.Error("view should contain Cancel button")
	}
	if !strings.Contains(view, "Next") {
		t.Error("view should contain Next button")
	}
}

// --- Review step tests ---

func TestInstallWizard_ReviewRender(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	// Advance: provider -> location -> method -> review
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review
	w.width = 80
	w.height = 30

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()

	if !strings.Contains(view, "Claude Code") {
		t.Error("review view should contain provider name")
	}
	if !strings.Contains(view, "my-rule") {
		t.Error("review view should contain item name")
	}
	if !strings.Contains(view, "Target") {
		t.Error("review view should contain Target label")
	}
	if !strings.Contains(view, "Symlink") {
		t.Error("review view should contain method label")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("review view should contain Cancel button")
	}
	if !strings.Contains(view, "Back") {
		t.Error("review view should contain Back button")
	}
	if !strings.Contains(view, "Install") {
		t.Error("review view should contain Install button")
	}
}

func TestInstallWizard_ReviewJSONMerge(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-hook", catalog.Hooks, "/fake/hooks/my-hook")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w.width = 80
	w.height = 30

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()

	if !strings.Contains(view, "Target") {
		t.Error("JSON merge review should contain Target label")
	}
	if !strings.Contains(view, "JSON merge") {
		t.Error("JSON merge review should contain 'JSON merge' method")
	}
	if !strings.Contains(view, "Claude Code") {
		t.Error("review view should contain provider name")
	}
}

func TestInstallWizard_ReviewRiskBanner(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)

	hookDir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"command":"echo hello"}]}}`
	hookPath := filepath.Join(hookDir, "my-hook")
	if err := os.MkdirAll(hookPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookPath, "hooks.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name:    "my-hook",
		Type:    catalog.Hooks,
		Path:    hookPath,
		Files:   []string{"hooks.json"},
		Library: true,
	}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 80
	w.height = 30

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	view := w.viewReview()

	if !strings.Contains(view, "Runs commands") {
		t.Error("review should show 'Runs commands' risk for hook with command")
	}
	if !strings.Contains(view, "Risk Indicators") {
		t.Error("review should show 'Risk Indicators' in frame")
	}
}

func TestInstallWizard_ReviewConfirm(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	// No risks, single file: focus starts on preview zone.
	// Tab to buttons, then Tab/Right to Install.
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab}) // preview -> buttons (default: Back=1)
	if w.reviewZone != reviewZoneButtons {
		t.Fatalf("expected reviewZone=reviewZoneButtons, got %d", w.reviewZone)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight}) // Back(1) -> Install(2)
	if w.buttonCursor != 2 {
		t.Fatalf("expected buttonCursor=2 (Install), got %d", w.buttonCursor)
	}

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter on Install, got nil")
	}

	msg := cmd()
	result, ok := msg.(installResultMsg)
	if !ok {
		t.Fatalf("expected installResultMsg, got %T", msg)
	}
	if result.provider.Slug != "claude-code" {
		t.Errorf("expected provider slug=claude-code, got %s", result.provider.Slug)
	}
	if result.location != "global" {
		t.Errorf("expected location=global, got %s", result.location)
	}
	if result.method != "symlink" {
		t.Errorf("expected method=symlink, got %s", result.method)
	}
}

func TestInstallWizard_ReviewDoubleConfirm(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	// Tab to buttons, then right to Install
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})   // preview -> buttons
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight}) // Back(1) -> Install(2)

	w, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from first Enter, got nil")
	}

	_, cmd = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd from second Enter (double-confirm prevention)")
	}
}

func TestInstallWizard_ReviewCancel(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	// Tab to buttons, then left to Cancel
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})  // preview -> buttons (default: Back=1)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyLeft}) // Back(1) -> Cancel(0)

	if w.buttonCursor != 0 {
		t.Fatalf("expected buttonCursor=0, got %d", w.buttonCursor)
	}

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Cancel, got nil")
	}
	msg := cmd()
	if _, ok := msg.(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", msg)
	}
}

func TestInstallWizard_ReviewEscBackFilesystem(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if w.step != installStepMethod {
		t.Errorf("expected step=installStepMethod after Esc, got %d", w.step)
	}
	if w.shell.active != 2 {
		t.Errorf("expected shell.active=2 (method), got %d", w.shell.active)
	}
}

func TestInstallWizard_ReviewEscBackJSONMerge(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-hook", catalog.Hooks, "/fake/hooks/my-hook")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider after Esc, got %d", w.step)
	}
	if w.shell.active != 0 {
		t.Errorf("expected shell.active=0 (provider), got %d", w.shell.active)
	}
}

func TestInstallWizard_ReviewEscBackJSONMergeAutoSkip(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-hook", catalog.Hooks, "/fake/hooks/my-hook")

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd from Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(installCloseMsg); !ok {
		t.Errorf("expected installCloseMsg, got %T", msg)
	}
}

func TestInstallWizard_ReviewAutoScroll(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)

	hookDir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"command":"echo hello"}]}}`
	hookPath := filepath.Join(hookDir, "my-hook")
	if err := os.MkdirAll(hookPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookPath, "hooks.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name:    "my-hook",
		Type:    catalog.Hooks,
		Path:    hookPath,
		Files:   []string{"hooks.json"},
		Library: true,
	}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 80
	w.height = 30

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}
	if len(w.risks) == 0 {
		t.Fatal("expected risks to be non-empty")
	}

	// Focus should start on risks zone with auto-scroll applied
	if w.reviewZone != reviewZoneRisks {
		t.Errorf("expected reviewZone=reviewZoneRisks, got %d", w.reviewZone)
	}
	if w.riskBanner.cursor != 0 {
		t.Errorf("expected riskBanner.cursor=0, got %d", w.riskBanner.cursor)
	}

	// Preview should have content loaded (auto-scroll synced to first risk)
	if len(w.reviewPreview.lines) == 0 {
		t.Error("expected preview to have content from auto-scroll")
	}
	if w.reviewPreview.highlightLines == nil {
		t.Error("expected highlight lines to be set from auto-scroll")
	}

	// Verify the view renders without panicking
	view := w.viewReview()
	if !strings.Contains(view, "Runs commands") {
		t.Error("review should show 'Runs commands' risk")
	}
	if !strings.Contains(view, "Risk Indicators") {
		t.Error("review should show 'Risk Indicators' in frame border")
	}
}

func TestInstallWizard_ReviewTabCycle(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)

	hookDir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"command":"echo hello"}]}}`
	hookPath := filepath.Join(hookDir, "my-hook")
	if err := os.MkdirAll(hookPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookPath, "hooks.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name:    "my-hook",
		Type:    catalog.Hooks,
		Path:    hookPath,
		Files:   []string{"hooks.json"},
		Library: true,
	}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 80
	w.height = 30

	// Single file + risks: zones are [risks, preview, buttons]
	if w.reviewZone != reviewZoneRisks {
		t.Fatalf("expected initial zone=reviewZoneRisks, got %d", w.reviewZone)
	}

	// Tab: risks -> preview (single file, no tree)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZonePreview {
		t.Errorf("expected zone=reviewZonePreview after Tab, got %d", w.reviewZone)
	}

	// Tab: preview -> buttons
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZoneButtons {
		t.Errorf("expected zone=reviewZoneButtons after Tab, got %d", w.reviewZone)
	}

	// Tab: buttons -> risks (wraps)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZoneRisks {
		t.Errorf("expected zone=reviewZoneRisks after Tab wrap, got %d", w.reviewZone)
	}

	// Shift-Tab: risks -> buttons (reverse)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if w.reviewZone != reviewZoneButtons {
		t.Errorf("expected zone=reviewZoneButtons after Shift-Tab, got %d", w.reviewZone)
	}
}

func TestInstallWizard_ReviewSingleFile(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")
	item.Files = []string{"rule.md"}

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Single provider auto-skips to location; advance to review
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review
	w.width = 80
	w.height = 30

	if w.step != installStepReview {
		t.Fatalf("expected step=installStepReview, got %d", w.step)
	}

	// No risks, single file: should start on preview zone (tree is skipped)
	if w.reviewZone != reviewZonePreview {
		t.Errorf("expected reviewZone=reviewZonePreview for single file, got %d", w.reviewZone)
	}

	// Tab cycle: preview -> buttons -> preview (no tree, no risks)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZoneButtons {
		t.Errorf("expected zone=reviewZoneButtons, got %d", w.reviewZone)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZonePreview {
		t.Errorf("expected zone=reviewZonePreview (wrap), got %d", w.reviewZone)
	}
}

func TestInstallWizard_ReviewButtonNav(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	// No risks, single file: starts on preview zone
	if w.reviewZone != reviewZonePreview {
		t.Fatalf("expected reviewZone=reviewZonePreview, got %d", w.reviewZone)
	}

	// Tab to buttons zone
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZoneButtons {
		t.Fatalf("expected reviewZone=reviewZoneButtons, got %d", w.reviewZone)
	}
	// Default button is Back(1)
	if w.buttonCursor != 1 {
		t.Fatalf("expected buttonCursor=1 (Back, safe default), got %d", w.buttonCursor)
	}

	// Right: Back(1) -> Install(2)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight})
	if w.buttonCursor != 2 {
		t.Errorf("expected buttonCursor=2 after Right, got %d", w.buttonCursor)
	}

	// Right clamped at 2
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight})
	if w.buttonCursor != 2 {
		t.Errorf("expected buttonCursor=2 (clamped), got %d", w.buttonCursor)
	}

	// Left: Install(2) -> Back(1)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if w.buttonCursor != 1 {
		t.Errorf("expected buttonCursor=1 after Left, got %d", w.buttonCursor)
	}

	// Left: Back(1) -> Cancel(0)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if w.buttonCursor != 0 {
		t.Errorf("expected buttonCursor=0 after Left, got %d", w.buttonCursor)
	}

	// Left clamped at 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if w.buttonCursor != 0 {
		t.Errorf("expected buttonCursor=0 (clamped), got %d", w.buttonCursor)
	}
}

func TestInstallWizard_ReviewBackButton(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	// Tab to buttons zone (Back is default)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})
	if w.reviewZone != reviewZoneButtons {
		t.Fatalf("expected reviewZone=reviewZoneButtons, got %d", w.reviewZone)
	}
	if w.buttonCursor != 1 {
		t.Fatalf("expected buttonCursor=1 (Back), got %d", w.buttonCursor)
	}

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepMethod {
		t.Errorf("expected step=installStepMethod after Back, got %d", w.step)
	}
}

func TestInstallWizard_ReviewCopyMethod(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> location
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // location -> method
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})  // method: 0 -> 1 (Copy)
	if w.methodCursor != 1 {
		t.Fatalf("expected methodCursor=1, got %d", w.methodCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // method -> review

	// Tab to buttons, right to Install
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyTab})   // preview -> buttons
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRight}) // Back(1) -> Install(2)

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Install confirm")
	}
	msg := cmd()
	result, ok := msg.(installResultMsg)
	if !ok {
		t.Fatalf("expected installResultMsg, got %T", msg)
	}
	if result.method != "copy" {
		t.Errorf("expected method=copy, got %s", result.method)
	}
}

// --- App integration tests (B9 wiring) ---

// testAppWithLibraryItem creates an App with one library rule item and detected providers.
func testAppWithLibraryItem(t *testing.T) App {
	t.Helper()
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# My Rule"), 0644)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{
				Name:        "my-rule",
				DisplayName: "My Rule",
				Type:        catalog.Rules,
				Path:        itemDir,
				Files:       []string{"rule.md"},
				Library:     true,
			},
		},
	}
	provs := []provider.Provider{
		testInstallProvider("Claude Code", "claude-code", true),
	}
	app := NewApp(cat, provs, "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(App)
}

func TestApp_InstallKeyOpens(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Press 'i' to open the install wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)

	if app.wizardMode != wizardInstall {
		t.Errorf("expected wizardMode=wizardInstall, got %d", app.wizardMode)
	}
	if app.installWizard == nil {
		t.Fatal("expected installWizard to be non-nil")
	}
}

func TestApp_InstallKeyRegistryOnly(t *testing.T) {
	// Registry-only items (not in library) cannot be installed directly.
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{
				Name:     "ext-rule",
				Type:     catalog.Rules,
				Path:     "/fake/rules/ext",
				Files:    []string{"rule.md"},
				Library:  false,
				Registry: "some-registry",
			},
		},
	}
	provs := []provider.Provider{testInstallProvider("Claude Code", "claude-code", true)}
	app := NewApp(cat, provs, "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	m, _ = app.Update(keyRune('i'))
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone for registry-only item, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Error("expected installWizard to remain nil for registry-only item")
	}
	// Should show toast warning
	if !app.toast.visible {
		t.Error("expected toast warning for registry-only item")
	}
}

func TestApp_InstallKeyNoProviders(t *testing.T) {
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# My Rule"), 0644)

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{
				Name:    "my-rule",
				Type:    catalog.Rules,
				Path:    itemDir,
				Files:   []string{"rule.md"},
				Library: true,
			},
		},
	}
	// No detected providers.
	app := NewApp(cat, nil, "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(App)

	m, _ = app.Update(keyRune('i'))
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone with no providers, got %d", app.wizardMode)
	}
	if !app.toast.visible {
		t.Error("expected toast warning for no providers")
	}
}

func TestApp_InstallEscExits(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Fatalf("expected wizardMode=wizardInstall, got %d", app.wizardMode)
	}

	// Esc should produce installCloseMsg which closes wizard.
	// Since the wizard has a single provider, it auto-skips to location.
	// Esc from location (auto-skipped) produces installCloseMsg.
	m, cmd := app.Update(keyPress(tea.KeyEsc))
	app = m.(App)
	if cmd != nil {
		msg := cmd()
		m, _ = app.Update(msg)
		app = m.(App)
	}

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone after Esc, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Error("expected installWizard=nil after Esc")
	}
}

func TestApp_InstallDoneSuccess(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Simulate installDoneMsg without error.
	m, cmd := app.Update(installDoneMsg{
		itemName:     "My Rule",
		providerName: "Claude Code",
		targetPath:   "/some/path",
	})
	app = m.(App)

	// Should show success toast.
	if !app.toast.visible {
		t.Error("expected success toast")
	}
	// The cmd should include a rescan.
	if cmd == nil {
		t.Error("expected non-nil cmd (toast + rescan batch)")
	}
}

func TestApp_InstallDoneError(t *testing.T) {
	app := testAppWithLibraryItem(t)

	m, _ := app.Update(installDoneMsg{
		itemName:     "My Rule",
		providerName: "Claude Code",
		err:          fmt.Errorf("permission denied"),
	})
	app = m.(App)

	if !app.toast.visible {
		t.Error("expected error toast")
	}
}

func TestApp_InstallResultClosesWizard(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Fatalf("expected wizardMode=wizardInstall, got %d", app.wizardMode)
	}

	// Send installResultMsg directly (as if wizard confirmed).
	m, cmd := app.Update(installResultMsg{
		item:        app.installWizard.item,
		provider:    app.installWizard.providers[0],
		location:    "global",
		method:      "symlink",
		projectRoot: app.projectRoot,
	})
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone after installResultMsg, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Error("expected installWizard=nil after installResultMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (async install)")
	}
}

func TestApp_InstallCloseMsg(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)

	// Send installCloseMsg directly.
	m, _ = app.Update(installCloseMsg{})
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone after installCloseMsg, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Error("expected installWizard=nil after installCloseMsg")
	}
}

func TestApp_WizardCapturesKeys(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Fatalf("expected wizardMode=wizardInstall, got %d", app.wizardMode)
	}

	// Press '1' — should NOT switch groups (captured by wizard).
	m, _ = app.Update(keyRune('1'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Error("wizard mode should still be active after pressing '1'")
	}

	// Press 'q' — should NOT quit (captured by wizard).
	m, _ = app.Update(keyRune('q'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Error("wizard mode should still be active after pressing 'q'")
	}

	// Press 'R' — should NOT refresh (captured by wizard).
	m, _ = app.Update(keyRune('R'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Error("wizard mode should still be active after pressing 'R'")
	}
}

func TestApp_InstallDoneAfterClose(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// No wizard open. Send installDoneMsg — should still process (toast + rescan).
	m, cmd := app.Update(installDoneMsg{
		itemName:     "My Rule",
		providerName: "Claude Code",
		targetPath:   "/some/path",
	})
	app = m.(App)

	if !app.toast.visible {
		t.Error("expected toast even without active wizard")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (toast + rescan)")
	}
}

func TestApp_InstallWizardView(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard.
	m, _ := app.Update(keyRune('i'))
	app = m.(App)

	view := app.View()
	// The wizard view should be rendered (not the normal topbar+content).
	// It should contain "Install" (from the wizard shell header).
	if !strings.Contains(view, "Install") {
		t.Error("wizard view should contain 'Install' in the view")
	}
}

// --- Meta panel install button tests ---

func TestMetaPanel_InstallButton(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Path:    "/fake/rules/my-rule",
		Library: true,
	}
	data := metaPanelData{
		installed:  "--",
		canInstall: true,
	}
	view := renderMetaPanel(&item, data, 120)
	if !strings.Contains(view, "[i] Install") {
		t.Error("meta panel should contain '[i] Install' when canInstall is true")
	}
}

func TestMetaPanel_InstallButtonHidden(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Path:    "/fake/rules/my-rule",
		Library: true,
	}
	data := metaPanelData{
		installed:  "CC",
		canInstall: false,
	}
	view := renderMetaPanel(&item, data, 120)
	if strings.Contains(view, "[i] Install") {
		t.Error("meta panel should NOT contain '[i] Install' when canInstall is false")
	}
}

func TestMetaPanel_InstallButtonOrder(t *testing.T) {
	t.Parallel()
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Path:    "/fake/rules/my-rule",
		Library: true,
	}
	data := metaPanelData{
		installed:  "CC",
		canInstall: true,
	}
	view := renderMetaPanel(&item, data, 120)
	// Install should appear before Uninstall
	installIdx := strings.Index(view, "[i] Install")
	uninstallIdx := strings.Index(view, "[x] Uninstall")
	if installIdx < 0 {
		t.Fatal("expected '[i] Install' in view")
	}
	if uninstallIdx < 0 {
		t.Fatal("expected '[x] Uninstall' in view")
	}
	if installIdx > uninstallIdx {
		t.Error("[i] Install should appear before [x] Uninstall")
	}
}

func TestComputeMetaPanelData_CanInstall(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	data := computeMetaPanelData(item, []provider.Provider{prov}, t.TempDir())
	if !data.canInstall {
		t.Error("expected canInstall=true for library item with uninstalled detected provider")
	}
}

func TestComputeMetaPanelData_CanInstallRegistryOnly(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := catalog.ContentItem{
		Name:     "ext-rule",
		Type:     catalog.Rules,
		Path:     filepath.Join(t.TempDir(), "rules", "ext"),
		Library:  false,
		Registry: "some-registry",
	}

	data := computeMetaPanelData(item, []provider.Provider{prov}, t.TempDir())
	if data.canInstall {
		t.Error("expected canInstall=false for registry-only item")
	}
}

func TestComputeMetaPanelData_CanInstallContentRoot(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := catalog.ContentItem{
		Name:    "local-rule",
		Type:    catalog.Rules,
		Path:    filepath.Join(t.TempDir(), "rules", "local"),
		Library: false, // content root item, not global library
	}

	data := computeMetaPanelData(item, []provider.Provider{prov}, t.TempDir())
	if !data.canInstall {
		t.Error("expected canInstall=true for content root item (Library=false, Registry empty)")
	}
}

func TestComputeMetaPanelData_CanInstallNoProviders(t *testing.T) {
	t.Parallel()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	data := computeMetaPanelData(item, nil, t.TempDir())
	if data.canInstall {
		t.Error("expected canInstall=false with no providers")
	}
}

// TestComputeMetaPanelData_CanInstallUndetectedProvider pins the slice 4 contract:
// Detect() is advisory (provider/provider.go:39). The metapanel install hint
// must surface whenever any configured provider supports the type and is not
// already installed — regardless of whether that provider was auto-detected
// on disk. Filtering on prov.Detected hid the [i] Install affordance for users
// running providers from custom paths or via portable installs.
func TestComputeMetaPanelData_CanInstallUndetectedProvider(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", false) // undetected
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(t.TempDir(), "rules", "my-rule"))

	data := computeMetaPanelData(item, []provider.Provider{prov}, t.TempDir())
	if !data.canInstall {
		t.Error("expected canInstall=true for undetected provider that supports the type — Detect() is advisory, must not gate install affordance")
	}
}

func TestApp_LibraryInstallMsg(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Send libraryInstallMsg directly (simulates meta-install click).
	m, _ := app.Update(libraryInstallMsg{})
	a := m.(App)

	if a.wizardMode != wizardInstall {
		t.Errorf("expected wizardMode=wizardInstall after libraryInstallMsg, got %d", a.wizardMode)
	}
	if a.installWizard == nil {
		t.Error("expected installWizard to be non-nil after libraryInstallMsg")
	}
}

// --- Conflict provider helpers ---

// testConflictInstaller creates a provider whose InstallDir for Skills points to sharedPath.
// This simulates Gemini CLI or similar providers that write to a global shared path.
func testConflictInstaller(slug, name, sharedPath string) provider.Provider {
	sp := sharedPath
	return provider.Provider{
		Slug:     slug,
		Name:     name,
		Detected: true,
		Detect:   func(string) bool { return true },
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return sp // writes to shared dir
			}
			return filepath.Join(home, "."+slug, string(ct))
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		SymlinkSupport: map[catalog.ContentType]bool{
			catalog.Skills: true,
		},
	}
}

// testConflictReader creates a provider whose GlobalSharedReadPaths for Skills includes sharedPath.
// This simulates OpenCode or similar providers that read from a global shared path.
func testConflictReader(slug, name, sharedPath string) provider.Provider {
	sp := sharedPath
	return provider.Provider{
		Slug:     slug,
		Name:     name,
		Detected: true,
		Detect:   func(string) bool { return true },
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(home, "."+slug, "skills") // own dir
			}
			return ""
		},
		GlobalSharedReadPaths: func(home string, ct catalog.ContentType) []string {
			if ct == catalog.Skills {
				return []string{sp}
			}
			return nil
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		SymlinkSupport: map[catalog.ContentType]bool{
			catalog.Skills: true,
		},
	}
}

// --- "All providers" / conflict step tests ---

// TestInstallWizard_ProviderView_ShowsAllOption verifies the provider view renders
// an "All providers" option when there are 2+ providers.
func TestInstallWizard_ProviderView_ShowsAllOption(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w.width = 80

	view := w.viewProvider()
	if !strings.Contains(view, "All providers") {
		t.Error("provider view should show 'All providers' option when 2+ providers exist")
	}
}

// TestInstallWizard_ProviderView_NoAllOptionSingle verifies "All providers" is not
// shown when there's only one provider.
func TestInstallWizard_ProviderView_NoAllOptionSingle(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	// Single-provider auto-skips to location; manually reset to provider step for view test.
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.autoSkippedProvider = false
	w.width = 80

	view := w.viewProvider()
	if strings.Contains(view, "All providers") {
		t.Error("provider view should NOT show 'All providers' option for single provider")
	}
}

// TestInstallWizard_AllProviders_KeyA verifies pressing 'a' selects the "all providers" option.
func TestInstallWizard_AllProviders_KeyA(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	if w.selectAll {
		t.Error("expected selectAll=false initially")
	}

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !w.selectAll {
		t.Error("expected selectAll=true after pressing 'a'")
	}
}

// TestInstallWizard_AllProviders_ArrowClearsSelectAll verifies arrow keys clear selectAll.
func TestInstallWizard_AllProviders_ArrowClearsSelectAll(t *testing.T) {
	t.Parallel()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(t.TempDir(), "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, t.TempDir())
	w.selectAll = true

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.selectAll {
		t.Error("expected selectAll=false after pressing Down")
	}
}

// TestInstallWizard_AllProviders_NoConflict verifies that pressing Enter with
// selectAll and no conflicts emits installAllResultMsg directly.
func TestInstallWizard_AllProviders_NoConflict(t *testing.T) {
	t.Parallel()
	// Standard providers with no GlobalSharedReadPaths — no conflicts possible.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true

	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter with all providers (no conflicts)")
	}
	msg := cmd()
	result, ok := msg.(installAllResultMsg)
	if !ok {
		t.Fatalf("expected installAllResultMsg, got %T", msg)
	}
	if len(result.providers) == 0 {
		t.Error("expected non-empty providers list in installAllResultMsg")
	}
}

// TestInstallWizard_AllProviders_HasConflict verifies that pressing Enter with
// selectAll and detected conflicts enters installStepConflict.
func TestInstallWizard_AllProviders_HasConflict(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.step != installStepConflict {
		t.Errorf("expected step=installStepConflict after Enter with conflicts, got %d", w.step)
	}
	if len(w.conflicts) == 0 {
		t.Error("expected conflicts to be populated")
	}
	if w.shell.active != 1 {
		t.Errorf("expected shell.active=1 (Conflicts step), got %d", w.shell.active)
	}
}

// TestInstallWizard_ConflictNav verifies Up/Down navigation in the conflict step.
func TestInstallWizard_ConflictNav(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter conflict step

	if w.step != installStepConflict {
		t.Fatalf("expected installStepConflict, got %d", w.step)
	}
	if w.conflictCursor != 0 {
		t.Fatalf("expected conflictCursor=0 initially, got %d", w.conflictCursor)
	}

	// Down: 0 -> 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.conflictCursor != 1 {
		t.Errorf("expected conflictCursor=1 after Down, got %d", w.conflictCursor)
	}

	// Down: 1 -> 2
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.conflictCursor != 2 {
		t.Errorf("expected conflictCursor=2 after Down, got %d", w.conflictCursor)
	}

	// Down: clamped at 2
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.conflictCursor != 2 {
		t.Errorf("expected conflictCursor=2 (clamped), got %d", w.conflictCursor)
	}

	// Up: 2 -> 1
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.conflictCursor != 1 {
		t.Errorf("expected conflictCursor=1 after Up, got %d", w.conflictCursor)
	}

	// Up: 1 -> 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.conflictCursor != 0 {
		t.Errorf("expected conflictCursor=0 after Up, got %d", w.conflictCursor)
	}

	// Up: clamped at 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.conflictCursor != 0 {
		t.Errorf("expected conflictCursor=0 (clamped), got %d", w.conflictCursor)
	}
}

// TestInstallWizard_ConflictEnterSharedOnly verifies Enter on SharedOnly emits
// installAllResultMsg with the reader provider removed.
func TestInstallWizard_ConflictEnterSharedOnly(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter conflict step

	if w.step != installStepConflict {
		t.Fatalf("expected installStepConflict, got %d", w.step)
	}

	// cursor=0 = SharedOnly. Press Enter.
	_, cmd := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter on conflict step")
	}
	msg := cmd()
	result, ok := msg.(installAllResultMsg)
	if !ok {
		t.Fatalf("expected installAllResultMsg, got %T", msg)
	}
	// SharedOnly removes reader (opencode), keeps installer (gemini-cli)
	if len(result.providers) != 1 {
		t.Errorf("SharedOnly should leave 1 provider, got %d", len(result.providers))
	}
	if result.providers[0].Slug != "gemini-cli" {
		t.Errorf("SharedOnly should keep installer (gemini-cli), got %s", result.providers[0].Slug)
	}
}

// TestInstallWizard_ConflictEsc verifies Esc from conflict step returns to provider step.
func TestInstallWizard_ConflictEsc(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // enter conflict step

	if w.step != installStepConflict {
		t.Fatalf("expected installStepConflict, got %d", w.step)
	}

	// Esc goes back to provider
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if w.step != installStepProvider {
		t.Errorf("expected step=installStepProvider after Esc, got %d", w.step)
	}
	if w.shell.active != 0 {
		t.Errorf("expected shell.active=0 (Provider), got %d", w.shell.active)
	}
	if !w.selectAll {
		t.Error("expected selectAll=true preserved after Esc back to provider")
	}
}

// TestInstallWizard_ConflictView verifies viewConflict renders expected content.
func TestInstallWizard_ConflictView(t *testing.T) {
	t.Parallel()
	sharedPath := t.TempDir()
	provA := testConflictInstaller("gemini-cli", "Gemini CLI", sharedPath)
	provB := testConflictReader("opencode", "OpenCode", sharedPath)
	root := t.TempDir()
	item := testInstallItem("my-skill", catalog.Skills, filepath.Join(root, "skills", "my-skill"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)
	w.selectAll = true
	w.width = 80
	w.height = 30

	// Seed conflict state directly (no Enter needed for view test)
	w.conflicts = []installer.Conflict{{
		SharedPath:   sharedPath,
		InstallingTo: provA,
		AlsoReadBy:   []provider.Provider{provB},
	}}
	w.step = installStepConflict
	w.shell.SetSteps([]string{"Provider", "Conflicts"})
	w.shell.SetActive(1)

	view := w.viewConflict()

	if !strings.Contains(view, "conflict") {
		t.Error("conflict view should mention 'conflict'")
	}
	if !strings.Contains(view, "Shared path only") {
		t.Error("conflict view should show 'Shared path only' option")
	}
	if !strings.Contains(view, "Own dirs only") {
		t.Error("conflict view should show 'Own dirs only' option")
	}
	if !strings.Contains(view, "Install to all") {
		t.Error("conflict view should show 'Install to all' option")
	}
	if !strings.Contains(view, "Back") {
		t.Error("conflict view should contain Back button")
	}
	if !strings.Contains(view, "Install") {
		t.Error("conflict view should contain Install button")
	}
}

// TestApp_InstallAllResultMsg verifies the app handles installAllResultMsg correctly.
func TestApp_InstallAllResultMsg(t *testing.T) {
	app := testAppWithLibraryItem(t)

	// Open wizard
	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Fatalf("expected wizardMode=wizardInstall")
	}

	// Send installAllResultMsg directly (simulates conflict step confirm)
	prov := testInstallProvider("Claude Code", "claude-code", true)
	m, cmd := app.Update(installAllResultMsg{
		item:        app.installWizard.item,
		providers:   []provider.Provider{prov},
		projectRoot: app.projectRoot,
	})
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("expected wizardMode=wizardNone after installAllResultMsg, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Error("expected installWizard=nil after installAllResultMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (async install batch)")
	}
}

// --- Coverage boost: pure helpers + review zone handlers ---

func TestInstallView_TierBadge(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		tier     analyzer.ConfidenceTier
		wantText string
	}{
		{"low", analyzer.TierLow, "Low confidence"},
		{"medium", analyzer.TierMedium, "Medium confidence"},
		{"high", analyzer.TierHigh, "High confidence"},
		{"user", analyzer.TierUser, "User-asserted"},
		{"unknown", analyzer.ConfidenceTier("bogus"), ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tierBadge(tc.tier)
			if tc.wantText == "" {
				if got != "" {
					t.Errorf("expected empty for unknown tier, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantText) {
				t.Errorf("tierBadge(%v) = %q, want to contain %q", tc.tier, got, tc.wantText)
			}
		})
	}
}

func TestInstallWizard_Init_ReturnsNil(t *testing.T) {
	t.Parallel()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, "/fake/rules/my-rule")
	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	if cmd := w.Init(); cmd != nil {
		t.Errorf("expected nil cmd from Init, got %v", cmd)
	}
}

// reviewWizardWithHook builds a wizard advanced to the review step with risks
// populated (hook item triggers the risk analyzer).
func reviewWizardWithHook(t *testing.T) *installWizardModel {
	t.Helper()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	hookDir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"command":"echo hello"}]}}`
	hookPath := filepath.Join(hookDir, "my-hook")
	if err := os.MkdirAll(hookPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hookPath, "hooks.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{
		Name:    "my-hook",
		Type:    catalog.Hooks,
		Path:    hookPath,
		Files:   []string{"hooks.json"},
		Library: true,
	}
	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 80
	w.height = 30
	return w
}

func TestInstallWizard_UpdateKeyReviewRisks_JKNavigates(t *testing.T) {
	t.Parallel()
	w := reviewWizardWithHook(t)
	if w.reviewZone != reviewZoneRisks {
		t.Fatalf("expected reviewZone=reviewZoneRisks, got %d", w.reviewZone)
	}
	// Inject a synthetic second risk so j/k navigation has somewhere to go.
	w.risks = append(w.risks, catalog.RiskIndicator{
		Label:       "synthetic",
		Description: "extra",
		Level:       catalog.RiskMedium,
	})
	w.riskBanner = newRiskBanner(w.risks, 60)

	// 'j' advances cursor
	start := w.riskBanner.cursor
	w.updateKeyReviewRisks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if w.riskBanner.cursor != start+1 {
		t.Errorf("expected cursor=%d after 'j', got %d", start+1, w.riskBanner.cursor)
	}
	// 'k' retreats
	w.updateKeyReviewRisks(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if w.riskBanner.cursor != start {
		t.Errorf("expected cursor=%d after 'k', got %d", start, w.riskBanner.cursor)
	}
	// KeyDown and KeyUp also work
	w.updateKeyReviewRisks(tea.KeyMsg{Type: tea.KeyDown})
	if w.riskBanner.cursor != start+1 {
		t.Errorf("expected cursor=%d after Down, got %d", start+1, w.riskBanner.cursor)
	}
	w.updateKeyReviewRisks(tea.KeyMsg{Type: tea.KeyUp})
	if w.riskBanner.cursor != start {
		t.Errorf("expected cursor=%d after Up, got %d", start, w.riskBanner.cursor)
	}
}

// reviewWizardMultiFile returns a wizard advanced to review step with a 2-file
// item so the tree zone is active.
func reviewWizardMultiFile(t *testing.T) *installWizardModel {
	t.Helper()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "extra.md"), []byte("# B"), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Path:    itemDir,
		Files:   []string{"rule.md", "extra.md"},
		Library: true,
	}
	w := openInstallWizard(item, []provider.Provider{prov}, t.TempDir())
	w.width = 80
	w.height = 30
	// Advance single-provider auto-skip: location -> method -> review
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != installStepReview {
		t.Fatalf("expected installStepReview, got %d", w.step)
	}
	return w
}

func TestInstallWizard_UpdateKeyReviewTree_JKNavigates(t *testing.T) {
	t.Parallel()
	w := reviewWizardMultiFile(t)
	// Move focus to tree (multi-file: zones should be [tree, preview, buttons])
	w.setReviewZone(reviewZoneTree)
	startCursor := w.reviewTree.cursor
	w.updateKeyReviewTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if w.reviewTree.cursor == startCursor {
		t.Error("expected cursor to advance after 'j'")
	}
	w.updateKeyReviewTree(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if w.reviewTree.cursor != startCursor {
		t.Errorf("expected cursor=%d after 'k', got %d", startCursor, w.reviewTree.cursor)
	}
	w.updateKeyReviewTree(tea.KeyMsg{Type: tea.KeyDown})
	w.updateKeyReviewTree(tea.KeyMsg{Type: tea.KeyUp})
}

func TestInstallWizard_UpdateKeyReviewTree_EnterOnFileLoadsPreview(t *testing.T) {
	t.Parallel()
	w := reviewWizardMultiFile(t)
	w.setReviewZone(reviewZoneTree)
	// Cursor defaults to 0; first node should be a file (flat tree, 2 files).
	w.updateKeyReviewTree(tea.KeyMsg{Type: tea.KeyEnter})
	if len(w.reviewPreview.lines) == 0 {
		t.Error("expected preview to be loaded after Enter on file")
	}
}

func TestInstallWizard_UpdateKeyReviewPreview_Scrolls(t *testing.T) {
	t.Parallel()
	w := reviewWizardMultiFile(t)
	// Give preview some content so scroll has effect.
	w.reviewPreview.lines = make([]string, 200)
	for i := range w.reviewPreview.lines {
		w.reviewPreview.lines[i] = fmt.Sprintf("line %d", i)
	}
	w.reviewPreview.height = 10

	w.setReviewZone(reviewZonePreview)
	beforeOff := w.reviewPreview.offset
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyPgDown})
	if w.reviewPreview.offset <= beforeOff {
		t.Errorf("expected offset to advance after PgDown, got %d -> %d", beforeOff, w.reviewPreview.offset)
	}
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyPgUp})
	// j/k scrolling
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyDown})
	w.updateKeyReviewPreview(tea.KeyMsg{Type: tea.KeyUp})
}

func TestInstallWizard_LoadReviewTreeFile_EmptySelectionNoop(t *testing.T) {
	t.Parallel()
	w := reviewWizardMultiFile(t)
	w.reviewPreview.lines = nil
	// Force SelectedPath() to return "".
	w.reviewTree.cursor = -1
	w.loadReviewTreeFile()
	if len(w.reviewPreview.lines) != 0 {
		t.Errorf("expected preview unchanged on empty selection, got %d lines", len(w.reviewPreview.lines))
	}
}
