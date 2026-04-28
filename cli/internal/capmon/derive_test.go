package capmon

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func makeTestFormatDoc(provider string) *FormatDoc {
	return &FormatDoc{
		Provider:         provider,
		LastFetchedAt:    "2026-04-11T00:00:00Z",
		GenerationMethod: "human-edited",
		ContentTypes: map[string]ContentTypeFormatDoc{
			"skills": {
				Status: "supported",
				CanonicalMappings: map[string]CanonicalMapping{
					"display_name": {
						Supported:  true,
						Mechanism:  "yaml key: name",
						Confidence: "confirmed",
					},
					"project_scope": {
						Supported:  true,
						Mechanism:  ".agents/skills/<name>/SKILL.md",
						Confidence: "confirmed",
					},
				},
			},
		},
	}
}

func makeMultiCTFormatDoc(provider string) *FormatDoc {
	return &FormatDoc{
		Provider:         provider,
		LastFetchedAt:    "2026-04-11T00:00:00Z",
		GenerationMethod: "human-edited",
		ContentTypes: map[string]ContentTypeFormatDoc{
			"skills": {
				Status: "supported",
				CanonicalMappings: map[string]CanonicalMapping{
					"display_name": {
						Supported:  true,
						Mechanism:  "yaml key: name",
						Confidence: "confirmed",
					},
				},
			},
			"rules": {
				Status: "supported",
				CanonicalMappings: map[string]CanonicalMapping{
					"description": {
						Supported:  true,
						Mechanism:  "frontmatter description",
						Confidence: "confirmed",
					},
				},
			},
		},
	}
}

func makeMultiCTCanonicalKeysFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := `content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
    project_scope:
      description: "Project scope"
      type: bool
  rules:
    description:
      description: "Description"
      type: string
`
	path := filepath.Join(dir, "canonical-keys.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeTestCanonicalKeysFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := `content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
    description:
      description: "Description"
      type: string
    project_scope:
      description: "Project scope"
      type: bool
`
	path := filepath.Join(dir, "canonical-keys.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeriveSeederSpecs_Deterministic(t *testing.T) {
	t.Parallel()
	doc := makeTestFormatDoc("amp")
	canonicalKeysPath := makeTestCanonicalKeysFile(t)

	specs1, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("first derive: %v", err)
	}
	specs2, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("second derive: %v", err)
	}
	if !reflect.DeepEqual(specs1, specs2) {
		t.Error("DeriveSeederSpecs is not deterministic: two calls with same input produced different output")
	}
}

func TestDeriveSeederSpecs_UnknownKey(t *testing.T) {
	t.Parallel()
	doc := &FormatDoc{
		Provider:         "test",
		LastFetchedAt:    "2026-04-11T00:00:00Z",
		GenerationMethod: "human-edited",
		ContentTypes: map[string]ContentTypeFormatDoc{
			"skills": {
				Status: "supported",
				CanonicalMappings: map[string]CanonicalMapping{
					"not_a_real_key": {
						Supported:  true,
						Mechanism:  "something",
						Confidence: "confirmed",
					},
				},
			},
		},
	}
	canonicalKeysPath := makeTestCanonicalKeysFile(t)

	_, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err == nil {
		t.Fatal("expected error for unknown canonical key")
	}
}

func TestDeriveSeederSpecs_UnsupportedSkipped(t *testing.T) {
	t.Parallel()
	doc := &FormatDoc{
		Provider:         "test",
		LastFetchedAt:    "2026-04-11T00:00:00Z",
		GenerationMethod: "human-edited",
		ContentTypes: map[string]ContentTypeFormatDoc{
			"skills": {
				Status: "unsupported",
				CanonicalMappings: map[string]CanonicalMapping{
					"display_name": {Supported: true, Mechanism: "x", Confidence: "confirmed"},
				},
			},
		},
	}
	canonicalKeysPath := makeTestCanonicalKeysFile(t)

	specs, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Content types with status=unsupported should produce no spec at all
	// (not an empty spec — the whole content_type is omitted from output).
	if len(specs) != 0 {
		t.Errorf("expected 0 specs for doc with only unsupported content types, got %d", len(specs))
	}
}

// TestDeriveSeederSpecs_ReturnsOnePerSupportedContentType is the tracer bullet
// for the per-content-type derive shape. A doc with N supported content_types
// must produce N specs, one per content_type, never lumped together.
//
// Regression for the bug where DeriveSeederSpec hardcoded ContentType:"skills"
// and collapsed all content_types into a single spec — see commit 82b64ea4.
func TestDeriveSeederSpecs_ReturnsOnePerSupportedContentType(t *testing.T) {
	t.Parallel()
	doc := makeMultiCTFormatDoc("amp")
	canonicalKeysPath := makeMultiCTCanonicalKeysFile(t)

	specs, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("DeriveSeederSpecs: %v", err)
	}

	if len(specs) != 2 {
		t.Fatalf("expected 2 specs (one per supported content_type), got %d", len(specs))
	}

	gotContentTypes := make(map[string]bool)
	for _, s := range specs {
		gotContentTypes[s.ContentType] = true
		if s.Provider != "amp" {
			t.Errorf("spec for content_type %q has wrong provider: got %q, want amp", s.ContentType, s.Provider)
		}
	}
	if !gotContentTypes["skills"] {
		t.Error("missing spec for content_type skills")
	}
	if !gotContentTypes["rules"] {
		t.Error("missing spec for content_type rules")
	}
}

// TestDeriveSeederSpecs_SortedByContentType pins the documented contract:
// returned specs are sorted alphabetically by ContentType. Callers (and
// drift-detection tests) rely on this for stable file output ordering.
func TestDeriveSeederSpecs_SortedByContentType(t *testing.T) {
	t.Parallel()
	// Doc with three content_types in non-alphabetical map order to defeat any
	// accidental input-order preservation (Go map iteration is randomized but
	// not reverse-sorted).
	doc := &FormatDoc{
		Provider:         "test",
		LastFetchedAt:    "2026-04-11T00:00:00Z",
		GenerationMethod: "human-edited",
		ContentTypes: map[string]ContentTypeFormatDoc{
			"skills": {Status: "supported", CanonicalMappings: map[string]CanonicalMapping{"display_name": {Supported: true, Mechanism: "x", Confidence: "confirmed"}}},
			"rules":  {Status: "supported", CanonicalMappings: map[string]CanonicalMapping{"description": {Supported: true, Mechanism: "x", Confidence: "confirmed"}}},
			"agents": {Status: "supported", CanonicalMappings: map[string]CanonicalMapping{"display_name": {Supported: true, Mechanism: "x", Confidence: "confirmed"}}},
		},
	}
	dir := t.TempDir()
	keysPath := filepath.Join(dir, "canonical-keys.yaml")
	keys := `content_types:
  skills:
    display_name: {description: "x", type: string}
  rules:
    description: {description: "x", type: string}
  agents:
    display_name: {description: "x", type: string}
`
	if err := os.WriteFile(keysPath, []byte(keys), 0644); err != nil {
		t.Fatal(err)
	}

	specs, err := DeriveSeederSpecs(doc, keysPath)
	if err != nil {
		t.Fatalf("DeriveSeederSpecs: %v", err)
	}

	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, s.ContentType)
	}
	want := []string{"agents", "rules", "skills"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spec order: got %v, want %v (sorted alphabetically)", got, want)
	}
}

// TestDeriveSeederSpecs_NoCrossContentTypeBleed verifies each spec's
// ProposedMappings come ONLY from its own content_type's canonical_mappings.
// Cross-CT bleed was the symptom of the original bug — see derive.go before
// the per-CT refactor, which iterated all content_types into a single spec.
func TestDeriveSeederSpecs_NoCrossContentTypeBleed(t *testing.T) {
	t.Parallel()
	doc := makeMultiCTFormatDoc("amp")
	canonicalKeysPath := makeMultiCTCanonicalKeysFile(t)

	specs, err := DeriveSeederSpecs(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("DeriveSeederSpecs: %v", err)
	}

	byCT := make(map[string]*SeederSpec)
	for _, s := range specs {
		byCT[s.ContentType] = s
	}

	skillsSpec, ok := byCT["skills"]
	if !ok {
		t.Fatal("missing spec for skills")
	}
	if len(skillsSpec.ProposedMappings) != 1 {
		t.Errorf("skills spec: expected 1 proposed mapping, got %d", len(skillsSpec.ProposedMappings))
	}
	for _, m := range skillsSpec.ProposedMappings {
		if m.CanonicalKey == "description" {
			t.Errorf("cross-CT bleed: skills spec contains rules' canonical_key %q", m.CanonicalKey)
		}
	}

	rulesSpec, ok := byCT["rules"]
	if !ok {
		t.Fatal("missing spec for rules")
	}
	if len(rulesSpec.ProposedMappings) != 1 {
		t.Errorf("rules spec: expected 1 proposed mapping, got %d", len(rulesSpec.ProposedMappings))
	}
	for _, m := range rulesSpec.ProposedMappings {
		if m.CanonicalKey == "display_name" {
			t.Errorf("cross-CT bleed: rules spec contains skills' canonical_key %q", m.CanonicalKey)
		}
	}
}

func TestWriteSeederSpec_AtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := &SeederSpec{
		Provider:    "test",
		ContentType: "skills",
		ProposedMappings: []ProposedMapping{
			{CanonicalKey: "display_name", Supported: true, Mechanism: "yaml key: name", Confidence: "confirmed"},
		},
	}

	targetPath := filepath.Join(dir, "test-skills.yaml")
	if err := WriteSeederSpec(spec, targetPath); err != nil {
		t.Fatalf("WriteSeederSpec: %v", err)
	}

	// Verify the file exists and is readable.
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read written spec: %v", err)
	}
	if len(data) == 0 {
		t.Error("written spec file is empty")
	}

	// Verify no temp files were left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "test-skills.yaml" {
			t.Errorf("unexpected temp file left behind: %s", e.Name())
		}
	}
}
