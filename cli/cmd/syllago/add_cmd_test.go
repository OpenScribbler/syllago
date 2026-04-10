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
func setupHooksProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0644)
	return tmp
}

// setupAddProject creates a temp dir with claude-code rule files.
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

// runDiscoveryJSON runs "syllago add --from <provider> --json" (discovery mode,
// no positional target) and returns the parsed JSON output.
func runDiscoveryJSON(t *testing.T, tmp string) map[string]interface{} {
	t.Helper()
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	return result
}

func TestAddRequiresFrom(t *testing.T) {
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

func TestAddAllAndPositionalIsError(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() { addCmd.Flags().Set("all", "false") })

	err := addCmd.RunE(addCmd, []string{"rules"})
	if err == nil {
		t.Error("specifying both --all and a positional target should fail")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected error message to mention --all, got: %v", err)
	}
}

// TestAddDiscoveryMode verifies that no-arg invocation returns JSON with a
// "provider" field and "groups" array, without writing any files.
func TestAddDiscoveryMode(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	result := runDiscoveryJSON(t, tmp)

	if result["provider"] != "claude-code" {
		t.Errorf("expected provider=claude-code in JSON, got: %v", result["provider"])
	}

	// No files should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("discovery mode should not write anything, found %d entries", len(entries))
	}
}

// TestAddDiscoveryJSONGroups verifies that discovered rules appear in the
// groups array with status annotations.
func TestAddDiscoveryJSONGroups(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	result := runDiscoveryJSON(t, tmp)

	groups, ok := result["groups"].([]interface{})
	if !ok || len(groups) == 0 {
		t.Fatalf("expected non-empty groups array, got: %v", result["groups"])
	}

	// Find the rules group.
	var rulesGroup map[string]interface{}
	for _, g := range groups {
		gm := g.(map[string]interface{})
		if gm["type"] == "rules" {
			rulesGroup = gm
			break
		}
	}
	if rulesGroup == nil {
		t.Fatalf("expected a rules group in JSON output")
	}
	items, _ := rulesGroup["items"].([]interface{})
	if len(items) < 3 {
		t.Errorf("expected at least 3 items in rules group, got %d", len(items))
	}
}

// TestAddByType verifies "syllago add rules --from claude-code" writes files.
func TestAddByType(t *testing.T) {
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules"}); err != nil {
		t.Fatalf("add rules failed: %v", err)
	}

	// All 3 rules should have been written.
	for _, name := range []string{"security", "testing", "logging"} {
		itemDir := filepath.Join(globalDir, "rules", "claude-code", name)
		if _, err := os.Stat(itemDir); err != nil {
			t.Errorf("expected %s item dir at %s, got: %v", name, itemDir, err)
		}
	}
}

// TestAddSpecificItem verifies "syllago add rules/security --from claude-code".
func TestAddSpecificItem(t *testing.T) {
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add rules/security failed: %v", err)
	}

	// Only security should be written.
	itemDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("expected security item dir at %s, got: %v", itemDir, err)
	}

	// Other rules should not exist.
	for _, name := range []string{"testing", "logging"} {
		otherDir := filepath.Join(globalDir, "rules", "claude-code", name)
		if _, err := os.Stat(otherDir); err == nil {
			t.Errorf("expected %s to NOT be written, but it was", name)
		}
	}
}

// TestAddItemNotFound verifies that "syllago add rules/nonexistent" returns an error.
func TestAddItemNotFound(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	err := addCmd.RunE(addCmd, []string{"rules/nonexistent-xyz"})
	if err == nil {
		t.Error("expected error for nonexistent item, got nil")
	}
}

// TestAddUnknownType verifies that "syllago add widgets" returns an error.
func TestAddUnknownType(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	err := addCmd.RunE(addCmd, []string{"widgets"})
	if err == nil {
		t.Error("expected error for unknown content type")
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "true")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { addCmd.Flags().Set("dry-run", "false") })

	if err := addCmd.RunE(addCmd, []string{"rules"}); err != nil {
		t.Fatalf("add --dry-run failed: %v", err)
	}

	entries, err := os.ReadDir(globalDir)
	if err != nil {
		t.Fatalf("could not read globalDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected global dir to be empty during --dry-run, found %d entries", len(entries))
	}

	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	itemDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("expected item directory at %s, got error: %v", itemDir, err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
		t.Errorf("expected metadata at %s: %v", itemDir, err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, "rule.md")); err != nil {
		t.Errorf("expected rule.md at %s: %v", itemDir, err)
	}
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	destDir := filepath.Join(globalDir, "rules", "claude-code", "security")
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
	if m.SourceHash == "" {
		t.Error("expected source_hash to be set")
	}
}

func TestAddHooksDiscovery(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	// No files should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("discovery mode should not write anything, found %d entries", len(entries))
	}

	// JSON output should mention hooks.
	out := stdout.String()
	if !strings.Contains(strings.ToLower(out), "hook") {
		t.Errorf("expected hooks to appear in discovery JSON, got: %s", out)
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "")

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist, got error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 hook directories, got %d", len(entries))
	}

	for _, entry := range entries {
		itemDir := filepath.Join(hooksBase, entry.Name())
		if _, err := os.Stat(filepath.Join(itemDir, "hook.json")); err != nil {
			t.Errorf("expected hook.json in %s: %v", itemDir, err)
		}
		if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
			t.Errorf("expected .syllago.yaml in %s: %v", itemDir, err)
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "pre-bash-check")

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks --exclude failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 hook after --exclude, got %d", len(entries))
	}
	if entries[0].Name() == "pre-bash-check" {
		t.Errorf("excluded hook 'pre-bash-check' was still added")
	}
}

func TestAddHooksForce(t *testing.T) {
	t.Run("skip without force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		globalDir := t.TempDir()

		original := catalog.GlobalContentDirOverride
		catalog.GlobalContentDirOverride = globalDir
		t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create one hook directory.
		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		stdout, _ := output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, false, "project", nil, "", "", ""); err != nil {
			t.Fatalf("runAddHooks without force failed: %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "SKIP") {
			t.Errorf("expected SKIP message for existing hook, got: %s", out)
		}
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

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		_, _ = output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, true, "project", nil, "", "", ""); err != nil {
			t.Fatalf("runAddHooks with force failed: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(existingDir, "hook.json"))
		if strings.Contains(string(data), `"event":"old"`) {
			t.Errorf("expected hook.json to be overwritten with force, still has old content")
		}
		if !strings.Contains(string(data), "before_tool_execute") {
			t.Errorf("expected overwritten hook.json to contain 'before_tool_execute', got: %s", data)
		}
	})
}

func TestAddWarnsWhenProviderNotDetected(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	// Add an undetected provider with discovery paths pointing to tmp.
	installBase := t.TempDir()
	addTestProviderOpts(t, "undetected-add", "Undetected Add Provider", installBase, false)

	_, stderr := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "undetected-add")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")

	// Discovery mode (no args, no --all) — should still warn.
	_ = addCmd.RunE(addCmd, []string{})

	errOut := stderr.String()
	if !strings.Contains(errOut, "Warning: Undetected Add Provider not detected") {
		t.Errorf("expected provider-not-detected warning on stderr, got: %s", errOut)
	}
	if !strings.Contains(errOut, "syllago config paths --provider undetected-add") {
		t.Errorf("expected config paths hint in warning, got: %s", errOut)
	}
}

func TestAddNoWarningWhenProviderDetected(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	installBase := t.TempDir()
	addTestProviderOpts(t, "detected-add", "Detected Add Provider", installBase, true)

	_, stderr := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "detected-add")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")

	_ = addCmd.RunE(addCmd, []string{})

	errOut := stderr.String()
	if strings.Contains(errOut, "Warning") {
		t.Errorf("expected no warning for detected provider, got: %s", errOut)
	}
}

func TestAddNoWarningInJSONMode(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	installBase := t.TempDir()
	addTestProviderOpts(t, "undetected-add-json", "Undetected JSON", installBase, false)

	_, stderr := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "undetected-add-json")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")

	_ = addCmd.RunE(addCmd, []string{})

	errOut := stderr.String()
	if strings.Contains(errOut, "Warning") {
		t.Errorf("expected no warning in JSON mode, got: %s", errOut)
	}
}

// settingsWithMCP is a settings.json that contains both hooks and MCP configs.
const settingsWithMCP = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo pre-bash"}]
      }
    ]
  },
  "mcpServers": {
    "obsidian": {
      "command": "npx",
      "args": ["-y", "obsidian-mcp"],
      "env": {"VAULT_PATH": "/path/to/vault"}
    }
  }
}`

// setupMcpProject creates a temp dir with a project-scoped .claude/settings.json
// containing MCP configs.
func setupMcpProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsWithMCP), 0644)
	return tmp
}

func TestAddMcpDiscovery(t *testing.T) {
	tmp := setupMcpProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "obsidian") {
		t.Errorf("expected MCP server 'obsidian' in discovery output, got: %s", out)
	}
	if !strings.Contains(out, `"mcp"`) {
		t.Errorf("expected 'mcp' type group in JSON output, got: %s", out)
	}

	// No files should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("discovery should not write files, found %d entries", len(entries))
	}
}

func TestAddMcpWritesToGlobalDir(t *testing.T) {
	tmp := setupMcpProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp failed: %v", err)
	}

	mcpBase := filepath.Join(globalDir, "mcp", "claude-code")
	entries, err := os.ReadDir(mcpBase)
	if err != nil {
		t.Fatalf("expected global mcp dir, got error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 MCP server directory, got %d", len(entries))
	}
	if entries[0].Name() != "obsidian" {
		t.Errorf("expected 'obsidian' directory, got %q", entries[0].Name())
	}

	itemDir := filepath.Join(mcpBase, "obsidian")
	if _, err := os.Stat(filepath.Join(itemDir, "config.json")); err != nil {
		t.Errorf("expected config.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
		t.Errorf("expected .syllago.yaml: %v", err)
	}

	// Verify config.json has the nested format.
	data, _ := os.ReadFile(filepath.Join(itemDir, "config.json"))
	if !strings.Contains(string(data), `"mcpServers"`) {
		t.Errorf("expected nested mcpServers wrapper in config.json, got: %s", data)
	}
	if !strings.Contains(string(data), `"obsidian"`) {
		t.Errorf("expected server name in config.json, got: %s", data)
	}
}

func TestAddMcpScopeMetadata(t *testing.T) {
	tmp := setupMcpProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp failed: %v", err)
	}

	destDir := filepath.Join(globalDir, "mcp", "claude-code", "obsidian")
	m, err := metadata.Load(destDir)
	if err != nil || m == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if m.SourceProvider != "claude-code" {
		t.Errorf("expected source_provider=claude-code, got %q", m.SourceProvider)
	}
	if m.SourceScope != "project" {
		t.Errorf("expected source_scope=project, got %q", m.SourceScope)
	}
	if m.SourceProject == "" {
		t.Error("expected source_project to be set for project-scoped item")
	}
	if m.Type != "mcp" {
		t.Errorf("expected type=mcp, got %q", m.Type)
	}
}

func TestAddMcpCollision(t *testing.T) {
	tmp := setupMcpProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	// Pre-create an "obsidian" MCP item with global scope metadata.
	existingDir := filepath.Join(globalDir, "mcp", "claude-code", "obsidian")
	os.MkdirAll(existingDir, 0755)
	os.WriteFile(filepath.Join(existingDir, "config.json"), []byte(`{"mcpServers":{"obsidian":{}}}`), 0644)
	existingMeta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           "obsidian",
		Type:           "mcp",
		SourceScope:    "global",
		SourceProvider: "claude-code",
	}
	metadata.Save(existingDir, existingMeta)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp (collision) failed: %v", err)
	}

	// Should have obsidian (global, existing) and obsidian-2 (project, new).
	mcpBase := filepath.Join(globalDir, "mcp", "claude-code")
	entries, _ := os.ReadDir(mcpBase)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 directories after collision (obsidian + obsidian-2), got %d: %v", len(entries), names)
	}

	// Verify the new item has project scope.
	newDir := filepath.Join(mcpBase, "obsidian-2")
	m, err := metadata.Load(newDir)
	if err != nil || m == nil {
		t.Fatalf("metadata load for collision item failed: %v", err)
	}
	if m.SourceScope != "project" {
		t.Errorf("expected collision item source_scope=project, got %q", m.SourceScope)
	}
}

func TestAddMcpDryRun(t *testing.T) {
	tmp := setupMcpProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "true")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	t.Cleanup(func() { addCmd.Flags().Set("dry-run", "false") })

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp --dry-run failed: %v", err)
	}

	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("dry-run should not write files, found %d", len(entries))
	}

	out := stdout.String()
	if !strings.Contains(out, "obsidian") {
		t.Errorf("expected 'obsidian' in dry-run output, got: %s", out)
	}
	if !strings.Contains(out, "would be added") {
		t.Errorf("expected 'would be added' in dry-run output, got: %s", out)
	}
}

func TestAddHooksScopeDefaultIsAll(t *testing.T) {
	// Verify the scope flag's default is "all" not "global".
	f := addCmd.Flags().Lookup("scope")
	if f == nil {
		t.Fatal("scope flag not found")
	}
	if f.DefValue != "all" {
		t.Errorf("expected default scope='all', got %q", f.DefValue)
	}
}

func TestAddHooksScopeMetadata(t *testing.T) {
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
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "")

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected hook directories, got error: %v, count: %d", err, len(entries))
	}

	// Check metadata of first hook has scope info.
	firstDir := filepath.Join(hooksBase, entries[0].Name())
	m, err := metadata.Load(firstDir)
	if err != nil || m == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if m.SourceScope != "project" {
		t.Errorf("expected source_scope=project, got %q", m.SourceScope)
	}
	if m.SourceProject == "" {
		t.Error("expected source_project to be set for project-scoped hook")
	}
}

func TestAddPreservesSourceForNonCanonicalFormat(t *testing.T) {
	tmp := t.TempDir()
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	rulesDir := filepath.Join(tmp, ".cursor", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "my-rule.mdc"), []byte("# My Rule\ncontent"), 0644)

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "cursor")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "true")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { addCmd.Flags().Set("force", "false") })

	if err := addCmd.RunE(addCmd, []string{"rules/my-rule"}); err != nil {
		t.Fatalf("add rules/my-rule failed: %v", err)
	}

	dest := filepath.Join(globalDir, "rules", "cursor", "my-rule")
	if _, err := os.Stat(filepath.Join(dest, "rule.md")); err != nil {
		t.Errorf("expected canonical rule.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".source", "my-rule.mdc")); err != nil {
		t.Errorf("expected .source/my-rule.mdc: %v", err)
	}
}

func TestAddHooks_DisplayNameFlag(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)
	if err := runAddHooks(tmp, "claude-code", false, nil, false, "project", nil, "", "", "My Custom Name"); err != nil {
		t.Fatalf("runAddHooks with displayName failed: %v", err)
	}

	// Find an added hook and verify its metadata has the display name
	entries, err := os.ReadDir(filepath.Join(globalDir, "hooks", "claude-code"))
	if err != nil {
		t.Fatalf("reading hooks dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one hook directory")
	}

	hookDir := filepath.Join(globalDir, "hooks", "claude-code", entries[0].Name())
	meta, err := metadata.Load(hookDir)
	if err != nil {
		t.Fatalf("loading metadata: %v", err)
	}
	if meta == nil {
		t.Fatal("expected metadata to exist")
	}
	if meta.Name != "My Custom Name" {
		t.Errorf("expected display name 'My Custom Name', got %q", meta.Name)
	}
}
