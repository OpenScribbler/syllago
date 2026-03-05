package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
)

// settingsJSON is a minimal claude-code settings.json with two hook groups.
const settingsJSON = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo pre-bash", "statusMessage": "pre-bash-check"}]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit",
        "hooks": [{"type": "command", "command": "echo post-edit", "statusMessage": "post-edit-check"}]
      }
    ]
  }
}`

// setupHooksProject creates a temp dir with a project-scoped .claude/settings.json.
// Returns the temp dir path.
func setupHooksProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0644)

	return tmp
}

func TestAddRequiresFrom(t *testing.T) {
	// Reset flags
	addCmd.Flags().Set("from", "")
	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Error("add without --from should fail")
	}
}

func TestAddUnknownProvider(t *testing.T) {
	addCmd.Flags().Set("from", "nonexistent")
	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Error("add with unknown provider should fail")
	}
}

// setupAddProject creates a temp dir with claude-code rule files for
// testing the add command. Returns the temp dir path.
func setupAddProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	rulesDir := filepath.Join(tmp, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "testing.md"), []byte("# Testing"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "logging.md"), []byte("# Logging"), 0644)

	return tmp
}

// runAddPreviewJSON runs the add command in --preview + JSON mode and
// returns the parsed DiscoveryReport.
func runAddPreviewJSON(t *testing.T, tmp string, nameFilter string) parse.DiscoveryReport {
	t.Helper()

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "")
	addCmd.Flags().Set("name", nameFilter)
	addCmd.Flags().Set("preview", "true")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --preview failed: %v", err)
	}

	var report parse.DiscoveryReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	return report
}

func TestAddNameFilterMatchesSubstring(t *testing.T) {
	tmp := setupAddProject(t)

	report := runAddPreviewJSON(t, tmp, "secur")

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 file matching 'secur', got %d", len(report.Files))
	}
	if filepath.Base(report.Files[0].Path) != "security.md" {
		t.Errorf("expected security.md, got %s", report.Files[0].Path)
	}
}

func TestAddNameFilterCaseInsensitive(t *testing.T) {
	tmp := setupAddProject(t)

	report := runAddPreviewJSON(t, tmp, "TESTING")

	if len(report.Files) != 1 {
		t.Fatalf("expected 1 file matching 'TESTING', got %d", len(report.Files))
	}
	if filepath.Base(report.Files[0].Path) != "testing.md" {
		t.Errorf("expected testing.md, got %s", report.Files[0].Path)
	}
}

func TestAddNameFilterUpdatesCountsCorrectly(t *testing.T) {
	tmp := setupAddProject(t)

	report := runAddPreviewJSON(t, tmp, "logging")

	total := 0
	for _, c := range report.Counts {
		total += c
	}
	if total != 1 {
		t.Errorf("expected total count of 1 after filtering, got %d (counts: %v)", total, report.Counts)
	}
}

func TestAddNoNameFilterReturnsAll(t *testing.T) {
	tmp := setupAddProject(t)

	report := runAddPreviewJSON(t, tmp, "")

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

func TestAddNameFilterNoMatch(t *testing.T) {
	tmp := setupAddProject(t)

	report := runAddPreviewJSON(t, tmp, "nonexistent-xyz")

	if len(report.Files) != 0 {
		t.Errorf("expected 0 files for non-matching name, got %d", len(report.Files))
	}
}

func TestAddWritesToGlobalDir(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "rules")
	addCmd.Flags().Set("name", "security")
	addCmd.Flags().Set("preview", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Verify the file was written to <globalDir>/rules/claude-code/security/
	// (claude-code rules are provider-specific, not universal)
	itemDir := filepath.Join(globalDir, "rules", "claude-code", "security")
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

func TestAddDryRunDoesNotWrite(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "rules")
	addCmd.Flags().Set("name", "")
	addCmd.Flags().Set("preview", "false")
	addCmd.Flags().Set("dry-run", "true")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --dry-run failed: %v", err)
	}

	// Verify that globalDir is empty (nothing was written)
	entries, err := os.ReadDir(globalDir)
	if err != nil {
		t.Fatalf("could not read globalDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected global dir to be empty during --dry-run, found %d entries", len(entries))
	}

	// Verify output mentions dry-run
	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
	}
}

func TestAddHooksPreview(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "hooks")
	addCmd.Flags().Set("name", "")
	addCmd.Flags().Set("preview", "true")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	// Reset exclude to empty.
	addCmd.Flags().Set("exclude", "")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --type hooks --preview failed: %v", err)
	}

	// No files should be written.
	entries, err := os.ReadDir(globalDir)
	if err != nil {
		t.Fatalf("could not read globalDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected global dir to be empty during --preview, found %d entries", len(entries))
	}

	// Output should list the hook names and mention "would be added".
	out := stdout.String()
	if !strings.Contains(out, "would be added") {
		t.Errorf("expected 'would be added' in preview output, got: %s", out)
	}
	// Both hooks should be listed.
	if !strings.Contains(out, "pre-bash-check") {
		t.Errorf("expected 'pre-bash-check' hook in preview output, got: %s", out)
	}
	if !strings.Contains(out, "post-edit-check") {
		t.Errorf("expected 'post-edit-check' hook in preview output, got: %s", out)
	}
}

func TestAddHooksWritesToGlobalDir(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "hooks")
	addCmd.Flags().Set("name", "")
	addCmd.Flags().Set("preview", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --type hooks failed: %v", err)
	}

	// Both hooks should have been written to <globalDir>/hooks/claude-code/<name>/.
	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist, got error: %v", err)
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

func TestAddHooksExclude(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "hooks")
	addCmd.Flags().Set("name", "")
	addCmd.Flags().Set("preview", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	// Exclude the pre-bash-check hook by its derived name.
	addCmd.Flags().Set("exclude", "pre-bash-check")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --type hooks --exclude failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 hook after --exclude, got %d", len(entries))
	}
	if entries[0].Name() == "pre-bash-check" {
		t.Errorf("excluded hook 'pre-bash-check' was still added")
	}
}

func TestAddHooksForce(t *testing.T) {
	// Call runAddHooks directly to avoid cobra StringArray flag accumulation
	// across tests (pflag StringArray.Set appends rather than replaces).
	t.Run("skip without force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		globalDir := t.TempDir()

		original := catalog.GlobalContentDirOverride
		catalog.GlobalContentDirOverride = globalDir
		t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

		_, _ = output.SetForTest(t)

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create one of the hook directories to simulate an existing item.
		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		stdout, _ := output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, false, "project", nil); err != nil {
			t.Fatalf("runAddHooks without force failed: %v", err)
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
		globalDir := t.TempDir()

		original := catalog.GlobalContentDirOverride
		catalog.GlobalContentDirOverride = globalDir
		t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

		_, _ = output.SetForTest(t)

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create the hook directory.
		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		_, _ = output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, true, "project", nil); err != nil {
			t.Fatalf("runAddHooks with force failed: %v", err)
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

func TestAddWritesMetadata(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("type", "rules")
	addCmd.Flags().Set("name", "security")
	addCmd.Flags().Set("preview", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	destDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	metaPath := filepath.Join(destDir, metadata.FileName)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("expected .syllago.yaml to be written after add")
	}
	m, err := metadata.Load(destDir)
	if err != nil || m == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if m.SourceProvider != "claude-code" {
		t.Errorf("expected source_provider=claude-code, got %q", m.SourceProvider)
	}
	if m.AddedAt == nil {
		t.Error("expected added_at to be set")
	}
	if m.SourceType != "provider" {
		t.Errorf("expected source_type=provider, got %q", m.SourceType)
	}
}

func TestAddPreservesSourceForNonCanonicalFormat(t *testing.T) {
	tmp := t.TempDir()
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	// Create a fake .mdc file (Cursor format) in a provider-like structure.
	rulesDir := filepath.Join(tmp, ".cursor", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "my-rule.mdc"), []byte("# My Rule\ncontent"), 0644)

	// Simulate a DiscoveredFile with .mdc extension.
	file := parse.DiscoveredFile{
		Path:        filepath.Join(rulesDir, "my-rule.mdc"),
		ContentType: catalog.Rules,
	}

	dest, err := writeAddedContent(file, "cursor", false, true, true)
	if err != nil {
		t.Fatalf("writeAddedContent failed: %v", err)
	}

	// The canonical file should exist (cursor .mdc is converted to canonical .md).
	if _, err := os.Stat(filepath.Join(dest, "rule.md")); err != nil {
		t.Errorf("expected canonical content file rule.md, got error: %v", err)
	}

	// .source/ should exist with the original.
	sourceFile := filepath.Join(dest, ".source", "my-rule.mdc")
	if _, err := os.Stat(sourceFile); err != nil {
		t.Errorf("expected .source/my-rule.mdc to exist, got error: %v", err)
	}
}
