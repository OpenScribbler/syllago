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

func TestAnalyzer_ContentSignalFallback_PAIStyle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// PAI-style layout: Packs/<name>/SKILL.md — not matched by any pattern detector.
	setupFile(t, root, "Packs/redteam-skill/SKILL.md",
		"---\nname: Red Team Skill\ndescription: Security testing\n---\nContent.\n")
	setupFile(t, root, "Packs/coding-skill/SKILL.md",
		"---\nname: Coding Skill\ndescription: Writes code\n---\nContent.\n")

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	total := len(result.Auto) + len(result.Confirm)
	if total < 2 {
		t.Errorf("expected ≥2 detected items for PAI-style layout, got %d", total)
	}
	for _, item := range result.AllItems() {
		if item.Provider == "content-signal" && item.Type != catalog.Skills {
			t.Errorf("content-signal item %q should be Skills, got %v", item.Name, item.Type)
		}
	}
}

func TestAnalyzer_ContentSignalFallback_NoDoubleClassify(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Standard CC layout — should be handled by CC detector, NOT content-signal.
	setupFile(t, root, ".claude/agents/my-agent.md",
		"---\nname: My Agent\n---\nAgent body.\n")

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	for _, item := range result.AllItems() {
		if item.Provider == "content-signal" {
			t.Errorf("pattern-matched file %q should not be classified by content-signal detector", item.Path)
		}
	}
}

func TestAnalyzer_ContentSignalFallback_StrictDisabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "Packs/test-skill/SKILL.md",
		"---\nname: Test\n---\nContent.\n")

	cfg := DefaultConfig()
	cfg.Strict = true
	a := New(cfg)
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	for _, item := range result.AllItems() {
		if item.Provider == "content-signal" {
			t.Errorf("strict mode should disable content-signal fallback, but found %q", item.Name)
		}
	}
}

func TestShouldTriggerInteractiveFallback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		itemCount int
		want      bool
	}{
		{0, true},
		{3, true},
		{5, true},
		{6, false},
		{20, false},
	}
	for _, tc := range cases {
		result := &AnalysisResult{}
		for range tc.itemCount {
			result.Confirm = append(result.Confirm, &DetectedItem{})
		}
		got := ShouldTriggerInteractiveFallback(result)
		if got != tc.want {
			t.Errorf("itemCount=%d: got %v, want %v", tc.itemCount, got, tc.want)
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
