package catalog

import (
	"testing"
)

// --- ContentType.Label (0% coverage) ---

func TestContentType_Label(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ct   ContentType
		want string
	}{
		{Skills, "Skills"},
		{Agents, "Agents"},
		{MCP, "MCP Servers"},
		{Rules, "Rules"},
		{Hooks, "Hooks"},
		{Commands, "Commands"},
		{Loadouts, "Loadouts"},
		{ContentType("unknown-type"), "unknown-type"}, // fallback
	} {
		if got := tc.ct.Label(); got != tc.want {
			t.Errorf("Label(%q) = %q, want %q", tc.ct, got, tc.want)
		}
	}
}

// --- Catalog filter and count helpers (0% coverage) ---

func makeCatalog() *Catalog {
	return &Catalog{Items: []ContentItem{
		// Shared (no Library, no Registry)
		{Name: "rule-shared", Type: Rules},
		{Name: "skill-shared", Type: Skills},
		// Library items
		{Name: "rule-lib", Type: Rules, Library: true},
		{Name: "skill-lib", Type: Skills, Library: true},
		// Registry items
		{Name: "rule-reg-a", Type: Rules, Registry: "acme"},
		{Name: "rule-reg-b", Type: Rules, Registry: "beta"},
		{Name: "skill-reg-a", Type: Skills, Registry: "acme"},
	}}
}

func TestCatalog_ByTypeShared(t *testing.T) {
	t.Parallel()
	cat := makeCatalog()

	got := cat.ByTypeShared(Rules)
	if len(got) != 1 {
		t.Fatalf("got %d shared rules, want 1", len(got))
	}
	if got[0].Name != "rule-shared" {
		t.Errorf("got %q, want %q", got[0].Name, "rule-shared")
	}

	if got := cat.ByTypeShared(Skills); len(got) != 1 || got[0].Name != "skill-shared" {
		t.Errorf("ByTypeShared(Skills) = %+v, want [skill-shared]", got)
	}
	if got := cat.ByTypeShared(Hooks); len(got) != 0 {
		t.Errorf("ByTypeShared(Hooks) = %d items, want 0", len(got))
	}
}

func TestCatalog_ByRegistry(t *testing.T) {
	t.Parallel()
	cat := makeCatalog()

	got := cat.ByRegistry("acme")
	if len(got) != 2 {
		t.Fatalf("got %d acme items, want 2", len(got))
	}
	for _, item := range got {
		if item.Registry != "acme" {
			t.Errorf("item %q has Registry=%q, want acme", item.Name, item.Registry)
		}
	}

	if got := cat.ByRegistry("beta"); len(got) != 1 {
		t.Errorf("ByRegistry(beta) = %d, want 1", len(got))
	}
	if got := cat.ByRegistry("nonexistent"); len(got) != 0 {
		t.Errorf("ByRegistry(nonexistent) = %d, want 0", len(got))
	}
}

func TestCatalog_CountLibrary(t *testing.T) {
	t.Parallel()
	cat := makeCatalog()
	if got := cat.CountLibrary(); got != 2 {
		t.Errorf("CountLibrary = %d, want 2", got)
	}
	empty := &Catalog{}
	if got := empty.CountLibrary(); got != 0 {
		t.Errorf("empty CountLibrary = %d, want 0", got)
	}
}

func TestCatalog_ByTypeLibrary(t *testing.T) {
	t.Parallel()
	cat := makeCatalog()

	if got := cat.ByTypeLibrary(Rules); len(got) != 1 || got[0].Name != "rule-lib" {
		t.Errorf("ByTypeLibrary(Rules) = %+v, want [rule-lib]", got)
	}
	if got := cat.ByTypeLibrary(Skills); len(got) != 1 || got[0].Name != "skill-lib" {
		t.Errorf("ByTypeLibrary(Skills) = %+v, want [skill-lib]", got)
	}
	if got := cat.ByTypeLibrary(Hooks); len(got) != 0 {
		t.Errorf("ByTypeLibrary(Hooks) = %d, want 0", len(got))
	}
}

func TestCatalog_CountRegistry(t *testing.T) {
	t.Parallel()
	cat := makeCatalog()
	if got := cat.CountRegistry("acme"); got != 2 {
		t.Errorf("CountRegistry(acme) = %d, want 2", got)
	}
	if got := cat.CountRegistry("beta"); got != 1 {
		t.Errorf("CountRegistry(beta) = %d, want 1", got)
	}
	if got := cat.CountRegistry("nonexistent"); got != 0 {
		t.Errorf("CountRegistry(nonexistent) = %d, want 0", got)
	}
}
