package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestEnsureGlobalContentDir_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	err := ensureGlobalContentDir(tmp)
	if err != nil {
		t.Fatalf("ensureGlobalContentDir: %v", err)
	}
	contentDir := filepath.Join(tmp, ".syllago", "content")
	if _, statErr := os.Stat(contentDir); os.IsNotExist(statErr) {
		t.Errorf("global content dir should exist at %s", contentDir)
	}
}

func TestInitCreatesConfig(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Reset flag state
	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes failed: %v", err)
	}

	if !config.Exists(tmp) {
		t.Error("config.json should exist after init")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	err := initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Error("init should fail when config already exists (no --force)")
	}
}

func TestInitNonInteractiveSkipsPrompt(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Simulate non-TTY: isInteractive returns false, so no prompt is needed
	// even without --yes flag.
	origIsInteractive := isInteractive
	isInteractive = func() bool { return false }
	defer func() { isInteractive = origIsInteractive }()

	initCmd.Flags().Set("yes", "false")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init in non-interactive mode failed: %v", err)
	}

	if !config.Exists(tmp) {
		t.Error("config.json should exist after init in non-interactive mode")
	}
}

func TestInitShortYesFlag(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Verify the -y shorthand is registered and works
	flag := initCmd.Flags().ShorthandLookup("y")
	if flag == nil {
		t.Fatal("-y shorthand flag not registered on init command")
	}
	if flag.Name != "yes" {
		t.Errorf("-y should be shorthand for --yes, got --%s", flag.Name)
	}

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init -y failed: %v", err)
	}

	if !config.Exists(tmp) {
		t.Error("config.json should exist after init -y")
	}
}

func TestInitForceOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"old"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("force", "true")
	initCmd.Flags().Set("yes", "true")
	err := initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("init --force --yes failed: %v", err)
	}
}

func TestInitCreatesLocalDir(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, "local")); err != nil {
		t.Error("local/ directory should exist after init")
	}
}

func TestInitWritesGitignore(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatal(".gitignore should exist after init")
	}
	content := string(data)
	if !strings.Contains(content, "local/") {
		t.Error(".gitignore should contain local/")
	}
	if !strings.Contains(content, ".syllago/registries/") {
		t.Error(".gitignore should contain .syllago/registries/")
	}
}

func TestInitGitignoreNoDuplicates(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	// Pre-populate .gitignore with one of the entries
	os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("local/\n"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	count := strings.Count(string(data), "local/")
	if count != 1 {
		t.Errorf(".gitignore should contain local/ exactly once, got %d", count)
	}
}

// --- initWizard unit tests ---

func TestInitWizard_DefaultsSelectDetectedProviders(t *testing.T) {
	detected := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
	}
	allProviders := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Cursor", Slug: "cursor", Detected: false},
	}
	w := newInitWizard(detected, allProviders)

	if !w.isChecked(0) {
		t.Error("detected provider should be checked by default")
	}
	if w.isChecked(1) {
		t.Error("non-detected provider should be unchecked by default")
	}
}

func TestInitWizard_SpaceTogglesProvider(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeySpace})
	if w.isChecked(0) {
		t.Error("space should uncheck a checked provider")
	}

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !w.isChecked(0) {
		t.Error("space should re-check an unchecked provider")
	}
}

func TestInitWizard_EnterMarksDone(t *testing.T) {
	// Enter on the provider step now advances to the registry step, then
	// selecting "Skip for now" finishes the wizard.
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	// Advance past provider step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if w.done {
		t.Fatal("Enter on provider step should advance to registry step, not finish")
	}

	// Move cursor to "Skip for now" (index 3) and confirm
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("Selecting skip on registry step should mark wizard as done")
	}
	slugs := w.selectedSlugs()
	if len(slugs) != 1 || slugs[0] != "claude-code" {
		t.Errorf("selectedSlugs should return ['claude-code'], got %v", slugs)
	}
}

func TestInitWizard_EscCancels(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !w.cancelled {
		t.Error("Esc should set cancelled to true")
	}
	if !w.done {
		t.Error("Esc should also mark wizard as done")
	}
}

func TestInitWizard_CursorNavigation(t *testing.T) {
	allProviders := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
		{Name: "Cursor", Slug: "cursor"},
		{Name: "Windsurf", Slug: "windsurf"},
	}
	w := newInitWizard(nil, allProviders)

	if w.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", w.cursor)
	}

	// Move down
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.cursor != 1 {
		t.Errorf("cursor should be 1 after down, got %d", w.cursor)
	}

	// Move down again
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.cursor != 2 {
		t.Errorf("cursor should be 2 after second down, got %d", w.cursor)
	}

	// Can't go past the end
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.cursor != 2 {
		t.Errorf("cursor should stay at 2 at bottom, got %d", w.cursor)
	}

	// Move up
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.cursor != 1 {
		t.Errorf("cursor should be 1 after up, got %d", w.cursor)
	}

	// Can't go above 0
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.cursor != 0 {
		t.Errorf("cursor should stay at 0 at top, got %d", w.cursor)
	}
}

func TestInitWizard_SelectedSlugsEmpty(t *testing.T) {
	allProviders := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code"},
		{Name: "Cursor", Slug: "cursor"},
	}
	w := newInitWizard(nil, allProviders)

	slugs := w.selectedSlugs()
	if len(slugs) != 0 {
		t.Errorf("no providers selected, expected empty slugs, got %v", slugs)
	}
}

func TestInitWizard_EnterAdvancesToRegistryStep(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	// Provider step: Enter should advance to registry step, not set done
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.done {
		t.Error("Enter on provider step should not mark done")
	}
	if w.step != stepRegistry {
		t.Errorf("step should be stepRegistry (%d) after Enter, got %d", stepRegistry, w.step)
	}
}

func TestInitWizard_SkipRegistryMarksDone(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	// Advance past provider step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Move cursor to "Skip for now" (index 3) and select it
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("selecting Skip should mark wizard as done")
	}
	if w.registryAction != "skip" {
		t.Errorf("registryAction should be 'skip', got %q", w.registryAction)
	}
}

func TestInitWizard_OfficialRegistryOption(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	// Advance past provider step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Option 0 is "Add the official syllago meta-registry" — just press Enter
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("selecting official registry should mark wizard as done")
	}
	if w.registryAction != "add" {
		t.Errorf("registryAction should be 'add', got %q", w.registryAction)
	}
	if w.registryURL != registry.OfficialRegistryURL {
		t.Errorf("registryURL should be %q, got %q", registry.OfficialRegistryURL, w.registryURL)
	}
}
