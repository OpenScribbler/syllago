package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// Content stored under a retired provider directory, or whose metadata names
// a retired source provider, scans as the current provider while keeping its
// on-disk path.
func TestScanResolvesRetiredProviderSlug(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	ruleDir := filepath.Join(tmp, "rules", "windsurf", "old-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Old Rule\n"), 0644)
	os.WriteFile(filepath.Join(ruleDir, ".syllago.yaml"), []byte("name: old-rule\nsource_provider: windsurf\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)

	cat, err := Scan(tmp, tmp)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	rules := cat.ByType(Rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	if r.Provider != "devin" {
		t.Errorf("Provider = %q, want devin", r.Provider)
	}
	if r.Path != ruleDir {
		t.Errorf("Path = %q, want %q", r.Path, ruleDir)
	}
	if r.Meta == nil || r.Meta.SourceProvider != "devin" {
		t.Errorf("Meta.SourceProvider = %v, want devin", r.Meta)
	}
}
