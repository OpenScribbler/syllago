package capmon_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"gopkg.in/yaml.v3"
)

// TestSeederSpecs_AllLoad verifies every <slug>-<content_type>.yaml in
// .develop/seeder-specs/ parses cleanly against the current SeederSpec schema.
// Catches schema drift: if the SeederSpec struct changes in a way that breaks
// existing specs, this test fails before capmon pipelines break silently.
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

// TestDeriveOutputMatchesCommitted is the drift gate for committed
// docs/provider-capabilities/<slug>-<content_type>.yaml files. For every
// provider format doc, it runs DeriveSeederSpecs and compares the marshalled
// output byte-for-byte against the committed file. A non-zero diff means
// committed YAMLs have drifted from what derive currently produces — every
// downstream PR would carry the diff as incidental noise.
//
// This test was added to lock in the per-content-type fix from the derive.go
// refactor so future regressions surface immediately instead of polluting
// capmon-process PRs.
func TestDeriveOutputMatchesCommitted(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	formatsDir := filepath.Join(repoRoot, "docs", "provider-formats")
	capsDir := filepath.Join(repoRoot, "docs", "provider-capabilities")
	canonicalKeysPath := filepath.Join(repoRoot, "docs", "spec", "canonical-keys.yaml")

	entries, err := os.ReadDir(formatsDir)
	if err != nil {
		t.Fatalf("read formats dir %s: %v", formatsDir, err)
	}

	checked := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		provider := strings.TrimSuffix(e.Name(), ".yaml")

		doc, err := capmon.LoadFormatDoc(filepath.Join(formatsDir, e.Name()))
		if err != nil {
			t.Errorf("%s: load format doc: %v", provider, err)
			continue
		}

		specs, err := capmon.DeriveSeederSpecs(doc, canonicalKeysPath)
		if err != nil {
			t.Errorf("%s: derive specs: %v", provider, err)
			continue
		}

		for _, spec := range specs {
			gotBytes, err := yaml.Marshal(spec)
			if err != nil {
				t.Errorf("%s/%s: marshal spec: %v", provider, spec.ContentType, err)
				continue
			}

			committedPath := capmon.SeederSpecPath(capsDir, provider, spec.ContentType)
			wantBytes, err := os.ReadFile(committedPath)
			if err != nil {
				t.Errorf("%s/%s: read committed %s: %v", provider, spec.ContentType, committedPath, err)
				continue
			}

			if !bytes.Equal(gotBytes, wantBytes) {
				t.Errorf("%s/%s: drift detected — derive output does not match committed %s\nRun: syllago capmon derive --provider=%s --output-dir docs/provider-capabilities", provider, spec.ContentType, committedPath, provider)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no provider/content_type pairs checked — formatsDir empty or all derives failed")
	}
	t.Logf("verified %d (provider, content_type) pairs match committed bytes", checked)
}
