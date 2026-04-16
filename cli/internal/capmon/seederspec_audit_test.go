package capmon_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// TestSeederSpecs_AllLoad verifies every *-skills.yaml in .develop/seeder-specs/
// parses cleanly against the current SeederSpec schema. Catches schema drift:
// if the SeederSpec struct changes in a way that breaks existing specs, this
// test fails before capmon pipelines break silently.
func TestSeederSpecs_AllLoad(t *testing.T) {
	specsDir := filepath.Join("..", "..", "..", ".develop", "seeder-specs")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("seeder-specs directory not present (gitignored dev artifact): %s", specsDir)
		}
		t.Fatalf("read specs dir %s: %v", specsDir, err)
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}
		path := filepath.Join(specsDir, name)
		spec, err := capmon.LoadSeederSpec(path)
		if err != nil {
			t.Errorf("%s: load failed: %v", name, err)
			continue
		}
		if spec.Provider == "" {
			t.Errorf("%s: provider field is empty", name)
		}
		if spec.ContentType == "" {
			t.Errorf("%s: content_type field is empty", name)
		}
		// Empty proposed_mappings is valid when the spec records a confirmed
		// negative (provider explicitly does not support this content type).
		// Non-empty specs must have non-empty canonical_key on every entry.
		if len(spec.ProposedMappings) == 0 && spec.Notes == "" {
			t.Errorf("%s: proposed_mappings is empty and notes field does not explain why", name)
		}
		for i, m := range spec.ProposedMappings {
			if m.CanonicalKey == "" {
				t.Errorf("%s: proposed_mappings[%d].canonical_key is empty", name, i)
			}
		}
		loaded++
	}
	if loaded == 0 {
		t.Fatalf("no seeder specs found in %s", specsDir)
	}
	t.Logf("loaded %d seeder specs cleanly", loaded)
}
