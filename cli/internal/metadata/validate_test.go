package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- ValidationError.String() tests ---

func TestValidationError_String_WithField(t *testing.T) {
	t.Parallel()
	e := ValidationError{Field: "README.md", Message: "Missing"}
	got := e.String()
	if got != "README.md: Missing" {
		t.Errorf("String() = %q, want %q", got, "README.md: Missing")
	}
}

func TestValidationError_String_WithoutField(t *testing.T) {
	t.Parallel()
	e := ValidationError{Message: "Something went wrong"}
	got := e.String()
	if got != "Something went wrong" {
		t.Errorf("String() = %q, want %q", got, "Something went wrong")
	}
}

// --- Validate tests ---

func TestValidate_ValidItem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")
	os.MkdirAll(itemDir, 0755)

	// Write valid .syllago.yaml
	meta := &Meta{ID: NewID(), Name: "test-item", Type: "rules"}
	Save(itemDir, meta)

	// Write README.md
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# Test"), 0644)

	errs := Validate(itemDir, "rules", dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid item, got %d: %v", len(errs), errs)
	}
}

func TestValidate_MissingMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")
	os.MkdirAll(itemDir, 0755)

	errs := Validate(itemDir, "rules", dir)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Message, "Missing") && e.Field == FileName {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing %s, got: %v", FileName, errs)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")
	os.MkdirAll(itemDir, 0755)

	// Write .syllago.yaml with missing required fields
	os.WriteFile(filepath.Join(itemDir, FileName), []byte("description: only this\n"), 0644)

	errs := Validate(itemDir, "rules", dir)

	// Should have errors for missing id, name, type + README
	fieldErrors := map[string]bool{}
	for _, e := range errs {
		if strings.Contains(e.Message, "Missing required field: id") {
			fieldErrors["id"] = true
		}
		if strings.Contains(e.Message, "Missing required field: name") {
			fieldErrors["name"] = true
		}
		if strings.Contains(e.Message, "Missing required field: type") {
			fieldErrors["type"] = true
		}
	}
	for _, field := range []string{"id", "name", "type"} {
		if !fieldErrors[field] {
			t.Errorf("expected error about missing %s field", field)
		}
	}
}

func TestValidate_MissingReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-item", Type: "rules"}
	Save(itemDir, meta)
	// No README.md

	errs := Validate(itemDir, "rules", dir)
	found := false
	for _, e := range errs {
		if e.Field == "README.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about missing README.md, got: %v", errs)
	}
}

func TestValidate_Skills_RequiresFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-skill", Type: "skills"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# Skill"), 0644)

	// No SKILL.md
	errs := Validate(itemDir, "skills", dir)
	found := false
	for _, e := range errs {
		if e.Field == "SKILL.md" && strings.Contains(e.Message, "Missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing SKILL.md, got: %v", errs)
	}
}

func TestValidate_Skills_ValidFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-skill", Type: "skills"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# Skill"), 0644)
	os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("---\nname: test\ndescription: a skill\n---\nBody\n"), 0644)

	errs := Validate(itemDir, "skills", dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid skill, got: %v", errs)
	}
}

func TestValidate_Skills_MissingFrontmatterFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-skill", Type: "skills"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# Skill"), 0644)
	// SKILL.md with frontmatter but missing "description"
	os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("---\nname: test\n---\nBody\n"), 0644)

	errs := Validate(itemDir, "skills", dir)
	found := false
	for _, e := range errs {
		if e.Field == "SKILL.md" && strings.Contains(e.Message, "description") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing description in SKILL.md frontmatter, got: %v", errs)
	}
}

func TestValidate_Agents_RequiresFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-agent")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-agent", Type: "agents"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# Agent"), 0644)

	// No AGENT.md
	errs := Validate(itemDir, "agents", dir)
	found := false
	for _, e := range errs {
		if e.Field == "AGENT.md" && strings.Contains(e.Message, "Missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing AGENT.md, got: %v", errs)
	}
}

func TestValidate_MCP_MissingConfigJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-mcp")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-mcp", Type: "mcp"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# MCP"), 0644)

	// No config.json
	errs := Validate(itemDir, "mcp", dir)
	found := false
	for _, e := range errs {
		if e.Field == "config.json" && strings.Contains(e.Message, "Missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about missing config.json, got: %v", errs)
	}
}

func TestValidate_MCP_ValidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-mcp")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-mcp", Type: "mcp"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# MCP"), 0644)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{"name": "test"}`), 0644)

	errs := Validate(itemDir, "mcp", dir)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid MCP item, got: %v", errs)
	}
}

func TestValidate_MCP_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-mcp")
	os.MkdirAll(itemDir, 0755)

	meta := &Meta{ID: NewID(), Name: "test-mcp", Type: "mcp"}
	Save(itemDir, meta)
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte("# MCP"), 0644)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(`{invalid json`), 0644)

	errs := Validate(itemDir, "mcp", dir)
	found := false
	for _, e := range errs {
		if e.Field == "config.json" && strings.Contains(e.Message, "Invalid JSON") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error about invalid JSON, got: %v", errs)
	}
}

// --- parseFrontmatterFields tests ---

func TestParseFrontmatterFields_Valid(t *testing.T) {
	t.Parallel()
	data := []byte("---\nname: my-skill\ndescription: does things\n---\n# Body\n")
	fm := parseFrontmatterFields(data)
	if fm == nil {
		t.Fatal("expected non-nil result")
	}
	if fm["name"] != "my-skill" {
		t.Errorf("name = %q, want %q", fm["name"], "my-skill")
	}
	if fm["description"] != "does things" {
		t.Errorf("description = %q, want %q", fm["description"], "does things")
	}
}

func TestParseFrontmatterFields_NoFrontmatter(t *testing.T) {
	t.Parallel()
	data := []byte("# Just a heading\nSome content\n")
	fm := parseFrontmatterFields(data)
	if fm != nil {
		t.Errorf("expected nil for no frontmatter, got %v", fm)
	}
}

func TestParseFrontmatterFields_NoClosingDelimiter(t *testing.T) {
	t.Parallel()
	data := []byte("---\nname: test\n# No closing delimiter\n")
	fm := parseFrontmatterFields(data)
	if fm != nil {
		t.Errorf("expected nil for unclosed frontmatter, got %v", fm)
	}
}

func TestParseFrontmatterFields_WindowsLineEndings(t *testing.T) {
	t.Parallel()
	data := []byte("---\r\nname: test\r\ndescription: foo\r\n---\r\nBody\r\n")
	fm := parseFrontmatterFields(data)
	if fm == nil {
		t.Fatal("expected non-nil result for CRLF content")
	}
	if fm["name"] != "test" {
		t.Errorf("name = %q, want %q", fm["name"], "test")
	}
}

func TestParseFrontmatterFields_InvalidYAML(t *testing.T) {
	t.Parallel()
	data := []byte("---\n: invalid: yaml: here\n---\nBody\n")
	fm := parseFrontmatterFields(data)
	// Invalid YAML should return nil
	if fm != nil {
		t.Errorf("expected nil for invalid YAML, got %v", fm)
	}
}

func TestParseFrontmatterFields_NonStringValues(t *testing.T) {
	t.Parallel()
	data := []byte("---\nname: test\ncount: 42\nenabled: true\n---\nBody\n")
	fm := parseFrontmatterFields(data)
	if fm == nil {
		t.Fatal("expected non-nil result")
	}
	// Only string values should be in the map
	if fm["name"] != "test" {
		t.Errorf("name = %q, want %q", fm["name"], "test")
	}
	// Non-string values should be empty (not present)
	if fm["count"] != "" {
		t.Errorf("count should be empty for non-string value, got %q", fm["count"])
	}
}
