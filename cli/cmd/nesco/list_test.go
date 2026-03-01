package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/output"
)

// setupListRepo creates a temp nesco repo with items across types and sources.
func setupListRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create the skills/ marker so findContentRepoRoot resolves.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)

	// Shared skill.
	sharedSkill := filepath.Join(root, "skills", "code-review")
	os.MkdirAll(sharedSkill, 0755)
	os.WriteFile(filepath.Join(sharedSkill, "SKILL.md"), []byte("---\nname: Code Review\ndescription: Systematic code review\n---\n"), 0644)
	os.WriteFile(filepath.Join(sharedSkill, "README.md"), []byte("# code-review\n"), 0644)

	// Local skill.
	localSkill := filepath.Join(root, "local", "skills", "greeting")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("---\nname: Greeting\ndescription: Says hello to the user\n---\n"), 0644)

	// Local agent.
	localAgent := filepath.Join(root, "local", "agents", "code-reviewer")
	os.MkdirAll(localAgent, 0755)
	os.WriteFile(filepath.Join(localAgent, "AGENT.md"), []byte("---\nname: Code Reviewer\ndescription: Code review agent\n---\n"), 0644)
	os.WriteFile(filepath.Join(localAgent, "README.md"), []byte("# code-reviewer\n"), 0644)

	return root
}

func TestListShowsAllItems(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	out := stdout.String()

	// Should contain both skills.
	if !strings.Contains(out, "code-review") {
		t.Errorf("expected shared skill 'code-review' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected local skill 'greeting' in output, got:\n%s", out)
	}

	// Should contain the agent.
	if !strings.Contains(out, "code-reviewer") {
		t.Errorf("expected local agent 'code-reviewer' in output, got:\n%s", out)
	}

	// Should show type headers with counts.
	if !strings.Contains(out, "Skills (2)") {
		t.Errorf("expected 'Skills (2)' header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Agents (1)") {
		t.Errorf("expected 'Agents (1)' header in output, got:\n%s", out)
	}

	// Should show source labels.
	if !strings.Contains(out, "[local") {
		t.Errorf("expected '[local' source label in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[shared") {
		t.Errorf("expected '[shared' source label in output, got:\n%s", out)
	}
}

func TestListFilterByType(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)

	listCmd.Flags().Set("type", "skills")
	defer listCmd.Flags().Set("type", "")

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --type skills failed: %v", err)
	}

	out := stdout.String()

	// Should contain skills.
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected 'greeting' in skills-filtered output, got:\n%s", out)
	}
	if !strings.Contains(out, "code-review") {
		t.Errorf("expected 'code-review' in skills-filtered output, got:\n%s", out)
	}

	// Should NOT contain agents.
	if strings.Contains(out, "code-reviewer") {
		t.Errorf("type=skills filter should exclude agents, got:\n%s", out)
	}
	if strings.Contains(out, "Agents") {
		t.Errorf("type=skills filter should not show Agents header, got:\n%s", out)
	}
}

func TestListFilterBySource(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)

	listCmd.Flags().Set("source", "local")
	defer listCmd.Flags().Set("source", "all")

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --source local failed: %v", err)
	}

	out := stdout.String()

	// Should contain local items.
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected local 'greeting' in output, got:\n%s", out)
	}

	// Should NOT contain shared items.
	if strings.Contains(out, "code-review") && !strings.Contains(out, "code-reviewer") {
		// code-review is shared, code-reviewer is local agent.
		// We need a tighter check: "code-review" without "code-reviewer" prefix.
	}
	// Check that the shared skill specifically is absent.
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "code-review ") && strings.Contains(line, "shared") {
			t.Errorf("source=local should exclude shared 'code-review', got line: %s", line)
		}
	}
}

func TestListJSON(t *testing.T) {
	root := setupListRepo(t)
	withFakeRepoRoot(t, root)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	var result listResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, stdout.String())
	}

	if len(result.Groups) == 0 {
		t.Fatal("expected at least one group in JSON output")
	}

	// Verify skills group exists with correct count.
	found := false
	for _, g := range result.Groups {
		if g.Type == "Skills" {
			found = true
			if g.Count != 2 {
				t.Errorf("expected 2 skills, got %d", g.Count)
			}
		}
	}
	if !found {
		t.Error("expected Skills group in JSON output")
	}
}

func TestListEmpty(t *testing.T) {
	root := t.TempDir()
	// Create a minimal marker so the root resolves.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)
	withFakeRepoRoot(t, root)

	_, stderr := output.SetForTest(t)

	err := listCmd.RunE(listCmd, []string{})
	if err != nil {
		t.Fatalf("list on empty repo failed: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "No items found") {
		t.Errorf("expected 'No items found' message, got stderr:\n%s", errOut)
	}
}
