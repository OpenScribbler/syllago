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

// setupGlobalLibrary creates a temp dir structured as a ~/.syllago/content
// global library with a skill inside it. Returns the library root.
func setupGlobalLibrary(t *testing.T) string {
	t.Helper()
	globalDir := t.TempDir()

	// Create a skill at globalDir/skills/my-skill/
	skillDir := filepath.Join(globalDir, "skills", "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nDoes something.\n"), 0644)

	return globalDir
}

// withGlobalLibrary overrides the catalog's global content dir override.
func withGlobalLibrary(t *testing.T, dir string) {
	t.Helper()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = dir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
}

func TestInstallUnknownProvider(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "nonexistent-provider")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestInstallItemNotFound(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")

	err := installCmd.RunE(installCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error when item not found in library")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("expected item name in error message, got: %v", err)
	}
}

func TestInstallDryRunDoesNotWrite(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider", "Test Provider", installBase)

	stdout, _ := output.SetForTest(t)

	installCmd.Flags().Set("to", "test-provider")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("dry-run", "true")
	defer installCmd.Flags().Set("dry-run", "false")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install --dry-run failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
	}

	// Nothing should have been written to the install base.
	entries, _ := os.ReadDir(installBase)
	if len(entries) > 0 {
		t.Errorf("dry-run should not write files, but found entries in install base")
	}
}

func TestInstallTypeFilter(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("type", "rules")
	defer installCmd.Flags().Set("type", "")

	// The library only has a skill, so filtering for rules should yield nothing.
	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install with type filter failed: %v", err)
	}
	// No error expected — empty result is a normal no-op.
}

func TestInstallFlagsRegistered(t *testing.T) {
	flags := []string{"to", "type", "method", "dry-run", "base-dir", "no-input", "all"}
	for _, name := range flags {
		if installCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag to be registered on installCmd", name)
		}
	}
}

func TestInstallJSONOutputOnSuccess(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider-json", "Test Provider JSON", installBase)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	installCmd.Flags().Set("to", "test-provider-json")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install --json failed: %v", err)
	}

	var result installResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	if len(result.Installed) == 0 {
		t.Error("expected at least one installed item in JSON output")
	}
	item := result.Installed[0]
	if item.Name == "" {
		t.Error("expected non-empty name in installed item")
	}
	if item.Type == "" {
		t.Error("expected non-empty type in installed item")
	}
	if item.Method == "" {
		t.Error("expected non-empty method in installed item")
	}
}

func TestInstallWarnsWhenProviderNotDetected(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	// Provider with Detected=false triggers warning.
	addTestProviderOpts(t, "undetected-prov", "Undetected Provider", installBase, false)

	_, stderr := output.SetForTest(t)

	installCmd.Flags().Set("to", "undetected-prov")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "Warning: Undetected Provider not detected") {
		t.Errorf("expected provider-not-detected warning on stderr, got: %s", errOut)
	}
	if !strings.Contains(errOut, "syllago config paths --provider undetected-prov") {
		t.Errorf("expected config paths hint in warning, got: %s", errOut)
	}
}

func TestInstallNoWarningWhenProviderDetected(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	// Provider with Detected=true should NOT trigger warning.
	addTestProviderOpts(t, "detected-prov", "Detected Provider", installBase, true)

	_, stderr := output.SetForTest(t)

	installCmd.Flags().Set("to", "detected-prov")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	errOut := stderr.String()
	if strings.Contains(errOut, "Warning") {
		t.Errorf("expected no warning for detected provider, got: %s", errOut)
	}
}

func TestInstallNoWarningInJSONMode(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProviderOpts(t, "undetected-json", "Undetected JSON", installBase, false)

	_, stderr := output.SetForTest(t)
	output.JSON = true

	installCmd.Flags().Set("to", "undetected-json")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	errOut := stderr.String()
	if strings.Contains(errOut, "Warning") {
		t.Errorf("expected no warning in JSON mode, got: %s", errOut)
	}
}

func TestInstallJSONOutputOnSkip(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	// Use claude-code which uses symlink and won't have the skill installed there,
	// but the provider doesn't support the type if we restrict to an unsupported type.
	// Instead, just use a provider with no matching type via --type filter.
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")
	// Filtering for a type not in the library means 0 items → JSON should still be valid.
	installCmd.Flags().Set("type", "rules")
	defer installCmd.Flags().Set("type", "")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install --json with no matches should not error: %v", err)
	}

	// Even with empty results, output should be valid JSON.
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		// No JSON output when no items were found (function returns early before Print).
		return
	}
	var result installResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
}

func TestInstallRequiresExplicitIntent(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")

	// No name, no --all, no --type → should error.
	err := installCmd.RunE(installCmd, []string{})
	if err == nil {
		t.Fatal("expected error when no name, --all, or --type is specified")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected hint about --all in error, got: %v", err)
	}
}

func TestInstallAllConflictsWithName(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	// --all + name → should error.
	err := installCmd.RunE(installCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when both --all and a name are specified")
	}
	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("expected conflict message, got: %v", err)
	}
}

func TestInstallAllInstallsEverything(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProvider(t, "test-prov-all", "Test Provider All", installBase)

	stdout, _ := output.SetForTest(t)

	installCmd.Flags().Set("to", "test-prov-all")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install --all failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected 'my-skill' in output, got: %s", out)
	}
}
