package capmon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestGenerateHooksSpecTables_BasicOutput(t *testing.T) {
	capsDir := t.TempDir()
	specDir := t.TempDir()

	// Write a minimal provider YAML
	yamlContent := `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: PreToolUse
        blocking: "true"
      after_tool_execute:
        native_name: PostToolUse
        blocking: "false"
    tools:
      shell:
        native: BashTool
`
	if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML fixture: %v", err)
	}

	// Write a markdown file with sentinel markers
	mdContent := "# Events\n\nSome prose here.\n\n" +
		capmon.GeneratedBannerStart + "\n" +
		"old content\n" +
		capmon.GeneratedBannerEnd + "\n\n" +
		"More prose after.\n"
	mdPath := filepath.Join(specDir, "events.md")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}

	if err := capmon.GenerateHooksSpecTables(capsDir, specDir); err != nil {
		t.Fatalf("GenerateHooksSpecTables: %v", err)
	}

	out, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	body := string(out)

	// Sentinels must be preserved
	if !strings.Contains(body, capmon.GeneratedBannerStart) {
		t.Error("output must contain start sentinel")
	}
	if !strings.Contains(body, capmon.GeneratedBannerEnd) {
		t.Error("output must contain end sentinel")
	}
	// Provider slug must appear in generated table
	if !strings.Contains(body, "test-provider") {
		t.Error("provider slug must appear in generated table")
	}
	// Native event name must appear
	if !strings.Contains(body, "PreToolUse") {
		t.Error("native event name must appear in generated table")
	}
	// Prose outside the sentinels must survive untouched
	if !strings.Contains(body, "Some prose here.") {
		t.Error("prose before sentinel must survive")
	}
	if !strings.Contains(body, "More prose after.") {
		t.Error("prose after sentinel must survive")
	}
	// Old content must be replaced
	if strings.Contains(body, "old content") {
		t.Error("old generated content must be replaced")
	}
}

func TestGenerateHooksSpecTables_MissingBanner(t *testing.T) {
	capsDir := t.TempDir()
	specDir := t.TempDir()

	// Markdown file with NO sentinels
	mdPath := filepath.Join(specDir, "no-sentinels.md")
	if err := os.WriteFile(mdPath, []byte("# Just prose\n\nNo markers here.\n"), 0644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}

	// No provider YAML needed — the function should skip files without markers
	if err := capmon.GenerateHooksSpecTables(capsDir, specDir); err != nil {
		t.Errorf("GenerateHooksSpecTables should succeed (skip files with no markers), got: %v", err)
	}

	// File content must be unchanged
	out, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(out) != "# Just prose\n\nNo markers here.\n" {
		t.Error("file without sentinels must not be modified")
	}
}

func TestReplaceGeneratedSection_MissingEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.md")
	content := capmon.GeneratedBannerStart + "\nsome content\n" // no end marker
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := capmon.ReplaceGeneratedSection(path, "new content")
	if err == nil {
		t.Error("expected error when end marker is missing")
	}
}

func TestGenerateHooksSpecTables_MissingEndMarker(t *testing.T) {
	capsDir := t.TempDir()
	specDir := t.TempDir()

	// Write a minimal provider YAML
	yamlContent := "schema_version: \"1\"\nslug: test-provider\ncontent_types:\n  hooks:\n    supported: true\n    events:\n      before_tool_execute:\n        native_name: PreToolUse\n        blocking: \"true\"\n"
	if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	// Markdown file with start but no end marker
	brokenMD := "# Events\n" + capmon.GeneratedBannerStart + "\nsome content\n"
	if err := os.WriteFile(filepath.Join(specDir, "events.md"), []byte(brokenMD), 0644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	err := capmon.GenerateHooksSpecTables(capsDir, specDir)
	if err == nil {
		t.Error("expected error when end marker is missing in spec file")
	}
}

// TestGenerateHooksSpecTables_SkipsSeedYAMLs ensures that per-content-type
// seed YAMLs (which use top-level `provider:` instead of `slug:`) are skipped
// instead of producing empty columns in the generated table.
func TestGenerateHooksSpecTables_SkipsSeedYAMLs(t *testing.T) {
	capsDir := t.TempDir()
	specDir := t.TempDir()

	// Real capability YAML with a slug.
	realYAML := `schema_version: "1"
slug: real-provider
display_name: Real
content_types:
  hooks:
    supported: true
    events:
      before_tool_execute:
        native_name: PreToolUse
`
	if err := os.WriteFile(filepath.Join(capsDir, "real-provider.yaml"), []byte(realYAML), 0644); err != nil {
		t.Fatalf("write real: %v", err)
	}

	// Per-content-type seed YAML — uses `provider:` not `slug:`, so it parses
	// with empty Slug. Must be skipped, not silently included with a blank column.
	seedYAML := `provider: real-provider
content_type: skills
proposed_mappings: []
`
	if err := os.WriteFile(filepath.Join(capsDir, "real-provider-skills.yaml"), []byte(seedYAML), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	mdPath := filepath.Join(specDir, "events.md")
	mdContent := "# Events\n\n" + capmon.GeneratedBannerStart + "\n\n" + capmon.GeneratedBannerEnd + "\n"
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	if err := capmon.GenerateHooksSpecTables(capsDir, specDir); err != nil {
		t.Fatalf("GenerateHooksSpecTables: %v", err)
	}

	out, _ := os.ReadFile(mdPath)
	body := string(out)
	// Real provider must appear exactly once.
	if strings.Count(body, "real-provider") != 1 {
		t.Errorf("real-provider must appear once in generated table, body:\n%s", body)
	}
	// No empty-slug column (would render as " | |" with nothing between pipes).
	if strings.Contains(body, "|  |") {
		t.Errorf("generated table contains an empty-slug column, body:\n%s", body)
	}
}

func TestReplaceGeneratedSection_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	original := "before\n" + capmon.GeneratedBannerStart + "\nold\n" + capmon.GeneratedBannerEnd + "\nafter\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := capmon.ReplaceGeneratedSection(path, "new-content"); err != nil {
		t.Fatalf("ReplaceGeneratedSection: %v", err)
	}
	out, _ := os.ReadFile(path)
	body := string(out)
	if !strings.Contains(body, "before\n") {
		t.Error("content before sentinel must survive")
	}
	if !strings.Contains(body, "\nafter\n") {
		t.Error("content after sentinel must survive")
	}
	if !strings.Contains(body, "new-content") {
		t.Error("new content must be present")
	}
	if strings.Contains(body, "old") {
		t.Error("old content must be replaced")
	}
}
