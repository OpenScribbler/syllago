package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestAnalyzer_EmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if !result.IsEmpty() {
		t.Errorf("expected empty result, got Auto=%d Confirm=%d", len(result.Auto), len(result.Confirm))
	}
}

func TestAnalyzer_SyllagoCanonical(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "skills/my-skill/SKILL.md",
		"---\nname: My Skill\ndescription: Does things\n---\nBody.\n")

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Skills at 0.95 confidence should land in Auto (threshold 0.80).
	if len(result.Auto) == 0 {
		t.Fatal("expected at least 1 Auto item")
	}

	found := false
	for _, item := range result.Auto {
		if item.Name == "my-skill" && item.Type == catalog.Skills {
			found = true
			if item.DisplayName != "My Skill" {
				t.Errorf("DisplayName = %q, want %q", item.DisplayName, "My Skill")
			}
		}
	}
	if !found {
		t.Errorf("expected my-skill in Auto, got %v", result.Auto)
	}
}

func TestAnalyzer_HooksAlwaysConfirm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a hook with high confidence — should still land in Confirm.
	setupFile(t, root, "hooks/claude-code/lint/hook.json", `{"event": "PostToolUse"}`)

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	// Hooks always go to Confirm regardless of confidence.
	for _, item := range result.Auto {
		if item.Type == catalog.Hooks {
			t.Errorf("hook found in Auto — should always be in Confirm: %s", item.Name)
		}
	}
}

func TestAnalyzer_SensitiveRoot(t *testing.T) {
	t.Parallel()
	a := New(DefaultConfig())
	_, err := a.Analyze("/")
	if err == nil {
		t.Error("expected error for sensitive root path, got nil")
	}
}
