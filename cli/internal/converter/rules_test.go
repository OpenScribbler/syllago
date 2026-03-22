package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Cursor → canonical → Windsurf ---

func TestCursorAlwaysApplyToWindsurf(t *testing.T) {
	input := []byte("---\ndescription: \"Always on rule\"\nalwaysApply: true\n---\n\nDo the thing.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "trigger: always_on")
	assertContains(t, out, "description: Always on rule")
	assertContains(t, out, "Do the thing.")
	assertEqual(t, "rule.md", result.Filename)
}

func TestCursorGlobsToWindsurf(t *testing.T) {
	input := []byte("---\ndescription: \"TS rule\"\nglobs:\n  - \"*.ts\"\n  - \"*.tsx\"\nalwaysApply: false\n---\n\nUse strict.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "trigger: glob")
	assertContains(t, out, "*.ts")
	assertContains(t, out, "Use strict.")
}

func TestCursorModelDecisionToWindsurf(t *testing.T) {
	input := []byte("---\ndescription: \"Apply when writing tests\"\nalwaysApply: false\n---\n\nTest conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "trigger: model_decision")
	assertContains(t, out, "description: Apply when writing tests")
}

func TestCursorManualToWindsurf(t *testing.T) {
	input := []byte("---\nalwaysApply: false\n---\n\nManual rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "trigger: manual")
}

// --- Windsurf → canonical → Cursor ---

func TestWindsurfAlwaysOnToCursor(t *testing.T) {
	input := []byte("---\ntrigger: always_on\ndescription: \"Global rule\"\n---\n\nGlobal content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: true")
	assertContains(t, out, "description: Global rule")
	assertEqual(t, "rule.mdc", result.Filename)
}

func TestWindsurfGlobToCursor(t *testing.T) {
	input := []byte("---\ntrigger: glob\nglobs: \"*.ts, *.tsx\"\ndescription: \"TS files\"\n---\n\nTS content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: false")
	assertContains(t, out, "*.ts")
	assertContains(t, out, "*.tsx")
}

func TestWindsurfManualToCursor(t *testing.T) {
	input := []byte("---\ntrigger: manual\n---\n\nManual rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: false")
	assertNotContains(t, out, "globs:")
}

// --- Cursor → canonical → Claude (single-file) ---

func TestCursorAlwaysApplyToClaude(t *testing.T) {
	input := []byte("---\ndescription: \"Global rule\"\nalwaysApply: true\n---\n\nAlways active content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result.Content == nil {
		t.Fatal("expected content for alwaysApply:true rule, got nil")
	}
	out := string(result.Content)
	assertContains(t, out, "Always active content.")
	// Single-file format should NOT have frontmatter
	assertNotContains(t, out, "---")
}

func TestCursorNotAlwaysApplyEmbedsScopeToClaude(t *testing.T) {
	input := []byte("---\ndescription: \"Conditional rule\"\nalwaysApply: false\n---\n\nConditional content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result.Content == nil {
		t.Fatal("expected content with embedded scope, got nil")
	}
	out := string(result.Content)
	assertContains(t, out, "Conditional content.")
	assertContains(t, out, "**Scope:** Apply when: Conditional rule")
	assertContains(t, out, "syllago:converted")
}

// --- Round-trip tests ---

func TestCanonicalRoundTrip(t *testing.T) {
	// Canonical → Windsurf → Canonical should preserve semantics
	original := "---\ndescription: Round-trip test\nalwaysApply: true\nglobs:\n    - \"*.go\"\n---\n\nGo content.\n"

	conv := &RulesConverter{}
	windsurfResult, err := conv.Render([]byte(original), provider.Windsurf)
	if err != nil {
		t.Fatalf("Render to Windsurf: %v", err)
	}

	backToCanonical, err := conv.Canonicalize(windsurfResult.Content, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize from Windsurf: %v", err)
	}

	meta, body, err := parseCanonical(backToCanonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true after round-trip")
	}
	if meta.Description != "Round-trip test" {
		t.Errorf("expected description 'Round-trip test', got %q", meta.Description)
	}
	assertContains(t, body, "Go content.")
}

// --- Missing frontmatter defaults ---

func TestMissingFrontmatterDefaultsToAlwaysApply(t *testing.T) {
	input := []byte("# Plain Rule\n\nJust some content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true for plain markdown")
	}
	assertContains(t, body, "Just some content.")
}

func TestPlainMarkdownCanonicalize(t *testing.T) {
	input := []byte("# Some Rule\n\nContent here.\n")

	conv := &RulesConverter{}
	result, err := conv.Canonicalize(input, "generic")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseCanonical(result.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true for plain markdown with unknown provider")
	}
}

// --- Warning generation ---

func TestGlobRuleToCodexEmbedsScopeAsGlobs(t *testing.T) {
	input := []byte("---\ndescription: \"Scoped rule\"\nglobs:\n  - \"*.py\"\nalwaysApply: false\n---\n\nPython rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result.Content == nil {
		t.Fatal("expected content with embedded scope, got nil")
	}
	out := string(result.Content)
	assertContains(t, out, "Python rule.")
	assertContains(t, out, "**Scope:** Apply only when working with files matching: *.py")
}

func TestBareNonAlwaysApplyEmbedsScopeAsExplicit(t *testing.T) {
	input := []byte("---\nalwaysApply: false\n---\n\nManual rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result.Content == nil {
		t.Fatal("expected content with embedded scope, got nil")
	}
	out := string(result.Content)
	assertContains(t, out, "Manual rule.")
	assertContains(t, out, "**Scope:** Apply only when explicitly asked.")
}

// --- Windsurf model_decision ---

func TestWindsurfModelDecisionToCursor(t *testing.T) {
	input := []byte("---\ntrigger: model_decision\ndescription: \"Use when refactoring\"\n---\n\nRefactoring guide.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: false")
	assertContains(t, out, "description: Use when refactoring")
	assertNotContains(t, out, "globs:")
}

// --- Cline rules ---

func TestClineRuleRender(t *testing.T) {
	// Build canonical input with globs directly (skipping a source provider)
	input := []byte("---\ndescription: TypeScript rule\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nUse strict TypeScript.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Cline)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "paths:")
	assertNotContains(t, out, "globs:")
	assertContains(t, out, "*.ts")
	assertContains(t, out, "*.tsx")
	assertContains(t, out, "Use strict TypeScript.")
}

func TestClineRuleCanonicalize(t *testing.T) {
	input := []byte("---\npaths:\n    - \"*.go\"\n    - \"*.mod\"\n---\n\nGo conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "globs:")
	assertNotContains(t, out, "paths:")
	assertContains(t, out, "*.go")
	assertContains(t, out, "Go conventions.")
}

// --- Kiro rules ---

func TestKiroRuleRender(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nAlways follow these guidelines.\n")
	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertContains(t, string(result.Content), "Always follow")
	assertNotContains(t, string(result.Content), "---") // no frontmatter
}

func TestKiroRuleScopedEmbedsProse(t *testing.T) {
	input := []byte("---\ndescription: TS files\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n---\n\nTypeScript rule.\n")
	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "TypeScript rule.")
	assertContains(t, out, "**Scope:**")
	assertContains(t, out, "*.ts")
}

// --- Roo Code rules ---

func TestRooCodeRuleRender(t *testing.T) {
	input := []byte("---\ndescription: Go conventions\nalwaysApply: true\n---\n\nUse gofmt.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Roo Code rules are plain markdown with description as HTML comment
	assertContains(t, out, "<!-- Go conventions -->")
	assertContains(t, out, "Use gofmt.")
	// Should not have YAML frontmatter
	assertNotContains(t, out, "---")
	assertNotContains(t, out, "alwaysApply")

	// Filename should be slugified from description
	assertEqual(t, "go-conventions.md", result.Filename)
}

func TestRooCodeRuleGlobsWarning(t *testing.T) {
	input := []byte("---\ndescription: TS rule\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n---\n\nUse strict.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about glob scoping not supported by Roo Code")
	}
	assertContains(t, result.Warnings[0], "glob")
}

func TestRooCodeRuleNoDescription(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nPlain rule content.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Plain rule content.")
	assertNotContains(t, out, "<!--")
	assertEqual(t, "rule.md", result.Filename)
}

// --- Zed rules ---

func TestZedRuleRender(t *testing.T) {
	input := []byte("---\ndescription: Project guide\nalwaysApply: true\n---\n\nFollow the guide.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Zed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "<!-- Project guide -->")
	assertContains(t, out, "Follow the guide.")
	assertEqual(t, ".rules", result.Filename)
}

func TestZedRuleGlobsWarning(t *testing.T) {
	input := []byte("---\ndescription: Scoped rule\nalwaysApply: false\nglobs:\n    - \"*.py\"\n---\n\nPython rule.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Zed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about glob scoping not supported by Zed")
	}
}

// --- Cursor globs format ---

func TestCursorRenderGlobsAsCommaSeparatedString(t *testing.T) {
	input := []byte("---\ndescription: TS rule\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nUse strict.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should be a comma-separated string, not a YAML array
	assertContains(t, out, "globs: '*.ts, *.tsx'")
	assertNotContains(t, out, "- \"*.ts\"")
	assertContains(t, out, "alwaysApply: false")
	assertContains(t, out, "Use strict.")
}

func TestCursorRoundTripGlobs(t *testing.T) {
	// Start with a Cursor rule using comma-separated globs (native format)
	input := []byte("---\ndescription: TS rule\nglobs: \"*.ts, *.tsx\"\nalwaysApply: false\n---\n\nUse strict.\n")

	conv := &RulesConverter{}
	// Cursor → canonical
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify canonical has globs as array
	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}
	if len(meta.Globs) != 2 {
		t.Fatalf("expected 2 globs, got %d: %v", len(meta.Globs), meta.Globs)
	}
	assertEqual(t, "*.ts", meta.Globs[0])
	assertEqual(t, "*.tsx", meta.Globs[1])
	assertContains(t, body, "Use strict.")

	// canonical → Cursor (should produce comma-separated string)
	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "globs: '*.ts, *.tsx'")
	assertNotContains(t, out, "- \"*.ts\"")
	assertEqual(t, "rule.mdc", result.Filename)
}

func TestCursorRenderNoGlobsOmitted(t *testing.T) {
	input := []byte("---\ndescription: Always rule\nalwaysApply: true\n---\n\nAlways on.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: true")
	assertNotContains(t, out, "globs:")
}

// --- Helpers ---

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", needle, haystack)
	}
}

func assertEqual(t *testing.T, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}
