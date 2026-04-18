package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// writeBuiltinSkill creates a shared-content skill at <projectRoot>/skills/<name>/
// tagged as "builtin" via .syllago.yaml so catalog.Scan treats it as a builtin.
func writeBuiltinSkill(t *testing.T, projectRoot, name string) string {
	t.Helper()
	dir := filepath.Join(projectRoot, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: " + name + "\ndescription: builtin test skill\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0644); err != nil {
		t.Fatal(err)
	}
	meta := "id: " + name + "\nname: " + name + "\ntags:\n  - builtin\n"
	if err := os.WriteFile(filepath.Join(dir, ".syllago.yaml"), []byte(meta), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writePlainSkill creates an un-tagged (non-builtin) skill for negative fixtures.
func writePlainSkill(t *testing.T, projectRoot, name string) string {
	t.Helper()
	dir := filepath.Join(projectRoot, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: " + name + "\ndescription: plain test skill\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// testSkillsProvider builds a provider stub that installs skills under
// <home>/.test-provider/skills/ for use in installBuiltins tests.
func testSkillsProvider() provider.Provider {
	return provider.Provider{
		Name: "Test Provider",
		Slug: "test-provider",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(home, ".test-provider", "skills")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Skills
		},
		Detected: true,
	}
}

func TestInstallBuiltins_NoBuiltinsReturnsNil(t *testing.T) {
	repo := t.TempDir()
	writePlainSkill(t, repo, "not-builtin")

	// Bypass the Scanln prompt regardless of interactivity.
	initCmd.Flags().Set("yes", "true")
	defer initCmd.Flags().Set("yes", "false")

	result := installBuiltins(initCmd, repo, []provider.Provider{testSkillsProvider()})
	if result != nil {
		t.Errorf("expected nil when no builtins are tagged, got %v", result)
	}
}

func TestInstallBuiltins_InstallsTaggedBuiltin(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeBuiltinSkill(t, repo, "alpha")

	initCmd.Flags().Set("yes", "true")
	defer initCmd.Flags().Set("yes", "false")

	result := installBuiltins(initCmd, repo, []provider.Provider{testSkillsProvider()})
	if len(result) != 1 {
		t.Fatalf("expected 1 installed item, got %d: %v", len(result), result)
	}
	if result[0].Name != "alpha" {
		t.Errorf("installed Name = %q, want %q", result[0].Name, "alpha")
	}
	if result[0].Provider != "test-provider" {
		t.Errorf("installed Provider = %q, want %q", result[0].Provider, "test-provider")
	}

	target := filepath.Join(home, ".test-provider", "skills", "alpha")
	if _, err := os.Lstat(target); err != nil {
		t.Errorf("expected install target at %s: %v", target, err)
	}
}

func TestInstallBuiltins_SkipsWhenAlreadyInstalled(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeBuiltinSkill(t, repo, "beta")

	// Pre-create the target so CheckStatus returns StatusInstalled.
	targetDir := filepath.Join(home, ".test-provider", "skills")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "beta"), []byte("already"), 0644); err != nil {
		t.Fatal(err)
	}

	initCmd.Flags().Set("yes", "true")
	defer initCmd.Flags().Set("yes", "false")

	result := installBuiltins(initCmd, repo, []provider.Provider{testSkillsProvider()})
	if len(result) != 0 {
		t.Errorf("expected 0 installs when already installed, got %d: %v", len(result), result)
	}
}

func TestInstallBuiltins_SkipsProviderWithoutSupport(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeBuiltinSkill(t, repo, "gamma")

	// Provider stub that returns no install dir for Skills → StatusNotAvailable.
	noSkillProv := provider.Provider{
		Name: "No-Skills Provider",
		Slug: "no-skills",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return false
		},
		Detected: true,
	}

	initCmd.Flags().Set("yes", "true")
	defer initCmd.Flags().Set("yes", "false")

	result := installBuiltins(initCmd, repo, []provider.Provider{noSkillProv})
	if len(result) != 0 {
		t.Errorf("expected 0 installs when provider does not support Skills, got %d: %v", len(result), result)
	}
}

func TestInstallBuiltins_ContinuesPastInstallError(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeBuiltinSkill(t, repo, "epsilon")

	// Block symlink creation by placing a regular file where the install
	// dir's parent should be — MkdirAll will fail with "not a directory".
	blocker := filepath.Join(home, ".test-provider")
	if err := os.WriteFile(blocker, []byte("not-a-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	initCmd.Flags().Set("yes", "true")
	defer initCmd.Flags().Set("yes", "false")

	result := installBuiltins(initCmd, repo, []provider.Provider{testSkillsProvider()})
	if len(result) != 0 {
		t.Errorf("expected 0 installs when install fails, got %d: %v", len(result), result)
	}
}

func TestInstallBuiltins_JSONModeSkipsPrompt(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeBuiltinSkill(t, repo, "delta")

	// JSON output mode bypasses the interactive prompt even without --yes.
	initCmd.Flags().Set("yes", "false")
	defer initCmd.Flags().Set("yes", "false")

	origJSON := output.JSON
	output.JSON = true
	t.Cleanup(func() { output.JSON = origJSON })

	result := installBuiltins(initCmd, repo, []provider.Provider{testSkillsProvider()})
	if len(result) != 1 {
		t.Errorf("expected 1 install in JSON mode, got %d: %v", len(result), result)
	}
}

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

// TestInitJSONOutputEmitsResult exercises the JSON output branch of runInit
// (lines 173-179). Setting output.JSON = true triggers the initResult emission
// after config/local/gitignore work is done.
func TestInitJSONOutputEmitsResult(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	defer initCmd.Flags().Set("yes", "false")

	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes (json) failed: %v", err)
	}

	var result initResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON initResult on stdout, got %q: %v", stdout.String(), err)
	}
	if result.ConfigPath == "" {
		t.Error("initResult.ConfigPath should be populated")
	}
	if !strings.HasSuffix(result.ConfigPath, filepath.Join(".syllago", "config.json")) {
		t.Errorf("ConfigPath should end with .syllago/config.json, got %q", result.ConfigPath)
	}
}

// TestInitCreatesGlobalConfigWhenMissing exercises the global-config creation
// branch of runInit (lines 154-162). With $HOME pointed at a fresh temp dir,
// the global config does not exist and should be created by SaveGlobal.
func TestInitCreatesGlobalConfigWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Point HOME at a fresh temp dir so ~/.syllago/config.json doesn't exist.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	defer initCmd.Flags().Set("yes", "false")

	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes failed: %v", err)
	}

	globalCfg := filepath.Join(tmpHome, ".syllago", "config.json")
	if _, err := os.Stat(globalCfg); err != nil {
		t.Errorf("expected ~/.syllago/config.json to be created, got stat error: %v", err)
	}
}

// TestInitWizard_InitReturnsNil verifies the wizard's Init command is nil
// (no startup cmd needed — no initial async work).
func TestInitWizard_InitReturnsNil(t *testing.T) {
	w := newInitWizard(nil, nil)
	if cmd := w.Init(); cmd != nil {
		t.Errorf("initWizard.Init() should return nil cmd, got %v", cmd)
	}
}

// TestInitWizard_ViewProviderStep renders step 0 and checks for the expected prompt.
func TestInitWizard_ViewProviderStep(t *testing.T) {
	all := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Cursor", Slug: "cursor", Detected: false},
	}
	w := newInitWizard(all[:1], all)
	v := w.View()

	if !strings.Contains(v, "Which tools") {
		t.Errorf("View should contain the provider-step prompt, got %q", v)
	}
	if !strings.Contains(v, "Claude Code") {
		t.Errorf("View should list Claude Code, got %q", v)
	}
	if !strings.Contains(v, "(detected)") {
		t.Errorf("View should tag detected providers, got %q", v)
	}
	if !strings.Contains(v, "(not found)") {
		t.Errorf("View should tag non-detected providers, got %q", v)
	}
}

// TestInitWizard_ViewRegistryStep renders step 1 and checks for registry options.
func TestInitWizard_ViewRegistryStep(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := w.View()
	if !strings.Contains(v, "Set up a registry") {
		t.Errorf("View should contain registry prompt, got %q", v)
	}
	if !strings.Contains(v, "Skip for now") {
		t.Errorf("View should list skip option, got %q", v)
	}
}

// TestInitWizard_ViewInputStepForAddURL renders step 2 in add-URL mode.
func TestInitWizard_ViewInputStepForAddURL(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	// Advance provider -> registry -> select "Add custom" (cursor 1) -> input
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := w.View()
	if !strings.Contains(v, "Enter the registry URL") {
		t.Errorf("View should contain URL prompt, got %q", v)
	}
}

// TestInitWizard_ViewInputStepForCreate renders step 2 in create mode.
func TestInitWizard_ViewInputStepForCreate(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Move to "Create new registry" (cursor 2) and press Enter
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	v := w.View()
	if !strings.Contains(v, "Enter a name") {
		t.Errorf("View should contain create-name prompt, got %q", v)
	}
}

// TestInitWizard_RegistryCursorNavigation exercises up/down nav and bounds at the registry step.
func TestInitWizard_RegistryCursorNavigation(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // into registry step

	if w.registryCursor != 0 {
		t.Fatalf("registryCursor should start at 0, got %d", w.registryCursor)
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.registryCursor != 1 {
		t.Errorf("expected 1 after one down, got %d", w.registryCursor)
	}
	// Walk to bottom
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	if w.registryCursor != 3 {
		t.Errorf("registryCursor should clamp to 3 at the bottom, got %d", w.registryCursor)
	}
	// Walk back up past top
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyUp})
	if w.registryCursor != 0 {
		t.Errorf("registryCursor should clamp to 0 at the top, got %d", w.registryCursor)
	}
}

// TestInitWizard_RegistryEscCancels verifies Esc on the registry step cancels.
func TestInitWizard_RegistryEscCancels(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !w.cancelled {
		t.Error("Esc on registry step should set cancelled")
	}
	if !w.done {
		t.Error("Esc on registry step should set done")
	}
}

// TestInitWizard_InputAddURLConfirms exercises the add-URL happy path through stepInput.
func TestInitWizard_InputAddURLConfirms(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown}) // add custom
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type a URL one rune at a time
	for _, r := range "https://example.com/r.git" {
		w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("entering a URL and pressing Enter should finish the wizard")
	}
	if w.registryAction != "add" {
		t.Errorf("registryAction should be 'add', got %q", w.registryAction)
	}
	if w.registryURL != "https://example.com/r.git" {
		t.Errorf("registryURL mismatch, got %q", w.registryURL)
	}
}

// TestInitWizard_InputCreateNameConfirms exercises the create-registry happy path.
func TestInitWizard_InputCreateNameConfirms(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown}) // add
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown}) // create
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	for _, r := range "team-registry" {
		w, _ = w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("entering a name and pressing Enter should finish the wizard")
	}
	if w.registryAction != "create" {
		t.Errorf("registryAction should be 'create', got %q", w.registryAction)
	}
	if w.registryName != "team-registry" {
		t.Errorf("registryName mismatch, got %q", w.registryName)
	}
}

// TestInitWizard_InputEmptyEnterIgnored verifies Enter with blank input does not finish.
func TestInitWizard_InputEmptyEnterIgnored(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter}) // into input step

	// Press Enter without typing anything
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if w.done {
		t.Error("Enter on empty input should be ignored, not finish the wizard")
	}
}

// TestInitWizard_InputEscCancels verifies Esc on stepInput cancels.
func TestInitWizard_InputEscCancels(t *testing.T) {
	w := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !w.cancelled {
		t.Error("Esc on input step should set cancelled")
	}
	if !w.done {
		t.Error("Esc on input step should set done")
	}
}

// TestInitWizardModel_Wrapper exercises the tea.Model-compliant wrapper.
// It verifies Init/View delegation and that Update returns tea.Quit when done.
func TestInitWizardModel_Wrapper(t *testing.T) {
	inner := newInitWizard(nil, []provider.Provider{{Name: "X", Slug: "x"}})
	m := initWizardModel{wizard: inner}

	if cmd := m.Init(); cmd != nil {
		t.Errorf("initWizardModel.Init() should be nil, got %v", cmd)
	}

	if !strings.Contains(m.View(), "Which tools") {
		t.Errorf("Wrapper View should delegate to initWizard.View(), got %q", m.View())
	}

	// Esc the inner wizard via the wrapper. Because the inner becomes done,
	// Update should return tea.Quit.
	nextModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd after Esc, got nil")
	}
	// tea.Quit is a function; compare by function pointer is flaky — execute it and
	// check it returns a tea.QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from cmd, got %T", msg)
	}
	wrapped, ok := nextModel.(initWizardModel)
	if !ok {
		t.Fatalf("Update should return initWizardModel, got %T", nextModel)
	}
	if !wrapped.wizard.cancelled {
		t.Error("inner wizard should be cancelled after Esc")
	}
}

// TestInitWizardModel_NotDoneReturnsNonQuitCmd exercises the not-done branch
// of initWizardModel.Update (cursor nav doesn't finish, so tea.Quit must not fire).
func TestInitWizardModel_NotDoneReturnsNonQuitCmd(t *testing.T) {
	m := initWizardModel{wizard: newInitWizard(nil, []provider.Provider{
		{Name: "A", Slug: "a"},
		{Name: "B", Slug: "b"},
	})}
	nextModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})

	wrapped, ok := nextModel.(initWizardModel)
	if !ok {
		t.Fatalf("Update should return initWizardModel, got %T", nextModel)
	}
	if wrapped.wizard.done {
		t.Error("KeyDown on provider step should not finish the wizard")
	}
	// cmd may be nil (no command) — but it must NOT be tea.Quit.
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Error("KeyDown on provider step must not return tea.Quit")
		}
	}
}

// TestInitWizard_SpaceBoundsGuard exercises the cursor-OOB guard in KeySpace.
func TestInitWizard_SpaceBoundsGuard(t *testing.T) {
	// Build a wizard, then shrink w.checks to force cursor >= len(checks).
	w := newInitWizard(nil, []provider.Provider{
		{Name: "A", Slug: "a"},
		{Name: "B", Slug: "b"},
	})
	w.cursor = 1
	w.checks = w.checks[:1] // cursor(1) >= len(checks)(1)

	w, _ = w.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(w.checks) != 1 {
		t.Errorf("checks should not grow when cursor is OOB, got %d", len(w.checks))
	}
}

// TestInitNoProvidersDetectedPrintsNone exercises the empty-detected-list branch
// of runInit (line 107-109). With AllProviders cleared, DetectedOnly returns [],
// and the non-interactive printer should emit "(none detected)".
func TestInitNoProvidersDetectedPrintsNone(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Clear registered providers so none can match.
	origProviders := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = nil
	t.Cleanup(func() { provider.AllProviders = origProviders })

	// Force the non-interactive branch.
	origIsInteractive := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = origIsInteractive })

	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")
	defer initCmd.Flags().Set("yes", "false")

	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes failed: %v", err)
	}

	// The "none detected" branch still writes an empty-provider config.
	cfg, err := config.Load(tmp)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty Providers when none detected, got %v", cfg.Providers)
	}
}
