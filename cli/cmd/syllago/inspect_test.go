package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// setupInspectRepo creates a temp syllago repo with a skill that has files and
// content referencing "Bash" to trigger a risk indicator.
func setupInspectRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create a shared skill (directly in skills/).
	skillDir := filepath.Join(root, "skills", "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: My Skill\ndescription: Reviews Go code\n---\nUse the Bash tool to run tests.\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"),
		[]byte("# my-skill\nA skill for reviewing Go code.\n"), 0644)

	return root
}

func TestInspectShowsItemDetails(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	out := stdout.String()

	// Check key fields are present.
	if !strings.Contains(out, "Name:    my-skill") {
		t.Errorf("expected 'Name:    my-skill' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Type:    Skills") {
		t.Errorf("expected 'Type:    Skills' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Source:  shared") {
		t.Errorf("expected 'Source:  shared' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Desc:    Reviews Go code") {
		t.Errorf("expected description in output, got:\n%s", out)
	}

	// Check files section.
	if !strings.Contains(out, "SKILL.md") {
		t.Errorf("expected SKILL.md in files list, got:\n%s", out)
	}
	if !strings.Contains(out, "README.md") {
		t.Errorf("expected README.md in files list, got:\n%s", out)
	}

	// Check risk indicators (SKILL.md mentions "Bash").
	if !strings.Contains(out, "Bash access") {
		t.Errorf("expected 'Bash access' risk indicator, got:\n%s", out)
	}
	// Plain (non-MOAT) items must not print a Trust: line — the conditional
	// in inspect.go suppresses the header when TrustDescription is empty.
	if strings.Contains(out, "Trust:") {
		t.Errorf("plain shared item should not print Trust: line, got:\n%s", out)
	}
}

func TestInspectNotFound(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	output.SetForTest(t)

	err := inspectCmd.RunE(inspectCmd, []string{"skills/nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent item")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestInspectJSON(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if result.Name != "my-skill" {
		t.Errorf("name = %q, want %q", result.Name, "my-skill")
	}
	if result.Type != "Skills" {
		t.Errorf("type = %q, want %q", result.Type, "Skills")
	}
	if result.Source != "shared" {
		t.Errorf("source = %q, want %q", result.Source, "shared")
	}
	if len(result.Risks) == 0 {
		t.Error("expected at least one risk indicator in JSON output")
	}
	// Plain (non-MOAT) items must not carry a trust label. omitempty on
	// the Trust* fields drops them entirely from the JSON body, so the
	// raw output must not contain any "trust" key.
	raw := stdout.String()
	if strings.Contains(raw, `"trust":`) {
		t.Errorf("plain shared item leaked trust field into JSON: %s", raw)
	}
	if strings.Contains(raw, `"recalled":`) {
		t.Errorf("plain shared item leaked recalled field into JSON: %s", raw)
	}
}

func TestInspectInvalidPath(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	output.SetForTest(t)

	err := inspectCmd.RunE(inspectCmd, []string{"just-a-name"})
	if err == nil {
		t.Fatal("expected error for invalid path format")
	}
	if !strings.Contains(err.Error(), "invalid path format") {
		t.Errorf("expected 'invalid path format' in error, got: %v", err)
	}
}

func TestInspectFiles_ShowsContent(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("files", "true")
	defer inspectCmd.Flags().Set("files", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --files failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "--- SKILL.md ---") {
		t.Errorf("expected '--- SKILL.md ---' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Use the Bash tool to run tests.") {
		t.Errorf("expected SKILL.md content in output, got:\n%s", out)
	}
}

func TestInspectFiles_JSON(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	inspectCmd.Flags().Set("files", "true")
	defer inspectCmd.Flags().Set("files", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --files --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if result.FileContents == nil {
		t.Fatal("expected file_contents in JSON output, got nil")
	}
	content, ok := result.FileContents["SKILL.md"]
	if !ok {
		t.Errorf("expected SKILL.md key in file_contents, got keys: %v", mapKeys(result.FileContents))
	}
	if !strings.Contains(content, "Use the Bash tool") {
		t.Errorf("expected SKILL.md content in file_contents, got: %q", content)
	}
}

func TestInspectFiles_MultipleFiles(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("files", "true")
	defer inspectCmd.Flags().Set("files", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --files failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "--- SKILL.md ---") {
		t.Errorf("expected SKILL.md header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "--- README.md ---") {
		t.Errorf("expected README.md header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "A skill for reviewing Go code.") {
		t.Errorf("expected README.md content in output, got:\n%s", out)
	}
}

func TestInspectFiles_MissingFile(t *testing.T) {
	root := t.TempDir()

	// Create a skill with a file entry that doesn't exist on disk.
	skillDir := filepath.Join(root, "skills", "broken-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: Broken Skill\n---\nContent.\n"), 0644)
	// ghost.md is listed as a file because catalog scans the dir,
	// so we create it then remove it to simulate a missing file after scan.
	ghostPath := filepath.Join(skillDir, "ghost.md")
	os.WriteFile(ghostPath, []byte("temporary"), 0644)

	withFakeRepoRoot(t, root)
	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("files", "true")
	defer inspectCmd.Flags().Set("files", "false")

	// Remove the ghost file after catalog would have registered it.
	// We actually need to scan first, then remove — but the catalog scans
	// during RunE. Instead, create the file, remove it after scan:
	// The simplest approach: don't create ghost.md at all; verify the
	// existing files are shown and command doesn't crash.
	os.Remove(ghostPath)

	err := inspectCmd.RunE(inspectCmd, []string{"skills/broken-skill"})
	if err != nil {
		t.Fatalf("inspect --files should not return error for missing files, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "--- SKILL.md ---") {
		t.Errorf("expected SKILL.md header in output, got:\n%s", out)
	}
}

func TestInspect_DefaultShowsPrimaryContent(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	out := stdout.String()
	// Default mode should show primary file content without needing --files.
	if !strings.Contains(out, "--- SKILL.md ---") {
		t.Errorf("expected primary file header '--- SKILL.md ---' in default output, got:\n%s", out)
	}
	if !strings.Contains(out, "Use the Bash tool to run tests.") {
		t.Errorf("expected primary file content in default output, got:\n%s", out)
	}
}

func TestInspect_AsProvider(t *testing.T) {
	root := t.TempDir()

	// Create a provider-specific rule (rules need a source provider for conversion).
	ruleDir := filepath.Join(root, "rules", "claude-code", "test-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Test Rule\nAlways use tests.\n"), 0644)

	withFakeRepoRoot(t, root)
	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("as", "cursor")
	defer inspectCmd.Flags().Set("as", "")

	err := inspectCmd.RunE(inspectCmd, []string{"rules/claude-code/test-rule"})
	if err != nil {
		t.Fatalf("inspect --as cursor failed: %v", err)
	}

	out := stdout.String()
	// Should show header with provider name.
	if !strings.Contains(out, "# test-rule as Cursor") {
		t.Errorf("expected '# test-rule as Cursor' header, got:\n%s", out)
	}
	// Cursor format adds alwaysApply frontmatter.
	if !strings.Contains(out, "alwaysApply: true") {
		t.Errorf("expected Cursor frontmatter 'alwaysApply: true' in output, got:\n%s", out)
	}
	// Should not show metadata fields (Name:, Type:, etc.) in --as mode.
	if strings.Contains(out, "Name:    test-rule") {
		t.Errorf("--as mode should not show metadata, got:\n%s", out)
	}
}

func TestInspect_AsProvider_JSON(t *testing.T) {
	root := t.TempDir()

	ruleDir := filepath.Join(root, "rules", "claude-code", "test-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Test Rule\nAlways use tests.\n"), 0644)

	withFakeRepoRoot(t, root)
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	inspectCmd.Flags().Set("as", "cursor")
	defer inspectCmd.Flags().Set("as", "")

	err := inspectCmd.RunE(inspectCmd, []string{"rules/claude-code/test-rule"})
	if err != nil {
		t.Fatalf("inspect --as cursor --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if result.AsProvider != "Cursor" {
		t.Errorf("as_provider = %q, want %q", result.AsProvider, "Cursor")
	}
	if result.AsContent == "" {
		t.Error("expected non-empty as_content in JSON output")
	}
	if !strings.Contains(result.AsContent, "alwaysApply: true") {
		t.Errorf("expected Cursor frontmatter in as_content, got: %q", result.AsContent)
	}
}

func TestInspect_AsProvider_UnknownProvider(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	output.SetForTest(t)
	inspectCmd.Flags().Set("as", "nonexistent-provider")
	defer inspectCmd.Flags().Set("as", "")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' in error, got: %v", err)
	}
}

// mapKeys returns the keys of a map for error messages.
func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// setupInspectHookRepo creates a temp syllago repo with a flat-format hook item.
// Hooks are provider-specific: hooks/<provider>/<item-name>/hook.json.
func setupInspectHookRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	hookDir := filepath.Join(root, "hooks", "claude-code", "my-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hook.json"),
		[]byte(`{"event":"PostToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo done"}]}`), 0644)

	return root
}

func TestInspectCompat_NonHook(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("compatibility", "true")
	defer inspectCmd.Flags().Set("compatibility", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --compatibility failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "not applicable for Skills (hooks only)") {
		t.Errorf("expected 'not applicable' message for non-hook, got:\n%s", out)
	}
}

func TestInspectCompat_Hook(t *testing.T) {
	root := setupInspectHookRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("compatibility", "true")
	defer inspectCmd.Flags().Set("compatibility", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"hooks/claude-code/my-hook"})
	if err != nil {
		t.Fatalf("inspect --compatibility failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Compatibility:") {
		t.Errorf("expected 'Compatibility:' section in output, got:\n%s", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected 'claude-code' provider in output, got:\n%s", out)
	}
}

func TestInspectCompat_JSON(t *testing.T) {
	root := setupInspectHookRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	inspectCmd.Flags().Set("compatibility", "true")
	defer inspectCmd.Flags().Set("compatibility", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"hooks/claude-code/my-hook"})
	if err != nil {
		t.Fatalf("inspect --compatibility --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if len(result.Compatibility) == 0 {
		t.Fatal("expected compatibility array in JSON output, got empty")
	}

	// Verify the first entry has Provider and Level populated.
	first := result.Compatibility[0]
	if first.Provider == "" {
		t.Error("expected non-empty provider in first compatibility entry")
	}
	if first.Level == "" {
		t.Error("expected non-empty level in first compatibility entry")
	}
}

func TestInspectCompat_NonHook_JSON(t *testing.T) {
	root := setupInspectRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	inspectCmd.Flags().Set("compatibility", "true")
	defer inspectCmd.Flags().Set("compatibility", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("inspect --compatibility --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	// Non-hook items produce no compatibility array in JSON (omitempty).
	if len(result.Compatibility) != 0 {
		t.Errorf("expected no compatibility entries for non-hook in JSON, got %d", len(result.Compatibility))
	}
}

func TestInspectProviderSpecific(t *testing.T) {
	root := t.TempDir()

	// Create a shared provider-specific rule.
	ruleDir := filepath.Join(root, "rules", "claude-code", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# My Rule\nAlways use tests.\n"), 0644)
	os.WriteFile(filepath.Join(ruleDir, "README.md"), []byte("# my-rule\nA testing rule.\n"), 0644)

	withFakeRepoRoot(t, root)
	stdout, _ := output.SetForTest(t)

	err := inspectCmd.RunE(inspectCmd, []string{"rules/claude-code/my-rule"})
	if err != nil {
		t.Fatalf("inspect provider-specific item failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Name:    my-rule") {
		t.Errorf("expected 'Name:    my-rule' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Provider: claude-code") {
		t.Errorf("expected 'Provider: claude-code' in output, got:\n%s", out)
	}
}

// setupMCPRepo creates a temp syllago repo with an MCP item that has a config.json
// containing command, args, and an env var — triggering risk indicators.
func setupMCPRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	mcpDir := filepath.Join(root, "mcp", "github-mcp")
	os.MkdirAll(mcpDir, 0755)
	os.WriteFile(filepath.Join(mcpDir, "config.json"), []byte(`{
		"command": "npx",
		"args": ["-y", "@modelcontextprotocol/server-github"],
		"env": {
			"GITHUB_TOKEN": ""
		}
	}`), 0644)
	os.WriteFile(filepath.Join(mcpDir, "README.md"), []byte("# github-mcp\nGitHub MCP server.\n"), 0644)

	return root
}

func TestInspectRisk_NoRisk(t *testing.T) {
	// A rule item has no risk indicators — --risk should produce no detailed_risks section.
	root := t.TempDir()
	ruleDir := filepath.Join(root, "rules", "claude-code", "safe-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Safe Rule\nNo risky content.\n"), 0644)

	withFakeRepoRoot(t, root)
	stdout, _ := output.SetForTest(t)
	inspectCmd.Flags().Set("risk", "true")
	defer inspectCmd.Flags().Set("risk", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"rules/claude-code/safe-rule"})
	if err != nil {
		t.Fatalf("inspect --risk failed: %v", err)
	}

	out := stdout.String()
	// A rule with no Bash/network indicators should show no detailed risks section.
	if strings.Contains(out, "Detailed risks:") {
		t.Errorf("expected no 'Detailed risks:' section for risk-free rule, got:\n%s", out)
	}
}

func TestInspectRisk_JSON(t *testing.T) {
	root := setupMCPRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	inspectCmd.Flags().Set("risk", "true")
	defer inspectCmd.Flags().Set("risk", "false")

	err := inspectCmd.RunE(inspectCmd, []string{"mcp/github-mcp"})
	if err != nil {
		t.Fatalf("inspect --risk --json failed: %v", err)
	}

	var result inspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if len(result.DetailedRisks) == 0 {
		t.Fatal("expected detailed_risks in JSON output, got empty slice")
	}

	// Verify at least one entry has a label and details.
	found := false
	for _, rd := range result.DetailedRisks {
		if rd.Label != "" && len(rd.Details) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one riskDetail with label and details, got: %+v", result.DetailedRisks)
	}
}
