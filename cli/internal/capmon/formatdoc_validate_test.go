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

func TestValidateFormatDoc_SupportedUnknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// supported: unknown is invalid YAML for a bool field — validate should
	// catch it before the YAML parser produces an opaque type error.
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
        supported: unknown
        mechanism: "yaml key: name"
        confidence: unknown
`
	writeTestFormatDoc(t, formatsDir, "test-provider", content)

	err := ValidateFormatDoc(formatsDir, canonicalKeysPath, "test-provider")
	if err == nil {
		t.Fatal("expected error for supported: unknown")
	}
	if !strings.Contains(err.Error(), "supported: unknown") {
		t.Errorf("error should mention the bad pattern, got: %v", err)
	}
	if !strings.Contains(err.Error(), "supported: false") {
		t.Errorf("error should suggest the fix (supported: false), got: %v", err)
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

func TestValidateFormatDocWithWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	canonicalKeysPath := writeTestCanonicalKeys(t, dir)

	tests := []struct {
		name          string
		yamlContent   string
		wantWarnCount int
		wantWarnField string
	}{
		{
			name:          "no_warnings_when_empty",
			yamlContent:   validFormatDocContent,
			wantWarnCount: 0,
		},
		{
			name: "value_type_not_in_allow_list",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: bad_type
        name: Bad Type
        description: "test"
        source_ref: "https://example.com"
        value_type: "array<string>"
`,
			wantWarnCount: 1,
			wantWarnField: "value_type",
		},
		{
			name: "example_lang_not_in_allow_list",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: bad_lang
        name: Bad Lang
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: "cobol"
            code: "MOVE 1 TO X."
`,
			wantWarnCount: 1,
			wantWarnField: "examples",
		},
		{
			name: "example_lang_empty_is_warning",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: no_lang
        name: No Lang
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: ""
            code: "x: 1"
`,
			wantWarnCount: 1,
			wantWarnField: "lang",
		},
		{
			name: "example_code_empty_is_warning",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: no_code
        name: No Code
        description: "test"
        source_ref: "https://example.com"
        examples:
          - lang: "yaml"
            code: ""
`,
			wantWarnCount: 1,
			wantWarnField: "code",
		},
		{
			name: "source_section_not_in_allow_list",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: "2026-04-14T00:00:00Z"
        section: "Extension: model"
    canonical_mappings: {}
    provider_extensions: []
`,
			wantWarnCount: 1,
			wantWarnField: "section",
		},
		{
			name: "allow_listed_value_type_no_warning",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: ok_type
        name: OK Type
        description: "test"
        source_ref: "https://example.com"
        value_type: "string | string[]"
        examples:
          - lang: yaml
            code: "model: x"
`,
			wantWarnCount: 0,
		},
		{
			name: "multiple_violations_accumulate",
			yamlContent: `provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: multi
        name: Multi
        description: "test"
        source_ref: "https://example.com"
        value_type: "not-valid"
        examples:
          - lang: "perl"
            code: "x"
`,
			wantWarnCount: 2,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			subDir := filepath.Join(dir, "formats-"+tc.name)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFormatDoc(t, subDir, "test-provider", tc.yamlContent)
			warnings, err := ValidateFormatDocWithWarnings(subDir, canonicalKeysPath, "test-provider")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(warnings) != tc.wantWarnCount {
				t.Errorf("warning count = %d, want %d; warnings: %+v", len(warnings), tc.wantWarnCount, warnings)
			}
			if tc.wantWarnField != "" {
				found := false
				for _, w := range warnings {
					if strings.Contains(w.Field, tc.wantWarnField) || strings.Contains(w.Message, tc.wantWarnField) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no warning mentioning %q in fields %+v", tc.wantWarnField, warnings)
				}
			}
		})
	}
}

func TestValidationWarning_DeduplicationKey(t *testing.T) {
	t.Parallel()
	w1 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[x].value_type", Value: "bad"}
	w2 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[x].value_type", Value: "bad"}
	w3 := ValidationWarning{File: "/a/file.yaml", Field: "content_types.skills.provider_extensions[y].value_type", Value: "bad"}

	if w1.DeduplicationKey() != w2.DeduplicationKey() {
		t.Error("identical warnings must produce identical dedup keys")
	}
	if w1.DeduplicationKey() == w3.DeduplicationKey() {
		t.Error("warnings with different fields must produce different dedup keys")
	}
	if len(w1.DeduplicationKey()) != 16 {
		t.Errorf("dedup key must be 16 hex chars, got len %d: %q", len(w1.DeduplicationKey()), w1.DeduplicationKey())
	}
}
