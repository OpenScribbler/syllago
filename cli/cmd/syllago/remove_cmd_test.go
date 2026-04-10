package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// setupGlobalLibraryWithSkill creates a temp global content dir with one skill.
// Returns the global dir path and the skill directory path.
func setupGlobalLibraryWithSkill(t *testing.T, skillName string) (globalDir string, skillDir string) {
	t.Helper()
	tmp := t.TempDir()
	skillDir = filepath.Join(tmp, "skills", skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+skillName+"\ndescription: test skill\n---\n# "+skillName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return tmp, skillDir
}

func withGlobalDirOverride(t *testing.T, dir string) {
	t.Helper()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
}

func withNonInteractive(t *testing.T) {
	t.Helper()
	orig := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = orig })
}

func TestRemoveRequiresName(t *testing.T) {
	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true

	err := removeCmd.Args(removeCmd, []string{})
	if err == nil {
		t.Error("remove without name should fail args validation")
	}

	err = removeCmd.Args(removeCmd, []string{"a", "b"})
	if err == nil {
		t.Error("remove with two args should fail args validation")
	}
}

func TestRemoveRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "remove" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected remove command registered on rootCmd")
	}
}

func TestRemoveFlags(t *testing.T) {
	if f := removeCmd.Flags().Lookup("force"); f == nil {
		t.Error("expected --force flag on remove command")
	}
	if f := removeCmd.Flags().Lookup("type"); f == nil {
		t.Error("expected --type flag on remove command")
	}
	if f := removeCmd.Flags().Lookup("dry-run"); f == nil {
		t.Error("expected --dry-run flag on remove command")
	}
	if f := removeCmd.Flags().Lookup("no-input"); f == nil {
		t.Error("expected --no-input flag on remove command")
	}
}

func TestRemoveItemNotFound(t *testing.T) {
	globalDir, _ := setupGlobalLibraryWithSkill(t, "my-skill")
	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)

	err := removeCmd.RunE(removeCmd, []string{"nonexistent-skill"})
	if err == nil {
		t.Fatal("expected error for item not found, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-skill") {
		t.Errorf("expected error to mention item name, got: %s", err)
	}
}

func TestRemoveDryRunDoesNotDelete(t *testing.T) {
	globalDir, skillDir := setupGlobalLibraryWithSkill(t, "my-skill")
	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)

	stdout, _ := output.SetForTest(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)
	removeCmd.Flags().Set("dry-run", "true")

	err := removeCmd.RunE(removeCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("dry-run remove failed: %v", err)
	}

	// Directory should still exist after dry-run.
	if _, statErr := os.Stat(skillDir); os.IsNotExist(statErr) {
		t.Error("skill directory was deleted during dry-run, expected it to remain")
	}

	out := stdout.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected dry-run prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected item name in dry-run output, got: %s", out)
	}
}

func TestRemoveForceSkipsConfirmation(t *testing.T) {
	globalDir, skillDir := setupGlobalLibraryWithSkill(t, "my-skill")
	withGlobalDirOverride(t, globalDir)
	// Set interactive=true to prove that --force bypasses the prompt even
	// when stdin would be a terminal (we can't test the prompt directly in unit tests).
	origIsInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origIsInteractive })

	stdout, _ := output.SetForTest(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)
	removeCmd.Flags().Set("force", "true")

	err := removeCmd.RunE(removeCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("force remove failed: %v", err)
	}

	// Directory should be gone.
	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Error("skill directory still exists after remove, expected it to be deleted")
	}

	out := stdout.String()
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected item name in output, got: %s", out)
	}
}

func TestRemoveNoInputSkipsConfirmation(t *testing.T) {
	globalDir, skillDir := setupGlobalLibraryWithSkill(t, "no-input-skill")
	withGlobalDirOverride(t, globalDir)
	// Simulate a terminal so we confirm --no-input is what bypasses the prompt,
	// not the TTY state.
	origIsInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origIsInteractive })

	stdout, _ := output.SetForTest(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)
	removeCmd.Flags().Set("no-input", "true")

	err := removeCmd.RunE(removeCmd, []string{"no-input-skill"})
	if err != nil {
		t.Fatalf("--no-input remove failed: %v", err)
	}

	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Error("skill directory still exists after --no-input remove")
	}

	out := stdout.String()
	if !strings.Contains(out, "no-input-skill") {
		t.Errorf("expected item name in output, got: %s", out)
	}
}

func TestRemoveNonInteractiveDeletesWithoutPrompt(t *testing.T) {
	globalDir, skillDir := setupGlobalLibraryWithSkill(t, "auto-removed")
	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)

	stdout, _ := output.SetForTest(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)

	err := removeCmd.RunE(removeCmd, []string{"auto-removed"})
	if err != nil {
		t.Fatalf("non-interactive remove failed: %v", err)
	}

	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Error("skill directory still exists after non-interactive remove")
	}

	out := stdout.String()
	if !strings.Contains(out, "auto-removed") {
		t.Errorf("expected item name in output, got: %s", out)
	}
}

func TestRemoveAmbiguousTypeRequiresFlag(t *testing.T) {
	globalDir := t.TempDir()

	// Create same name in two content types: skills and agents.
	skillDir := filepath.Join(globalDir, "skills", "dual-name")
	agentDir := filepath.Join(globalDir, "agents", "dual-name")
	os.MkdirAll(skillDir, 0755)
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# dual-name\n"), 0644)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# dual-name\n"), 0644)

	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)

	err := removeCmd.RunE(removeCmd, []string{"dual-name"})
	if err == nil {
		t.Fatal("expected error for ambiguous name, got nil")
	}
	if !strings.Contains(err.Error(), "multiple types") {
		t.Errorf("expected error to mention 'multiple types', got: %s", err)
	}
}

func TestRemoveTypeFilterDisambiguates(t *testing.T) {
	globalDir := t.TempDir()

	// Same name in two types.
	skillDir := filepath.Join(globalDir, "skills", "dual-name")
	agentDir := filepath.Join(globalDir, "agents", "dual-name")
	os.MkdirAll(skillDir, 0755)
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# dual-name\n"), 0644)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# dual-name\n"), 0644)

	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)
	output.SetForTest(t)

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)
	removeCmd.Flags().Set("type", "skills")

	err := removeCmd.RunE(removeCmd, []string{"dual-name"})
	if err != nil {
		t.Fatalf("remove with --type failed: %v", err)
	}

	// skills/dual-name should be gone; agents/dual-name should remain.
	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Error("skills/dual-name still exists after targeted remove")
	}
	if _, statErr := os.Stat(agentDir); os.IsNotExist(statErr) {
		t.Error("agents/dual-name was removed when it should have been preserved")
	}
}

// resetRemoveFlags resets all remove command flags to their defaults between tests.
func resetRemoveFlags(t *testing.T) {
	t.Helper()
	removeCmd.Flags().Set("type", "")
	removeCmd.Flags().Set("force", "false")
	removeCmd.Flags().Set("dry-run", "false")
	removeCmd.Flags().Set("no-input", "false")
}

func TestRemoveJSONOutput(t *testing.T) {
	globalDir, _ := setupGlobalLibraryWithSkill(t, "json-skill")
	withGlobalDirOverride(t, globalDir)
	withNonInteractive(t)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	removeCmd.SilenceUsage = true
	removeCmd.SilenceErrors = true
	resetRemoveFlags(t)

	err := removeCmd.RunE(removeCmd, []string{"json-skill"})
	if err != nil {
		t.Fatalf("remove --json failed: %v", err)
	}

	var result removeResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	if result.Name != "json-skill" {
		t.Errorf("expected name %q, got %q", "json-skill", result.Name)
	}
	if result.Type == "" {
		t.Error("expected non-empty type in JSON output")
	}
	if result.RemovedPath == "" {
		t.Error("expected non-empty removed_path in JSON output")
	}
}
