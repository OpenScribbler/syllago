package capyaml_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
)

func TestLoadCapabilityYAML_Roundtrip(t *testing.T) {
	p := filepath.Join("testdata", "claude-code-minimal.yaml")
	caps, err := capyaml.LoadCapabilityYAML(p)
	if err != nil {
		t.Fatalf("LoadCapabilityYAML: %v", err)
	}
	if caps.Slug != "claude-code" {
		t.Errorf("Slug = %q", caps.Slug)
	}
	hooks, ok := caps.ContentTypes["hooks"]
	if !ok {
		t.Fatal("hooks content type missing")
	}
	if !hooks.Supported {
		t.Error("hooks should be supported")
	}
}

func TestValidateAgainstSchema_ValidDoc(t *testing.T) {
	p := filepath.Join("testdata", "claude-code-minimal.yaml")
	if err := capyaml.ValidateAgainstSchema(p, false); err != nil {
		t.Fatalf("ValidateAgainstSchema: %v", err)
	}
}

func TestValidateAgainstSchema_UnknownSchemaVersion(t *testing.T) {
	p := filepath.Join("testdata", "schema-version-99.yaml")
	err := capyaml.ValidateAgainstSchema(p, false)
	if err == nil {
		t.Error("expected error for unknown schema version")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("error %q should mention schema_version", err.Error())
	}
}

func TestValidateAgainstSchema_MigrationWindow_PreviousVersionAccepted(t *testing.T) {
	// When --migration-window is set, the previous schema version is also accepted.
	// With only version "1" in supportedSchemaVersions, the migration window does not
	// admit version "99" — only the immediately previous known version would be admitted
	// once a second version is added to the list. This test documents the expected behavior
	// for when a version bump happens in the future.
	p := filepath.Join("testdata", "claude-code-minimal.yaml")
	// Current version "1" should validate regardless of migrationWindow
	if err := capyaml.ValidateAgainstSchema(p, true); err != nil {
		t.Fatalf("current version should pass with migrationWindow=true: %v", err)
	}
	// Unknown version "99" should still fail even with migrationWindow=true
	p99 := filepath.Join("testdata", "schema-version-99.yaml")
	err := capyaml.ValidateAgainstSchema(p99, true)
	if err == nil {
		t.Error("version 99 should not pass even with migrationWindow=true (only current-minus-one is admitted)")
	}
}

func TestProviderExclusiveRoundtrip(t *testing.T) {
	p := filepath.Join("testdata", "claude-code-minimal.yaml")
	caps, err := capyaml.LoadCapabilityYAML(p)
	if err != nil {
		t.Fatalf("LoadCapabilityYAML: %v", err)
	}
	// Write to buffer and re-read
	var buf bytes.Buffer
	if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
		t.Fatalf("WriteCapabilityYAML: %v", err)
	}
	// provider_exclusive section must be present in output unchanged
	out := buf.String()
	if !strings.Contains(out, "provider_exclusive") {
		t.Error("provider_exclusive section missing from written YAML")
	}
	if !strings.Contains(out, "InstructionsLoaded") {
		t.Error("provider_exclusive entry InstructionsLoaded missing from written YAML")
	}
}

func TestLoadCapabilityYAML_FileNotFound(t *testing.T) {
	_, err := capyaml.LoadCapabilityYAML("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadCapabilityYAML_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.yaml")
	// This YAML has a tab indentation which is invalid
	bad := "schema_version: \"1\"\n\t: invalid_tab_key"
	if err := os.WriteFile(f, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := capyaml.LoadCapabilityYAML(f)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidateAgainstSchema_FileNotFound(t *testing.T) {
	err := capyaml.ValidateAgainstSchema("/nonexistent/path.yaml", false)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestCapabilityEntry_ConfidenceField(t *testing.T) {
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    supported: true
    capabilities:
      display_name:
        supported: true
        mechanism: "yaml frontmatter key: name"
        confidence: confirmed
      description:
        supported: true
        mechanism: "yaml frontmatter key: description"
        confidence: inferred
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	caps, err := capyaml.LoadCapabilityYAML(path)
	if err != nil {
		t.Fatalf("LoadCapabilityYAML: %v", err)
	}
	skills := caps.ContentTypes["skills"]
	dn := skills.Capabilities["display_name"]
	if dn.Confidence != "confirmed" {
		t.Errorf("display_name.confidence: got %q, want %q", dn.Confidence, "confirmed")
	}
	desc := skills.Capabilities["description"]
	if desc.Confidence != "inferred" {
		t.Errorf("description.confidence: got %q, want %q", desc.Confidence, "inferred")
	}
	// Round-trip: write and re-read
	var buf bytes.Buffer
	if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
		t.Fatalf("WriteCapabilityYAML: %v", err)
	}
	written := buf.String()
	if !strings.Contains(written, "confidence: confirmed") {
		t.Error("written YAML missing 'confidence: confirmed'")
	}
	if !strings.Contains(written, "confidence: inferred") {
		t.Error("written YAML missing 'confidence: inferred'")
	}
}

func TestProviderCapabilities_References(t *testing.T) {
	p := filepath.Join("testdata", "claude-code-minimal.yaml")
	caps, err := capyaml.LoadCapabilityYAML(p)
	if err != nil {
		t.Fatalf("LoadCapabilityYAML: %v", err)
	}
	// References table must load and round-trip
	ref, ok := caps.References["cc_hooks_docs"]
	if !ok {
		t.Fatal("references.cc_hooks_docs missing")
	}
	if ref.URL == "" {
		t.Error("ReferenceEntry.URL is empty")
	}
	if ref.FetchMethod == "" {
		t.Error("ReferenceEntry.FetchMethod is empty")
	}
	// Verify round-trip through WriteCapabilityYAML
	var buf bytes.Buffer
	if err := capyaml.WriteCapabilityYAML(&buf, caps); err != nil {
		t.Fatalf("WriteCapabilityYAML: %v", err)
	}
	if !strings.Contains(buf.String(), "cc_hooks_docs") {
		t.Error("references.cc_hooks_docs missing from written YAML")
	}
}
