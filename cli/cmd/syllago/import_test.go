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
