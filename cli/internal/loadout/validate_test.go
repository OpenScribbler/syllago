package loadout

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestValidate_Clean(t *testing.T) {
	t.Parallel()

	prov := provider.ClaudeCode
	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}},
		{Type: catalog.Skills, Name: "my-skill", Item: catalog.ContentItem{Name: "my-skill", Type: catalog.Skills}},
		{Type: catalog.MCP, Name: "my-mcp", Item: catalog.ContentItem{Name: "my-mcp", Type: catalog.MCP}},
	}

	results := Validate(refs, prov)
	if len(results) != 0 {
		t.Errorf("expected no issues, got %d: %v", len(results), results)
	}
}

func TestValidate_UnsupportedType(t *testing.T) {
	t.Parallel()

	// Create a provider that only supports Rules
	prov := provider.Provider{
		Name: "limited-provider",
		Slug: "limited",
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}
	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}},
		{Type: catalog.Skills, Name: "my-skill", Item: catalog.ContentItem{Name: "my-skill", Type: catalog.Skills}},
	}

	results := Validate(refs, prov)
	if len(results) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(results))
	}
	if results[0].Ref.Name != "my-skill" {
		t.Errorf("expected issue for my-skill, got %s", results[0].Ref.Name)
	}
}

func TestValidate_DuplicateRef(t *testing.T) {
	t.Parallel()

	prov := provider.ClaudeCode
	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}},
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}},
	}

	results := Validate(refs, prov)
	if len(results) != 1 {
		t.Fatalf("expected 1 issue (duplicate), got %d", len(results))
	}
	if results[0].Ref.Name != "my-rule" {
		t.Errorf("expected duplicate issue for my-rule, got %s", results[0].Ref.Name)
	}
}

func TestValidate_EmptyRefs(t *testing.T) {
	t.Parallel()

	prov := provider.ClaudeCode
	results := Validate(nil, prov)
	if len(results) != 0 {
		t.Errorf("expected no issues for nil refs, got %d", len(results))
	}

	results = Validate([]ResolvedRef{}, prov)
	if len(results) != 0 {
		t.Errorf("expected no issues for empty refs, got %d", len(results))
	}
}

func TestValidate_NilSupportsType(t *testing.T) {
	t.Parallel()

	// If SupportsType is nil, we skip that check (don't crash)
	prov := provider.Provider{
		Name:         "no-supports",
		Slug:         "none",
		SupportsType: nil,
	}
	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "my-rule", Item: catalog.ContentItem{Name: "my-rule", Type: catalog.Rules}},
	}

	results := Validate(refs, prov)
	if len(results) != 0 {
		t.Errorf("expected no issues with nil SupportsType, got %d", len(results))
	}
}
