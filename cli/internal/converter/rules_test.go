package converter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

// --- Kiro rules: canonicalize ---

func TestKiroCanonicalizeFileMatch(t *testing.T) {
	input := []byte("---\ninclusion: fileMatch\nfileMatchPattern: \"*.ts,*.tsx\"\ndescription: TypeScript files\n---\n\nUse strict TypeScript.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if meta.AlwaysApply {
		t.Error("expected alwaysApply:false for fileMatch rule")
	}
	if len(meta.Globs) != 2 {
		t.Fatalf("expected 2 globs, got %d: %v", len(meta.Globs), meta.Globs)
	}
	assertEqual(t, "*.ts", meta.Globs[0])
	assertEqual(t, "*.tsx", meta.Globs[1])
	assertEqual(t, "TypeScript files", meta.Description)
	assertContains(t, body, "Use strict TypeScript.")
}

func TestKiroCanonicalizeAlways(t *testing.T) {
	input := []byte("---\ninclusion: always\ndescription: Global guidelines\n---\n\nAlways follow these.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true for inclusion:always")
	}
	assertEqual(t, "Global guidelines", meta.Description)
	assertContains(t, body, "Always follow these.")
}

func TestKiroCanonicalizeNoFrontmatter(t *testing.T) {
	input := []byte("# Plain Kiro Rule\n\nJust some content.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true for rule without frontmatter")
	}
	assertContains(t, body, "Just some content.")
}

// --- Kiro rules: render ---

func TestKiroRenderAlwaysApply(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nAlways follow these guidelines.\n")
	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "inclusion: always")
	assertContains(t, out, "Always follow these guidelines.")
}

func TestKiroRenderFileMatch(t *testing.T) {
	input := []byte("---\ndescription: TS files\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nTypeScript rule.\n")
	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "inclusion: fileMatch")
	assertContains(t, out, "fileMatchPattern: '*.ts,*.tsx'")
	assertContains(t, out, "description: TS files")
	assertContains(t, out, "TypeScript rule.")
}

func TestKiroRenderAuto(t *testing.T) {
	// Non-alwaysApply, no globs → auto inclusion
	input := []byte("---\ndescription: Apply when refactoring\nalwaysApply: false\n---\n\nRefactoring guide.\n")
	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "inclusion: auto")
	assertContains(t, out, "description: Apply when refactoring")
	assertContains(t, out, "Refactoring guide.")
}

// --- Kiro round-trip ---

func TestKiroRoundTripFileMatch(t *testing.T) {
	// Kiro fileMatch rule → canonical → Kiro should preserve semantics
	input := []byte("---\ninclusion: fileMatch\nfileMatchPattern: \"*.go\"\ndescription: Go files\n---\n\nGo conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "inclusion: fileMatch")
	assertContains(t, out, "fileMatchPattern: '*.go'")
	assertContains(t, out, "description: Go files")
	assertContains(t, out, "Go conventions.")
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

// --- Amp Rules ---

func TestCanonicalizeAmpRule(t *testing.T) {
	input := []byte("---\nglobs:\n  - \"*.go\"\n  - \"*.rs\"\n---\n\nFollow Go and Rust best practices.\n")

	conv := &RulesConverter{}
	result, err := conv.Canonicalize(input, "amp")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "*.go")
	assertContains(t, out, "*.rs")
	assertContains(t, out, "Follow Go and Rust best practices.")
}

func TestCanonicalizeAmpRuleNoFrontmatter(t *testing.T) {
	input := []byte("Always follow security best practices.\n")

	conv := &RulesConverter{}
	result, err := conv.Canonicalize(input, "amp")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: true")
	assertContains(t, out, "Always follow security best practices.")
}

func TestCanonicalizeAmpRuleNoGlobs(t *testing.T) {
	// Frontmatter without globs → alwaysApply
	input := []byte("---\nsome_unknown_field: true\n---\n\nGeneral rule.\n")

	conv := &RulesConverter{}
	result, err := conv.Canonicalize(input, "amp")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: true")
}

func TestRenderAmpRule(t *testing.T) {
	input := []byte("---\nglobs:\n  - \"**/*.go\"\nalwaysApply: false\n---\n\nFollow Go conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Amp)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Amp strips **/ prefix from globs (it adds it implicitly)
	assertContains(t, out, "*.go")
	assertContains(t, out, "Follow Go conventions.")
	assertEqual(t, "AGENTS.md", result.Filename)
}

func TestRenderAmpRuleAlwaysApply(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nAlways do this.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Amp)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// AlwaysApply → plain markdown, no frontmatter
	assertNotContains(t, out, "---")
	assertContains(t, out, "Always do this.")
	assertEqual(t, "AGENTS.md", result.Filename)
}

func TestRenderAmpRuleDescriptionScope(t *testing.T) {
	input := []byte("---\ndescription: When working with tests\nalwaysApply: false\n---\n\nFollow test patterns.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Amp)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Apply when: When working with tests")
	assertEqual(t, "AGENTS.md", result.Filename)
}

func TestStripImplicitGlobPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**/*.go", "*.go"},
		{"*.go", "*.go"},
		{"../*.go", "../*.go"},
		{"./*.go", "./*.go"},
	}

	for _, tt := range tests {
		result := stripImplicitGlobPrefix(tt.input)
		assertEqual(t, tt.expected, result)
	}
}

// --- Copilot rules: canonicalize ---

func TestCopilotCanonicalizeApplyTo(t *testing.T) {
	input := []byte("---\napplyTo: \"*.ts, *.tsx\"\n---\n\nUse strict TypeScript.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if meta.AlwaysApply {
		t.Error("expected alwaysApply:false for applyTo rule")
	}
	if len(meta.Globs) != 2 {
		t.Fatalf("expected 2 globs, got %d: %v", len(meta.Globs), meta.Globs)
	}
	assertEqual(t, "*.ts", meta.Globs[0])
	assertEqual(t, "*.tsx", meta.Globs[1])
	assertContains(t, body, "Use strict TypeScript.")
}

func TestCopilotCanonicalizeNoFrontmatter(t *testing.T) {
	input := []byte("# Global Copilot Instructions\n\nAlways follow these rules.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true for copilot rule without frontmatter")
	}
	assertContains(t, body, "Always follow these rules.")
}

func TestCopilotCanonicalizeEmptyApplyTo(t *testing.T) {
	// applyTo present but empty → alwaysApply
	input := []byte("---\napplyTo: \"\"\n---\n\nGeneral rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true when applyTo is empty")
	}
}

// --- Copilot rules: render ---

func TestCopilotRenderGlobs(t *testing.T) {
	input := []byte("---\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nTypeScript rule.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "applyTo:")
	assertContains(t, out, "*.ts")
	assertContains(t, out, "*.tsx")
	assertContains(t, out, "TypeScript rule.")
	assertEqual(t, ".instructions.md", result.Filename)
}

func TestCopilotRenderAlwaysApply(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nGlobal instructions.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertNotContains(t, out, "---")
	assertNotContains(t, out, "applyTo")
	assertContains(t, out, "Global instructions.")
	assertEqual(t, "copilot-instructions.md", result.Filename)
}

func TestCopilotRenderDescriptionScope(t *testing.T) {
	input := []byte("---\ndescription: When writing tests\nalwaysApply: false\n---\n\nTest patterns.\n")

	conv := &RulesConverter{}
	result, err := conv.Render(input, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Apply when: When writing tests")
	assertEqual(t, "copilot-instructions.md", result.Filename)
}

// --- Copilot round-trip ---

func TestCopilotRoundTripApplyTo(t *testing.T) {
	// Copilot applyTo rule → canonical → Copilot should preserve semantics
	input := []byte("---\napplyTo: \"*.go\"\n---\n\nGo conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "applyTo")
	assertContains(t, out, "*.go")
	assertContains(t, out, "Go conventions.")
	assertEqual(t, ".instructions.md", result.Filename)
}

// --- Copilot to other providers ---

func TestCopilotApplyToToCursor(t *testing.T) {
	input := []byte("---\napplyTo: \"*.py\"\n---\n\nPython conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "alwaysApply: false")
	assertContains(t, out, "*.py")
	assertEqual(t, "rule.mdc", result.Filename)
}

// --- Markdown Rules ---

func TestRenderMarkdownRule(t *testing.T) {
	input := []byte("---\nalwaysApply: true\n---\n\nGeneric markdown rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Use a provider that doesn't have a specific renderer
	generic := provider.Provider{Slug: "unknown-provider", Name: "Unknown"}
	result, err := conv.Render(canonical.Content, generic)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Generic markdown rule.")
	assertEqual(t, "rule.md", result.Filename)
}

// --- Converter registry ---

func TestConverterFor(t *testing.T) {
	// All registered content types should return non-nil converters
	if For(catalog.Rules) == nil {
		t.Error("expected Rules converter to be registered")
	}
	if For(catalog.MCP) == nil {
		t.Error("expected MCP converter to be registered")
	}
	if For(catalog.Commands) == nil {
		t.Error("expected Commands converter to be registered")
	}
	if For(catalog.Agents) == nil {
		t.Error("expected Agents converter to be registered")
	}
	if For(catalog.Skills) == nil {
		t.Error("expected Skills converter to be registered")
	}
	if For(catalog.Hooks) == nil {
		t.Error("expected Hooks converter to be registered")
	}
}

// --- HasSourceFile / SourceFilePath / ResolveContentFile ---

func TestHasSourceFile(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// No .source dir
	if HasSourceFile(item) {
		t.Error("expected HasSourceFile to be false without .source dir")
	}

	// Create .source dir
	os.MkdirAll(filepath.Join(dir, SourceDir), 0755)
	if !HasSourceFile(item) {
		t.Error("expected HasSourceFile to be true with .source dir")
	}
}

func TestSourceFilePath(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// No .source dir
	if SourceFilePath(item) != "" {
		t.Error("expected empty path without .source dir")
	}

	// Create .source with a file
	sourceDir := filepath.Join(dir, SourceDir)
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "original.md"), []byte("content"), 0644)

	path := SourceFilePath(item)
	if path == "" {
		t.Error("expected non-empty path with source file")
	}
	assertContains(t, path, "original.md")
}

func TestSourceFilePathSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	sourceDir := filepath.Join(dir, SourceDir)
	os.MkdirAll(filepath.Join(sourceDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "file.md"), []byte("content"), 0644)

	path := SourceFilePath(item)
	assertContains(t, path, "file.md")
}

func TestResolveContentFile(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// No files → empty
	if ResolveContentFile(item) != "" {
		t.Error("expected empty path for empty dir")
	}

	// Create rule.md → found via known names
	os.WriteFile(filepath.Join(dir, "rule.md"), []byte("rule"), 0644)
	path := ResolveContentFile(item)
	assertContains(t, path, "rule.md")
}

func TestResolveContentFileFallbackToMD(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// Create a non-canonical .md file
	os.WriteFile(filepath.Join(dir, "custom.md"), []byte("custom"), 0644)
	path := ResolveContentFile(item)
	assertContains(t, path, "custom.md")
}

func TestResolveContentFileFallbackToTOML(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// Create .toml file only
	os.WriteFile(filepath.Join(dir, "command.toml"), []byte("toml"), 0644)
	path := ResolveContentFile(item)
	assertContains(t, path, "command.toml")
}

func TestResolveContentFileFallbackToJSON(t *testing.T) {
	dir := t.TempDir()
	item := catalog.ContentItem{Path: dir}

	// Create .json file only
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)
	path := ResolveContentFile(item)
	assertContains(t, path, "config.json")
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
