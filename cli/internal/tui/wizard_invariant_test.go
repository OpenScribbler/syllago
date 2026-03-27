package tui

import (
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Install Wizard invariants ---
//
// These tests verify the step machine's validateStep() assertions.
// Each test walks through steps manually, setting the required state
// at each transition. A panic means the invariant was violated.

func TestInstallWizard_ValidateStep_Forward(t *testing.T) {
	t.Parallel()
	// Walk through all 4 steps for a filesystem (non-JSON-merge) item.
	// No panics should occur.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Step 0: Provider
	w.step = installStepProvider
	w.validateStep() // should not panic

	// Step 1: Location (requires valid provider cursor, not installed)
	w.providerCursor = 0
	w.step = installStepLocation
	w.shell.SetActive(1)
	w.validateStep() // should not panic

	// Step 2: Method (requires valid location cursor, not JSON merge)
	w.locationCursor = 0 // "global"
	w.step = installStepMethod
	w.shell.SetActive(2)
	w.validateStep() // should not panic

	// Step 3: Review (requires valid provider, valid location for filesystem)
	w.step = installStepReview
	w.shell.SetActive(3)
	w.validateStep() // should not panic
}

func TestInstallWizard_ValidateStep_Esc(t *testing.T) {
	t.Parallel()
	// Start at review, walk backwards. No panics.
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Set up to review step
	w.providerCursor = 1
	w.locationCursor = 1
	w.step = installStepReview
	w.shell.SetActive(3)
	w.validateStep() // should not panic

	// Back to method
	w.step = installStepMethod
	w.shell.SetActive(2)
	w.validateStep() // should not panic

	// Back to location
	w.step = installStepLocation
	w.shell.SetActive(1)
	w.validateStep() // should not panic

	// Back to provider
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.validateStep() // should not panic
}

func TestInstallWizard_ValidateStep_AutoSkip(t *testing.T) {
	t.Parallel()
	// Single provider auto-skip: wizard opens at location step.
	prov := testInstallProvider("Claude Code", "claude-code", true)
	root := t.TempDir()
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := openInstallWizard(item, []provider.Provider{prov}, root)

	// openInstallWizard auto-skipped to location
	if w.step != installStepLocation {
		t.Fatalf("expected auto-skip to location, got step %d", w.step)
	}
	w.validateStep() // should not panic at location with auto-skipped provider
}

func TestInstallWizard_ValidateStep_JSONMerge(t *testing.T) {
	t.Parallel()
	// JSON merge path: provider -> review (skip location+method).
	provA := testInstallProvider("Claude Code", "claude-code", true)
	provB := testInstallProvider("Cursor", "cursor", true)
	root := t.TempDir()
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(root, "hooks", "my-hook"))

	w := openInstallWizard(item, []provider.Provider{provA, provB}, root)

	// Step 0: Provider
	w.step = installStepProvider
	w.shell.SetActive(0)
	w.validateStep() // should not panic

	// Step 3 (review): JSON merge skips location+method, but the step enum value
	// is still installStepReview. Shell active is 1 (second of 2 steps).
	w.providerCursor = 0
	w.step = installStepReview
	w.shell.SetActive(1)
	w.validateStep() // should not panic — isJSONMerge means locationCursor < 0 is OK
}

func TestInstallWizard_ValidateStep_PanicsOnEmpty(t *testing.T) {
	t.Parallel()
	// Verify that entering provider step with empty item panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty item at provider step")
		}
		msg, ok := r.(string)
		if !ok || msg != "wizard invariant: installStepProvider entered with empty item" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	// Item with empty Path
	item := catalog.ContentItem{Name: "bad", Type: catalog.Rules}
	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Location", "Method", "Review"}),
		step:              installStepProvider,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{false},
		projectRoot:       root,
	}
	w.validateStep() // should panic
}

func TestInstallWizard_ValidateStep_PanicsOnInstalledLocation(t *testing.T) {
	t.Parallel()
	// Verify that entering location step with installed provider panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for installed provider at location step")
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-rule", catalog.Rules, filepath.Join(root, "rules", "my-rule"))

	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Location", "Method", "Review"}),
		step:              installStepLocation,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{true}, // installed!
		providerCursor:    0,
		projectRoot:       root,
	}
	w.validateStep() // should panic
}

func TestInstallWizard_ValidateStep_PanicsOnJSONMergeMethod(t *testing.T) {
	t.Parallel()
	// Verify that entering method step for JSON merge type panics.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for JSON merge at method step")
		}
	}()

	root := t.TempDir()
	prov := testInstallProvider("Claude Code", "claude-code", true)
	item := testInstallItem("my-hook", catalog.Hooks, filepath.Join(root, "hooks", "my-hook"))

	w := &installWizardModel{
		shell:             newWizardShell("Install", []string{"Provider", "Review"}),
		step:              installStepMethod,
		item:              item,
		providers:         []provider.Provider{prov},
		providerInstalled: []bool{false},
		providerCursor:    0,
		isJSONMerge:       true,
		projectRoot:       root,
	}
	w.validateStep() // should panic
}
