package loadout

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestResolve_AllFound(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "security-conventions", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "secure-deploy", Type: catalog.Skills},
			{Name: "test-server", Type: catalog.MCP},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []string{"security-conventions"},
		Skills:   []string{"secure-deploy"},
		MCP:      []string{"test-server"},
	}

	refs, err := Resolve(manifest, cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %d", len(refs))
	}
}

func TestResolve_MissingRule(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "other-rule", Type: catalog.Rules, Provider: "claude-code"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []string{"security-conventions"},
	}

	_, err := Resolve(manifest, cat)
	if err == nil {
		t.Fatal("expected error for missing rule")
	}
	if !strings.Contains(err.Error(), "security-conventions not found") {
		t.Errorf("error should mention missing rule name, got: %v", err)
	}
}

func TestResolve_MultipleErrors(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{}, // empty catalog
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []string{"missing-rule"},
		Skills:   []string{"missing-skill"},
		MCP:      []string{"missing-mcp"},
	}

	_, err := Resolve(manifest, cat)
	if err == nil {
		t.Fatal("expected error for missing refs")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "missing-rule") {
		t.Errorf("error should mention missing-rule, got: %v", err)
	}
	if !strings.Contains(errStr, "missing-skill") {
		t.Errorf("error should mention missing-skill, got: %v", err)
	}
	if !strings.Contains(errStr, "missing-mcp") {
		t.Errorf("error should mention missing-mcp, got: %v", err)
	}
}

func TestResolve_ProviderScopedLookup(t *testing.T) {
	t.Parallel()

	// A rule exists for a different provider — should NOT match
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-rule", Type: catalog.Rules, Provider: "cursor"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []string{"my-rule"},
	}

	_, err := Resolve(manifest, cat)
	if err == nil {
		t.Fatal("expected error: rule exists for cursor but manifest targets claude-code")
	}
	if !strings.Contains(err.Error(), "my-rule not found") {
		t.Errorf("error should mention my-rule, got: %v", err)
	}
}

func TestResolve_UniversalIgnoresProvider(t *testing.T) {
	t.Parallel()

	// Universal types should match regardless of provider field
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills}, // no provider
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Skills:   []string{"my-skill"},
	}

	refs, err := Resolve(manifest, cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %d", len(refs))
	}
}

func TestResolve_EmptyManifest(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "some-rule", Type: catalog.Rules, Provider: "claude-code"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		// No rules, hooks, skills, etc.
	}

	refs, err := Resolve(manifest, cat)
	if err != nil {
		t.Fatalf("unexpected error for empty manifest: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for empty manifest, got %d", len(refs))
	}
}

func TestResolve_PrecedenceFirstWins(t *testing.T) {
	t.Parallel()

	// Catalog items are ordered by precedence — first match should win
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "my-skill", Type: catalog.Skills, Path: "/local/skills/my-skill", Local: true},
			{Name: "my-skill", Type: catalog.Skills, Path: "/repo/skills/my-skill"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Skills:   []string{"my-skill"},
	}

	refs, err := Resolve(manifest, cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refs[0].Item.Path != "/local/skills/my-skill" {
		t.Errorf("expected local item (first in catalog), got path: %s", refs[0].Item.Path)
	}
}
