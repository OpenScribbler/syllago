package converter

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// markdownBasedFormats are the file formats that use frontmatter.
// JSON, YAML-standalone, and TOML-standalone formats are excluded.
var markdownBasedFormats = map[provider.Format]bool{
	provider.FormatMarkdown: true,
	provider.FormatMDC:      true,
	// FormatYAML is standalone — not frontmatter in a markdown file
	// FormatTOML, FormatJSON, FormatJSONC: no frontmatter
}

// TestFrontmatterRegistry_Completeness verifies that every (provider, contentType)
// pair where SupportsType=true AND FileFormat is markdown-based has a registered
// frontmatter struct.
//
// Exceptions: providers that render plain markdown with no frontmatter are valid
// zero-registration cases. Those are listed in noFrontmatterOK.
func TestFrontmatterRegistry_Completeness(t *testing.T) {
	t.Parallel()

	// Providers that legitimately render without frontmatter for a given content type.
	// Key: "slug/contentType"
	noFrontmatterOK := map[string]bool{
		// Rules: these providers render plain markdown, no frontmatter
		"gemini-cli/rules": true,
		"codex/rules":      true,
		"zed/rules":        true,
		"opencode/rules":   true,
		"roo-code/rules":   true,
		// Codex skills render plain markdown with no frontmatter
		"codex/skills": true,
		// Agents: codex agents use TOML (not frontmatter in markdown)
		"codex/agents": true,
		// Commands: cursor/cline/windsurf/roo-code commands are plain markdown
		"cursor/commands":   true,
		"cline/commands":    true,
		"windsurf/commands": true,
		"roo-code/commands": true,
		// factory-droid, pi, crush use plain AGENTS.md for rules and plain
		// markdown (no frontmatter) for commands/prompt templates.
		"factory-droid/rules":    true,
		"factory-droid/commands": true,
		"pi/rules":               true,
		"pi/commands":            true,
		"crush/rules":            true,
	}

	contentTypesWithFrontmatter := []catalog.ContentType{
		catalog.Rules, catalog.Skills, catalog.Agents, catalog.Commands,
	}

	for _, prov := range provider.AllProviders {
		for _, ct := range contentTypesWithFrontmatter {
			if prov.SupportsType == nil || !prov.SupportsType(ct) {
				continue
			}
			if prov.FileFormat == nil {
				continue
			}
			format := prov.FileFormat(ct)
			if !markdownBasedFormats[format] {
				continue
			}

			key := prov.Slug + "/" + string(ct)
			if noFrontmatterOK[key] {
				continue
			}

			fields := FrontmatterFieldsFor(ct, prov.Slug)
			if fields == nil {
				t.Errorf("provider %q supports %s (format=%s) but no frontmatter struct registered",
					prov.Slug, ct, format)
			}
		}
	}
}

// TestFrontmatterRegistry_FieldAccuracy asserts specific expected fields for
// known (provider, contentType) pairs. This catches struct field renames and
// tag changes that would silently change the manifest output.
func TestFrontmatterRegistry_FieldAccuracy(t *testing.T) {
	t.Parallel()

	type check struct {
		ct     catalog.ContentType
		slug   string
		want   []string // all must be present
		absent []string // none must be present
	}

	checks := []check{
		// Rules
		{catalog.Rules, "claude-code", []string{"paths"}, []string{"description", "alwaysApply"}},
		{catalog.Rules, "cursor", []string{"description", "alwaysApply", "globs"}, nil},
		{catalog.Rules, "windsurf", []string{"trigger", "description", "globs"}, nil},
		{catalog.Rules, "kiro", []string{"inclusion", "fileMatchPattern", "name", "description"}, nil},
		{catalog.Rules, "copilot-cli", []string{"applyTo"}, nil},
		{catalog.Rules, "cline", []string{"paths"}, nil},
		{catalog.Rules, "amp", []string{"globs"}, nil},

		// Skills
		{catalog.Skills, "claude-code", []string{"name", "description", "license", "allowed-tools", "disallowed-tools", "context", "agent", "model", "effort", "disable-model-invocation", "user-invocable", "argument-hint", "hooks"}, nil},
		{catalog.Skills, "cursor", []string{"name", "description", "license", "compatibility", "metadata", "disable-model-invocation"}, []string{"allowed-tools", "hooks"}},
		{catalog.Skills, "copilot-cli", []string{"name", "description", "license", "argument-hint", "user-invocable", "disable-model-invocation"}, nil},
		{catalog.Skills, "kiro", []string{"name", "description", "license", "compatibility", "metadata"}, nil},
		{catalog.Skills, "opencode", []string{"name", "description", "license", "compatibility", "metadata"}, nil},
		{catalog.Skills, "gemini-cli", []string{"name", "description"}, nil},
		// NOTE: Expand these minimal assertions after inspecting the actual struct tags.
		// These four are registered in 5b but were missing from the original checks table.
		{catalog.Skills, "windsurf", []string{"name", "description"}, nil},
		{catalog.Skills, "amp", []string{"name", "description"}, nil},
		{catalog.Skills, "cline", []string{"name", "description"}, nil},
		{catalog.Skills, "roo-code", []string{"name", "description"}, nil},

		// Agents
		{catalog.Agents, "claude-code", []string{"name", "description", "tools", "disallowedTools", "model", "maxTurns", "permissionMode", "skills", "mcpServers", "memory", "background", "isolation", "effort", "hooks", "color"}, nil},
		{catalog.Agents, "cursor", []string{"name", "description", "model", "readonly", "is_background"}, nil},
		{catalog.Agents, "gemini-cli", []string{"name", "description", "tools", "model", "max_turns", "temperature", "timeout_mins", "kind"}, nil},
		{catalog.Agents, "copilot-cli", []string{"name", "description", "tools", "model", "target", "mcp-servers"}, nil},
		{catalog.Agents, "opencode", []string{"name", "description", "tools", "model", "steps", "color", "temperature"}, nil},
		{catalog.Agents, "kiro", []string{"name", "description", "model", "tools", "mcpServers"}, nil},
		{catalog.Agents, "roo-code", []string{"slug", "name", "roleDefinition", "whenToUse", "groups"}, nil},

		// Commands
		{catalog.Commands, "claude-code", []string{"name", "description", "allowed-tools", "context", "agent", "model", "disable-model-invocation", "user-invocable", "argument-hint", "effort"}, nil},
		{catalog.Commands, "copilot-cli", []string{"name", "description", "agent", "model", "tools", "argument-hint"}, nil},
		{catalog.Commands, "codex", []string{"description", "argument-hint"}, nil},
		{catalog.Commands, "opencode", []string{"description", "agent", "model", "subtask"}, nil},
		{catalog.Commands, "gemini-cli", []string{"name", "description"}, nil},
	}

	for _, c := range checks {
		c := c
		t.Run(string(c.ct)+"/"+c.slug, func(t *testing.T) {
			t.Parallel()
			fields := FrontmatterFieldsFor(c.ct, c.slug)
			if fields == nil {
				t.Fatalf("%s/%s: no frontmatter registered", c.ct, c.slug)
			}
			fieldSet := make(map[string]bool, len(fields))
			for _, f := range fields {
				fieldSet[f] = true
			}
			for _, want := range c.want {
				if !fieldSet[want] {
					t.Errorf("%s/%s: field %q missing from registered fields %v", c.ct, c.slug, want, fields)
				}
			}
			for _, absent := range c.absent {
				if fieldSet[absent] {
					t.Errorf("%s/%s: field %q should not be in registered fields %v", c.ct, c.slug, absent, fields)
				}
			}
		})
	}
}

// TestFrontmatterRegistry_RegisterFrontmatter_PanicOnNonStruct verifies that
// passing a non-struct to RegisterFrontmatter panics immediately (fail-fast at init).
func TestFrontmatterRegistry_RegisterFrontmatter_PanicOnNonStruct(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-struct input, got none")
		}
	}()

	RegisterFrontmatter(catalog.Rules, "test-provider", 42)
}

// TestFrontmatterRegistry_RegisterFrontmatter_PointerAccepted verifies that
// passing a pointer to a struct works (pointer is dereferenced).
func TestFrontmatterRegistry_RegisterFrontmatter_PointerAccepted(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Field1 string `yaml:"field_one"`
		Field2 int    `yaml:"field_two,omitempty"`
	}

	// Should not panic
	RegisterFrontmatter(catalog.Rules, "test-ptr-provider", &testStruct{})
	fields := FrontmatterFieldsFor(catalog.Rules, "test-ptr-provider")
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d: %v", len(fields), fields)
	}
	if fields[0] != "field_one" {
		t.Errorf("fields[0] = %q, want %q", fields[0], "field_one")
	}
	if fields[1] != "field_two" {
		t.Errorf("fields[1] = %q, want %q", fields[1], "field_two")
	}
}

// TestFrontmatterRegistry_SkipDashTag verifies fields tagged yaml:"-" are excluded.
func TestFrontmatterRegistry_SkipDashTag(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Visible string `yaml:"visible"`
		Skipped string `yaml:"-"`
	}

	RegisterFrontmatter(catalog.Rules, "test-dash-provider", testStruct{})
	fields := FrontmatterFieldsFor(catalog.Rules, "test-dash-provider")
	for _, f := range fields {
		if f == "-" || f == "Skipped" {
			t.Errorf("field tagged yaml:\"-\" should not appear in registered fields, got %q", f)
		}
	}
}

// TestFrontmatterRegistry_FrontmatterFieldsFor_UnregisteredReturnsNil verifies
// that looking up an unregistered (ct, slug) returns nil, not an error.
func TestFrontmatterRegistry_FrontmatterFieldsFor_UnregisteredReturnsNil(t *testing.T) {
	t.Parallel()

	result := FrontmatterFieldsFor(catalog.Rules, "no-such-provider-xyz")
	if result != nil {
		t.Errorf("expected nil for unregistered provider, got %v", result)
	}
}
