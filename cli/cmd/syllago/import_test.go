package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
)

// settingsJSON and setupHooksProject are defined in add_cmd_test.go.

func TestImportRequiresFrom(t *testing.T) {
	// Reset flags
	importCmd.Flags().Set("from", "")
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import without --from should fail")
	}
}

func TestImportUnknownProvider(t *testing.T) {
	importCmd.Flags().Set("from", "nonexistent")
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import with unknown provider should fail")
	}
}

// setupImportProject creates a temp dir with claude-code rule files for
// testing the import command. Returns the temp dir path.
func setupImportProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	rulesDir := filepath.Join(tmp, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "testing.md"), []byte("# Testing"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "logging.md"), []byte("# Logging"), 0644)

	return tmp
}

// runImportPreviewJSON runs the import command in --preview + JSON mode and
// returns the parsed DiscoveryReport.
func runImportPreviewJSON(t *testing.T, tmp string, nameFilter string) parse.DiscoveryReport {
	t.Helper()

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "")
	importCmd.Flags().Set("name", nameFilter)
	importCmd.Flags().Set("preview", "true")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --preview failed: %v", err)
	}

	var report parse.DiscoveryReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	return report
}

func TestImportNameFilterMatchesSubstring(t *testing.T) {
	tmp := setupImportProject(t)

	report := runImportPreviewJSON(t, tmp, "secur")

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 file matching 'secur', got %d", len(report.Files))
	}
	if filepath.Base(report.Files[0].Path) != "security.md" {
		t.Errorf("expected security.md, got %s", report.Files[0].Path)
	}
}

func TestImportNameFilterCaseInsensitive(t *testing.T) {
	tmp := setupImportProject(t)

	report := runImportPreviewJSON(t, tmp, "TESTING")

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 file matching 'TESTING', got %d", len(report.Files))
	}
	if filepath.Base(report.Files[0].Path) != "testing.md" {
		t.Errorf("expected testing.md, got %s", report.Files[0].Path)
	}
}

func TestImportNameFilterUpdatesCountsCorrectly(t *testing.T) {
	tmp := setupImportProject(t)

	report := runImportPreviewJSON(t, tmp, "logging")

	total := 0
	for _, c := range report.Counts {
		total += c
	}
	if total != 1 {
		t.Errorf("expected total count of 1 after filtering, got %d (counts: %v)", total, report.Counts)
	}
}

func TestImportNoNameFilterReturnsAll(t *testing.T) {
	tmp := setupImportProject(t)

	report := runImportPreviewJSON(t, tmp, "")

	// Should find at least the 3 rule files we created.
	ruleCount := 0
	for _, f := range report.Files {
		base := filepath.Base(f.Path)
		if base == "security.md" || base == "testing.md" || base == "logging.md" {
			ruleCount++
		}
	}
	if ruleCount != 3 {
		t.Errorf("expected 3 rule files without --name filter, found %d", ruleCount)
	}
}

func TestImportNameFilterNoMatch(t *testing.T) {
	tmp := setupImportProject(t)

	report := runImportPreviewJSON(t, tmp, "nonexistent-xyz")

	if len(report.Files) != 0 {
		t.Errorf("expected 0 files for non-matching name, got %d", len(report.Files))
	}
}

func TestImportWritesToLocal(t *testing.T) {
	tmp := setupImportProject(t)

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "rules")
	importCmd.Flags().Set("name", "security")
	importCmd.Flags().Set("preview", "false")
	importCmd.Flags().Set("dry-run", "false")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Verify the file was written to local/rules/claude-code/security/
	itemDir := filepath.Join(tmp, "local", "rules", "claude-code", "security")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("expected item directory at %s, got error: %v", itemDir, err)
	}

	// Check that .syllago.yaml was created
	metaPath := filepath.Join(itemDir, ".syllago.yaml")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("expected metadata at %s, got error: %v", metaPath, err)
	}

	// Check that a content file exists
	contentPath := filepath.Join(itemDir, "rule.md")
	if _, err := os.Stat(contentPath); err != nil {
		t.Errorf("expected content file at %s, got error: %v", contentPath, err)
	}
}

func TestImportDryRunDoesNotWrite(t *testing.T) {
	tmp := setupImportProject(t)

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "rules")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "false")
	importCmd.Flags().Set("dry-run", "true")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --dry-run failed: %v", err)
	}

	// Verify that local/ was NOT created
	localDir := filepath.Join(tmp, "local")
	if _, err := os.Stat(localDir); err == nil {
		t.Errorf("expected local/ to not exist during --dry-run, but it does")
	}

	// Verify output mentions dry-run
	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
	}
}

func TestImportHooksPreview(t *testing.T) {
	tmp := setupHooksProject(t)

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "hooks")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "true")
	importCmd.Flags().Set("dry-run", "false")
	importCmd.Flags().Set("scope", "project")
	importCmd.Flags().Set("force", "false")
	// Reset exclude to empty.
	importCmd.Flags().Set("exclude", "")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --type hooks --preview failed: %v", err)
	}

	// No files should be written.
	localDir := filepath.Join(tmp, "local")
	if _, err := os.Stat(localDir); err == nil {
		t.Errorf("expected local/ to not exist during --preview, but it does")
	}

	// Output should list the hook names and mention "would be imported".
	out := stdout.String()
	if !strings.Contains(out, "would be imported") {
		t.Errorf("expected 'would be imported' in preview output, got: %s", out)
	}
	// Both hooks should be listed.
	if !strings.Contains(out, "pre-bash-check") {
		t.Errorf("expected 'pre-bash-check' hook in preview output, got: %s", out)
	}
	if !strings.Contains(out, "post-edit-check") {
		t.Errorf("expected 'post-edit-check' hook in preview output, got: %s", out)
	}
}

func TestImportHooksWritesToLocal(t *testing.T) {
	tmp := setupHooksProject(t)

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "hooks")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "false")
	importCmd.Flags().Set("dry-run", "false")
	importCmd.Flags().Set("scope", "project")
	importCmd.Flags().Set("force", "false")
	importCmd.Flags().Set("exclude", "")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --type hooks failed: %v", err)
	}

	// Both hooks should have been written to local/hooks/claude-code/<name>/.
	hooksBase := filepath.Join(tmp, "local", "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected local/hooks/claude-code/ to exist, got error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 hook directories, got %d", len(entries))
	}

	// Each directory should contain hook.json and .syllago.yaml.
	for _, entry := range entries {
		itemDir := filepath.Join(hooksBase, entry.Name())
		if _, err := os.Stat(filepath.Join(itemDir, "hook.json")); err != nil {
			t.Errorf("expected hook.json in %s, got error: %v", itemDir, err)
		}
		if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
			t.Errorf("expected .syllago.yaml in %s, got error: %v", itemDir, err)
		}
	}
}

func TestImportHooksExclude(t *testing.T) {
	tmp := setupHooksProject(t)

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "hooks")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "false")
	importCmd.Flags().Set("dry-run", "false")
	importCmd.Flags().Set("scope", "project")
	importCmd.Flags().Set("force", "false")
	// Exclude the pre-bash-check hook by its derived name.
	importCmd.Flags().Set("exclude", "pre-bash-check")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --type hooks --exclude failed: %v", err)
	}

	hooksBase := filepath.Join(tmp, "local", "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected local/hooks/claude-code/ to exist, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 hook after --exclude, got %d", len(entries))
	}
	if entries[0].Name() == "pre-bash-check" {
		t.Errorf("excluded hook 'pre-bash-check' was still imported")
	}
}

func TestImportHooksForce(t *testing.T) {
	// Call runImportHooks directly to avoid cobra StringArray flag accumulation
	// across tests (pflag StringArray.Set appends rather than replaces).
	t.Run("skip without force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		_, _ = output.SetForTest(t)

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create one of the hook directories to simulate an existing item.
		existingDir := filepath.Join(tmp, "local", "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		stdout, _ := output.SetForTest(t)
		if err := runImportHooks(tmp, "claude-code", false, nil, false, "project", nil); err != nil {
			t.Fatalf("runImportHooks without force failed: %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "SKIP") {
			t.Errorf("expected SKIP message for existing hook, got: %s", out)
		}
		// The old content should still be there.
		data, _ := os.ReadFile(filepath.Join(existingDir, "hook.json"))
		if !strings.Contains(string(data), "old") {
			t.Errorf("expected existing hook.json to be unchanged without force")
		}
	})

	t.Run("overwrite with force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		_, _ = output.SetForTest(t)

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create the hook directory.
		existingDir := filepath.Join(tmp, "local", "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		_, _ = output.SetForTest(t)
		if err := runImportHooks(tmp, "claude-code", false, nil, true, "project", nil); err != nil {
			t.Fatalf("runImportHooks with force failed: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(existingDir, "hook.json"))
		if strings.Contains(string(data), `"event":"old"`) {
			t.Errorf("expected hook.json to be overwritten with force, still has old content")
		}
		// Verify new content has the correct event field.
		if !strings.Contains(string(data), "PreToolUse") {
			t.Errorf("expected overwritten hook.json to contain 'PreToolUse', got: %s", data)
		}
	})
}
