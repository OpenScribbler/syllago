package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestCanonicalKeys writes a minimal canonical-keys.yaml to dir for tests.
func writeTestCanonicalKeys(t *testing.T, dir string) string {
	t.Helper()
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

// writeTestFormatDoc writes a valid format doc YAML to dir/<provider>.yaml.
func writeTestFormatDoc(t *testing.T, dir, provider, content string) {
	t.Helper()
	path := filepath.Join(dir, provider+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const validFormatDocContent = `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited

content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/docs"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions:
      - id: custom_ext
        name: "Custom Extension"
        description: "A custom capability"
        source_ref: "https://example.com/docs"
        graduation_candidate: false
    notes: ""
`

func TestValidateFormatDoc_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFormatDoc(t, formatsDir, "test-provider", validFormatDocContent)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err != nil {
		t.Errorf("expected no error for valid format doc, got: %v", err)
	}
}

func TestValidateFormatDoc_UnknownKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      unknown_key:
        supported: true
        mechanism: "some mechanism"
        confidence: confirmed
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err == nil {
		t.Fatal("expected error for unknown canonical key")
	}
	if !strings.Contains(err.Error(), "unknown_key") {
		t.Errorf("error should mention unknown_key, got: %v", err)
	}
}

func TestValidateFormatDoc_MissingExtensionField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    provider_extensions:
      - id: my_ext
        name: "My Extension"
        source_ref: "https://example.com"
        graduation_candidate: false
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err == nil {
		t.Fatal("expected error for missing description field")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("error should mention description, got: %v", err)
	}
}

func TestValidateFormatDoc_InvalidConfidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: maybe
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err == nil {
		t.Fatal("expected error for invalid confidence value")
	}
	if !strings.Contains(err.Error(), "maybe") {
		t.Errorf("error should mention invalid value 'maybe', got: %v", err)
	}
}

func TestValidateFormatDoc_MissingProvider(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `provider: ""
last_fetched_at: "2026-04-11T00:00:00Z"
content_types:
  skills:
    status: supported
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err == nil {
		t.Fatal("expected error for empty provider field")
	}
}

func TestValidateFormatDoc_InformationalFieldsNotValidated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `provider: test-provider
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: anything-goes
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-11T00:00:00Z"
    notes: "This note can say literally anything and should not cause validation errors"
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err != nil {
		t.Errorf("expected no error — notes and generation_method are informational, got: %v", err)
	}
}
