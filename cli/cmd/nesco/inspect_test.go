package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
)

// setupInspectRepo creates a temp nesco repo with a skill that has files and
// content referencing "Bash" to trigger a risk indicator.
func setupInspectRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create the skills/ marker.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)

	// Create a skill in local/.
	skillDir := filepath.Join(root, "local", "skills", "my-skill")
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
	if !strings.Contains(out, "Source:  local") {
		t.Errorf("expected 'Source:  local' in output, got:\n%s", out)
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
	if result.Source != "local" {
		t.Errorf("source = %q, want %q", result.Source, "local")
	}
	if len(result.Risks) == 0 {
		t.Error("expected at least one risk indicator in JSON output")
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

func TestInspectProviderSpecific(t *testing.T) {
	root := t.TempDir()

	// Create marker and provider-specific rule.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	ruleDir := filepath.Join(root, "local", "rules", "claude-code", "my-rule")
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
