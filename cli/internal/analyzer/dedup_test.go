package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func findByName(items []*DetectedItem, name string) *DetectedItem {
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func TestDeduplicateItems_HookScriptSuppression(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "validate", Type: catalog.Hooks, InternalLabel: "hook-script", Path: "hooks/validate.ts", Provider: "claude-code"},
		{Name: "pre-tool:0", Type: catalog.Hooks, Path: ".claude/settings.json", Provider: "claude-code", Scripts: []string{"hooks/validate.ts"}},
	}

	deduped, _ := DeduplicateItems(items)
	if findByName(deduped, "validate") != nil {
		t.Error("hook-script 'validate' should be suppressed (consumed by wired hook)")
	}
	if findByName(deduped, "pre-tool:0") == nil {
		t.Error("wired hook 'pre-tool:0' should be kept")
	}
}

func TestDeduplicateItems_HookScriptNotSuppressed(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "lint", Type: catalog.Hooks, InternalLabel: "hook-script", Path: "hooks/lint.sh", Provider: "claude-code"},
	}

	deduped, _ := DeduplicateItems(items)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 item, got %d", len(deduped))
	}
	if deduped[0].Name != "lint" {
		t.Errorf("Name = %q, want %q", deduped[0].Name, "lint")
	}
}

func TestDeduplicateItems_SameHash_HigherConfidenceWins(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "my-skill", Type: catalog.Skills, Provider: "top-level", Confidence: 0.80, ContentHash: "abc123", Path: "skills/my-skill"},
		{Name: "my-skill", Type: catalog.Skills, Provider: "syllago", Confidence: 0.95, ContentHash: "abc123", Path: "skills/my-skill/SKILL.md"},
	}

	deduped, conflicts := DeduplicateItems(items)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
	if len(deduped) != 1 {
		t.Fatalf("expected 1 deduped item, got %d", len(deduped))
	}
	if deduped[0].Provider != "syllago" {
		t.Errorf("Provider = %q, want syllago (higher confidence)", deduped[0].Provider)
	}
	if len(deduped[0].Providers) != 1 {
		t.Errorf("Providers aliases = %d, want 1", len(deduped[0].Providers))
	}
}

func TestDeduplicateItems_SameHash_EqualConfidence_SyllagoWins(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "my-rule", Type: catalog.Rules, Provider: "claude-code", Confidence: 0.90, ContentHash: "def456", Path: ".claude/rules/my-rule.md"},
		{Name: "my-rule", Type: catalog.Rules, Provider: "syllago", Confidence: 0.90, ContentHash: "def456", Path: "rules/claude-code/my-rule"},
	}

	deduped, _ := DeduplicateItems(items)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 item, got %d", len(deduped))
	}
	if deduped[0].Provider != "syllago" {
		t.Errorf("Provider = %q, want syllago (Decision #19 tiebreak)", deduped[0].Provider)
	}
}

func TestDeduplicateItems_SameHash_EqualConfidence_CCBeatsTopLevel(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "agent-x", Type: catalog.Agents, Provider: "top-level", Confidence: 0.85, ContentHash: "ghi789", Path: "agents/agent-x.md"},
		{Name: "agent-x", Type: catalog.Agents, Provider: "claude-code", Confidence: 0.85, ContentHash: "ghi789", Path: ".claude/agents/agent-x.md"},
	}

	deduped, _ := DeduplicateItems(items)
	if len(deduped) != 1 {
		t.Fatalf("expected 1 item, got %d", len(deduped))
	}
	if deduped[0].Provider != "claude-code" {
		t.Errorf("Provider = %q, want claude-code (beats top-level)", deduped[0].Provider)
	}
}

func TestDeduplicateItems_DifferentHash_Conflict(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "my-skill", Type: catalog.Skills, Provider: "syllago", ContentHash: "hash1", Path: "skills/my-skill"},
		{Name: "my-skill", Type: catalog.Skills, Provider: "cursor", ContentHash: "hash2", Path: ".cursor/skills/my-skill"},
	}

	_, conflicts := DeduplicateItems(items)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
}

func TestDeduplicateItems_CrossType_NotDeduped(t *testing.T) {
	t.Parallel()

	items := []*DetectedItem{
		{Name: "foo", Type: catalog.Skills, Provider: "syllago", ContentHash: "h1", Path: "skills/foo"},
		{Name: "foo", Type: catalog.Rules, Provider: "syllago", ContentHash: "h2", Path: "rules/foo"},
	}

	deduped, conflicts := DeduplicateItems(items)
	if len(deduped) != 2 {
		t.Errorf("expected 2 items (different types), got %d", len(deduped))
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestDeduplicateItems_Empty(t *testing.T) {
	t.Parallel()

	deduped, conflicts := DeduplicateItems(nil)
	if len(deduped) != 0 {
		t.Errorf("expected 0 deduped, got %d", len(deduped))
	}
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}
