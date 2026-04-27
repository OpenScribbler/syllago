package loadout

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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
		Rules:    []ItemRef{{Name: "security-conventions"}},
		Skills:   []ItemRef{{Name: "secure-deploy"}},
		MCP:      []ItemRef{{Name: "test-server"}},
	}

	refs, err := Resolve(manifest, cat, manifest.Provider)
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
		Rules:    []ItemRef{{Name: "security-conventions"}},
	}

	_, err := Resolve(manifest, cat, manifest.Provider)
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
		Rules:    []ItemRef{{Name: "missing-rule"}},
		Skills:   []ItemRef{{Name: "missing-skill"}},
		MCP:      []ItemRef{{Name: "missing-mcp"}},
	}

	_, err := Resolve(manifest, cat, manifest.Provider)
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
		Rules:    []ItemRef{{Name: "my-rule"}},
	}

	_, err := Resolve(manifest, cat, manifest.Provider)
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
		Skills:   []ItemRef{{Name: "my-skill"}},
	}

	refs, err := Resolve(manifest, cat, manifest.Provider)
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

	refs, err := Resolve(manifest, cat, manifest.Provider)
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
			{Name: "my-skill", Type: catalog.Skills, Path: "/local/skills/my-skill", Library: true},
			{Name: "my-skill", Type: catalog.Skills, Path: "/repo/skills/my-skill"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Skills:   []ItemRef{{Name: "my-skill"}},
	}

	refs, err := Resolve(manifest, cat, manifest.Provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refs[0].Item.Path != "/local/skills/my-skill" {
		t.Errorf("expected local item (first in catalog), got path: %s", refs[0].Item.Path)
	}
}

// TestResolve_MultiProviderSlugOverride verifies that providerSlug is used for
// provider-specific lookup instead of manifest.Provider, enabling multi-provider apply.
// When applying a multi-provider loadout to gemini-cli, rules for gemini-cli must
// resolve even though manifest.Providers is set and manifest.Provider is empty.
func TestResolve_MultiProviderSlugOverride(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "concise-comments", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "concise-comments", Type: catalog.Rules, Provider: "gemini-cli"},
			{Name: "my-skill", Type: catalog.Skills},
		},
	}
	// Multi-provider manifest: no single Provider field set.
	manifest := &Manifest{
		Providers: []string{"claude-code", "gemini-cli"},
		Rules:     []ItemRef{{Name: "concise-comments"}},
		Skills:    []ItemRef{{Name: "my-skill"}},
	}

	// Resolve for gemini-cli — should find gemini-cli's rule, not claude-code's.
	refs, err := Resolve(manifest, cat, "gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var ruleRef *ResolvedRef
	for i := range refs {
		if refs[i].Type == catalog.Rules {
			ruleRef = &refs[i]
			break
		}
	}
	if ruleRef == nil {
		t.Fatal("expected a rule ref, got none")
	}
	if ruleRef.Item.Provider != "gemini-cli" {
		t.Errorf("expected gemini-cli rule, got provider %q", ruleRef.Item.Provider)
	}
}

// TestResolve_CrossProviderFallback verifies that when a single-source loadout
// (declares manifest.Provider = "claude-code") is applied --to a different
// target (e.g. gemini-cli) and the target has no flavored copy of the content,
// resolution falls back to the manifest's source provider. This enables the
// cross-provider apply workflow: read claude-code content, convert at install
// time to the gemini-cli target paths.
func TestResolve_CrossProviderFallback(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "smoke-rules", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "smoke-hook", Type: catalog.Hooks, Provider: "claude-code"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []ItemRef{{Name: "smoke-rules"}},
		Hooks:    []ItemRef{{Name: "smoke-hook"}},
	}

	refs, err := Resolve(manifest, cat, "gemini-cli")
	if err != nil {
		t.Fatalf("expected fallback resolution to succeed, got: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	for _, r := range refs {
		if r.Item.Provider != "claude-code" {
			t.Errorf("ref %s: expected provider claude-code (fallback), got %q", r.Name, r.Item.Provider)
		}
	}
}

// TestResolve_TargetWinsOverFallback verifies that when the target provider
// has its own flavored copy of the content, resolution prefers it over the
// manifest's source-provider fallback. Without this, multi-provider loadouts
// with both flavors present would resolve to the wrong copy.
func TestResolve_TargetWinsOverFallback(t *testing.T) {
	t.Parallel()

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "shared", Type: catalog.Rules, Provider: "claude-code"},
			{Name: "shared", Type: catalog.Rules, Provider: "gemini-cli"},
		},
	}
	manifest := &Manifest{
		Provider: "claude-code",
		Rules:    []ItemRef{{Name: "shared"}},
	}

	refs, err := Resolve(manifest, cat, "gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Item.Provider != "gemini-cli" {
		t.Errorf("expected target gemini-cli to win, got provider %q", refs[0].Item.Provider)
	}
}
