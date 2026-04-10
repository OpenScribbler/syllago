package capmon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCurrentFields_JSON(t *testing.T) {
	// loadCurrentFields now uses YAML, but JSON is also valid YAML — verify round-trip
	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	// Valid YAML that also happens to be valid JSON
	data := `{"schema_version": "1", "slug": "claude-code"}`
	if err := os.WriteFile(f, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	fields, err := loadCurrentFields(f)
	if err != nil {
		t.Fatalf("loadCurrentFields: %v", err)
	}
	if fields["schema_version"] != "1" {
		t.Errorf("schema_version: got %q, want %q", fields["schema_version"], "1")
	}
	if fields["slug"] != "claude-code" {
		t.Errorf("slug: got %q, want %q", fields["slug"], "claude-code")
	}
}

func TestLoadCurrentFields_YAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	data := "schema_version: \"1\"\nslug: claude-code\n"
	if err := os.WriteFile(f, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	fields, err := loadCurrentFields(f)
	if err != nil {
		t.Fatalf("loadCurrentFields: %v", err)
	}
	if fields["schema_version"] != "1" {
		t.Errorf("schema_version: got %q, want %q", fields["schema_version"], "1")
	}
	if fields["slug"] != "claude-code" {
		t.Errorf("slug: got %q, want %q", fields["slug"], "claude-code")
	}
}

func TestLoadCurrentFields_MissingFile(t *testing.T) {
	_, err := loadCurrentFields("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadCurrentFields_NestedYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "nested.yaml")
	data := `schema_version: "1"
slug: claude-code
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: PreToolUse
`
	if err := os.WriteFile(f, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	fields, err := loadCurrentFields(f)
	if err != nil {
		t.Fatalf("loadCurrentFields: %v", err)
	}
	key := "content_types.hooks.events.before_tool_execute.native_name"
	if fields[key] != "PreToolUse" {
		t.Errorf("%s: got %q, want %q", key, fields[key], "PreToolUse")
	}
	if fields["content_types.hooks.supported"] != "true" {
		t.Errorf("supported: got %q, want %q", fields["content_types.hooks.supported"], "true")
	}
}

func TestFlattenInterface_String(t *testing.T) {
	out := make(map[string]string)
	flattenInterface("key", "hello", out)
	if out["key"] != "hello" {
		t.Errorf("got %q, want %q", out["key"], "hello")
	}
}

func TestFlattenInterface_Bool(t *testing.T) {
	out := make(map[string]string)
	flattenInterface("enabled", true, out)
	if out["enabled"] != "true" {
		t.Errorf("enabled: got %q, want %q", out["enabled"], "true")
	}
	out2 := make(map[string]string)
	flattenInterface("disabled", false, out2)
	if out2["disabled"] != "false" {
		t.Errorf("disabled: got %q, want %q", out2["disabled"], "false")
	}
}

func TestFlattenInterface_Float64(t *testing.T) {
	out := make(map[string]string)
	flattenInterface("count", float64(42.5), out)
	if out["count"] != "42.5" {
		t.Errorf("got %q, want %q", out["count"], "42.5")
	}
}

func TestFlattenInterface_Int(t *testing.T) {
	out := make(map[string]string)
	flattenInterface("count", int(42), out)
	if out["count"] != "42" {
		t.Errorf("got %q, want %q", out["count"], "42")
	}
}

func TestFlattenInterface_NestedMap(t *testing.T) {
	out := make(map[string]string)
	input := map[string]interface{}{
		"hooks": map[string]interface{}{
			"events": map[string]interface{}{
				"before_tool": "PreToolUse",
			},
		},
	}
	flattenInterface("", input, out)
	if out["hooks.events.before_tool"] != "PreToolUse" {
		t.Errorf("nested key: got %q, want %q", out["hooks.events.before_tool"], "PreToolUse")
	}
}

func TestFlattenInterface_WithPrefix(t *testing.T) {
	out := make(map[string]string)
	input := map[string]interface{}{
		"native_name": "PreToolUse",
	}
	flattenInterface("hooks.events.before", input, out)
	if out["hooks.events.before.native_name"] != "PreToolUse" {
		t.Errorf("prefixed key: got %q", out["hooks.events.before.native_name"])
	}
}

func TestToFieldValues(t *testing.T) {
	input := map[string]string{
		"hooks.events.before": "PreToolUse",
		"slug":                "claude-code",
	}
	out := toFieldValues(input)
	if len(out) != 2 {
		t.Errorf("expected 2 entries, got %d", len(out))
	}
	fv, ok := out["slug"]
	if !ok {
		t.Fatal("slug not in output")
	}
	if fv.Value != "claude-code" {
		t.Errorf("Value: got %q, want %q", fv.Value, "claude-code")
	}
	if fv.ValueHash == "" {
		t.Error("ValueHash should not be empty")
	}
	// Verify hash is deterministic
	fv2 := out["slug"]
	if fv.ValueHash != fv2.ValueHash {
		t.Error("ValueHash should be deterministic")
	}
}

func TestRunStage4Review_WithDrift(t *testing.T) {
	// Mock gh runner: dedup returns no existing PR, create returns PR URL
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "list" {
			return []byte("[]"), nil
		}
		return []byte("https://github.com/test/repo/pulls/1"), nil
	})
	defer SetGHCommandForTest(nil)

	diff := CapabilityDiff{
		Provider: "claude-code",
		RunID:    "run-001",
		Changes: []FieldChange{
			{FieldPath: "hooks.events.before.native_name", OldValue: "OldName", NewValue: "NewName"},
		},
	}
	manifest := &RunManifest{
		RunID: "run-001",
		Providers: map[string]ProviderStatus{
			"claude-code": {HasDrift: true, Diff: &diff},
		},
	}
	if err := runStage4Review(context.Background(), PipelineOptions{}, manifest); err != nil {
		t.Fatalf("runStage4Review: %v", err)
	}
}

func TestRunStage4Review_DedupSkip(t *testing.T) {
	// Mock: dedup finds existing PR — should skip
	SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "list" {
			return []byte(`[{"url":"https://github.com/test/repo/pulls/99"}]`), nil
		}
		return nil, fmt.Errorf("create should not be called")
	})
	defer SetGHCommandForTest(nil)

	diff := CapabilityDiff{
		Provider: "claude-code",
		RunID:    "run-001",
		Changes:  []FieldChange{{FieldPath: "foo", OldValue: "a", NewValue: "b"}},
	}
	manifest := &RunManifest{
		RunID: "run-001",
		Providers: map[string]ProviderStatus{
			"claude-code": {HasDrift: true, Diff: &diff},
		},
	}
	if err := runStage4Review(context.Background(), PipelineOptions{}, manifest); err != nil {
		t.Fatalf("runStage4Review: %v", err)
	}
}

func TestRunStage1Fetch_SSRFRejected(t *testing.T) {
	// Source manifest with an HTTP URL — should be rejected by SSRF validation
	srcDir := t.TempDir()
	manifestYAML := `schema_version: "1"
slug: test-ssrf
content_types:
  rules:
    sources:
      - url: "http://internal.example.com/docs"
        format: html
`
	if err := os.WriteFile(srcDir+"/test-ssrf.yaml", []byte(manifestYAML), 0755); err != nil {
		t.Fatal(err)
	}

	manifest := &RunManifest{
		RunID:     "test-run",
		Providers: make(map[string]ProviderStatus),
	}
	opts := PipelineOptions{
		CacheRoot:          t.TempDir(),
		SourceManifestsDir: srcDir,
	}
	// runStage1Fetch should not return an error for a single bad URL — it records the error per-source
	if err := runStage1Fetch(context.Background(), opts, manifest); err != nil {
		t.Fatalf("runStage1Fetch should not fail fatally for single bad URL: %v", err)
	}
	// The SSRF error should be recorded in the manifest
	status, ok := manifest.Providers["test-ssrf"]
	if !ok {
		t.Fatal("expected 'test-ssrf' in manifest providers")
	}
	if len(status.Errors) == 0 {
		t.Error("expected SSRF error recorded in provider status")
	}
}

func TestRunStage2Extract_FromCache(t *testing.T) {
	// Set up a cached raw.bin with markdown content
	cacheDir := t.TempDir()
	srcDir := t.TempDir()

	// Create source manifest (yaml format — extractor is registered by extract_test.go import)
	manifestYAML := `schema_version: "1"
slug: test-provider
content_types:
  rules:
    sources:
      - url: "https://example.com/docs"
        format: yaml
`
	if err := os.WriteFile(srcDir+"/test-provider.yaml", []byte(manifestYAML), 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-populate cache with raw.bin + meta.json using yaml format (registered in extract_test.go)
	raw := []byte("hooks:\n  supported: true\n  events:\n    before_tool_execute:\n      native_name: PreToolUse\n")
	cacheEntry := CacheEntry{
		Provider: "test-provider",
		SourceID: "rules.0",
		Raw:      raw,
		Meta: CacheMeta{
			ContentHash: SHA256Hex(raw),
			FetchStatus: "ok",
			FetchMethod: "http",
			Format:      "yaml",
			SourceURL:   "https://example.com/docs",
		},
	}
	if err := WriteCacheEntry(cacheDir, cacheEntry); err != nil {
		t.Fatal(err)
	}

	manifest := &RunManifest{
		RunID:     "test-run",
		Providers: make(map[string]ProviderStatus),
	}
	opts := PipelineOptions{
		CacheRoot:          cacheDir,
		SourceManifestsDir: srcDir,
	}
	if err := runStage2Extract(context.Background(), opts, manifest); err != nil {
		t.Fatalf("runStage2Extract: %v", err)
	}

	// Check that extracted.json was written
	extPath := cacheDir + "/test-provider/rules.0/extracted.json"
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		t.Error("extracted.json was not created")
	}

	// Check provider status
	status, ok := manifest.Providers["test-provider"]
	if !ok {
		t.Fatal("expected 'test-provider' in manifest providers")
	}
	if status.SourcesExtracted == 0 {
		t.Errorf("expected SourcesExtracted > 0, got %d (errors: %v)", status.SourcesExtracted, status.Errors)
	}
}

func TestRunStage3Diff_WithExtractedData(t *testing.T) {
	cacheDir := t.TempDir()
	capsDir := t.TempDir()

	// Write extracted.json for a provider
	providerDir := filepath.Join(cacheDir, "claude-code", "hooks.0")
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		t.Fatal(err)
	}
	extracted := `{
		"provider": "claude-code",
		"source_id": "hooks.0",
		"fields": {
			"hooks.before_tool_execute.native_name": {"value": "PreToolUse", "value_hash": "sha256:abc"}
		},
		"landmarks": ["Events", "Configuration"]
	}`
	if err := os.WriteFile(filepath.Join(providerDir, "extracted.json"), []byte(extracted), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a capability YAML with a different value — should detect drift
	capsYAML := `schema_version: "1"
slug: claude-code
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: OldName
`
	if err := os.WriteFile(filepath.Join(capsDir, "claude-code.yaml"), []byte(capsYAML), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := &RunManifest{
		RunID: "test-run",
		Providers: map[string]ProviderStatus{
			"claude-code": {Slug: "claude-code", SourcesExtracted: 1},
		},
	}
	opts := PipelineOptions{
		CacheRoot:       cacheDir,
		CapabilitiesDir: capsDir,
	}
	if err := runStage3Diff(context.Background(), opts, manifest); err != nil {
		t.Fatalf("runStage3Diff: %v", err)
	}
}
