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

func TestDeriveSeederSpec_Deterministic(t *testing.T) {
	t.Parallel()
	doc := makeTestFormatDoc("amp")
	canonicalKeysPath := makeTestCanonicalKeysFile(t)

	spec1, err := DeriveSeederSpec(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("first derive: %v", err)
	}
	spec2, err := DeriveSeederSpec(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("second derive: %v", err)
	}
	if !reflect.DeepEqual(spec1, spec2) {
		t.Error("DeriveSeederSpec is not deterministic: two calls with same input produced different output")
	}
}

func TestDeriveSeederSpec_UnknownKey(t *testing.T) {
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

	_, err := DeriveSeederSpec(doc, canonicalKeysPath)
	if err == nil {
		t.Fatal("expected error for unknown canonical key")
	}
}

func TestDeriveSeederSpec_UnsupportedSkipped(t *testing.T) {
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

	spec, err := DeriveSeederSpec(doc, canonicalKeysPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Content types with status=unsupported should produce no proposed mappings.
	if len(spec.ProposedMappings) != 0 {
		t.Errorf("expected 0 proposed mappings for unsupported content type, got %d", len(spec.ProposedMappings))
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
