package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestClaudeSkillToGemini(t *testing.T) {
	input := []byte("---\nname: review\ndescription: Code review skill\nallowed-tools:\n  - Read\n  - Grep\nmodel: opus\ncontext: fork\n---\n\nReview code for best practices.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: review")
	assertContains(t, out, "description: Code review skill")
	assertContains(t, out, "Review code for best practices.")
	// Claude-specific fields embedded as prose
	assertContains(t, out, "read_file")
	assertContains(t, out, "grep_search")
	assertContains(t, out, "model: opus")
	assertContains(t, out, "isolated context")
	assertContains(t, out, "syllago:converted")
	// Should NOT have Claude-specific frontmatter fields
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestGeminiSkillToClaude(t *testing.T) {
	input := []byte("---\nname: helper\ndescription: General helper\n---\n\nHelp with tasks.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestSkillDisallowedToolsEmbedding(t *testing.T) {
	input := []byte("---\nname: safe\ndescription: Safe skill\ndisallowed-tools:\n  - Bash\n  - Write\n---\n\nDo safe things only.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "run_shell_command")
	assertContains(t, out, "write_file")
	assertContains(t, out, "Do not use")
}

func TestSkillNoFrontmatter(t *testing.T) {
	input := []byte("Just a plain skill with no frontmatter.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Just a plain skill")
	assertEqual(t, "SKILL.md", result.Filename)
}

// --- OpenCode skills ---

func TestClaudeSkillToOpenCode(t *testing.T) {
	input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\n---\n\nUse idiomatic Go patterns.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "# Go Expert")
	assertContains(t, out, "idiomatic Go")
	assertContains(t, out, "Go coding guidelines") // description embedded as prose
	assertNotContains(t, out, "allowed-tools")     // no YAML key in output
	assertContains(t, out, "Tool restriction")     // embedded as prose instead
	assertContains(t, out, "Read")                 // tool name preserved in prose
	assertEqual(t, "go-expert.md", result.Filename)
}

// --- Kiro skills ---

func TestClaudeSkillToKiro(t *testing.T) {
	input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\nuser-invocable: true\n---\n\nUse idiomatic Go patterns.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "# Go Expert")
	assertContains(t, out, "idiomatic Go")
	assertContains(t, out, "Go coding guidelines") // description embedded as prose
	assertNotContains(t, out, "allowed-tools")     // no YAML key in output
	assertContains(t, out, "Tool restriction")     // allowed-tools embedded as prose
	assertContains(t, out, "command menu")         // user-invocable embedded as prose
	assertEqual(t, "go-expert.md", result.Filename)
}

// --- AllowedTools parsing ---

// --- Windsurf skills ---

func TestClaudeSkillToWindsurf(t *testing.T) {
	input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\n  - Grep\nmodel: opus\ncontext: fork\n---\n\nUse idiomatic Go patterns.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: Go Expert")
	assertContains(t, out, "description: Go coding guidelines")
	assertContains(t, out, "idiomatic Go")
	// Claude-specific fields should be embedded as prose, not in frontmatter
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertContains(t, out, "Tool restriction")
	assertContains(t, out, "view_line_range")           // translated tool name for Windsurf
	assertContains(t, out, "isolated context")          // context:fork prose
	assertContains(t, out, "Designed for model: opus.") // model as prose note
	assertContains(t, out, "syllago:converted")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestWindsurfSkillCanonicalize(t *testing.T) {
	// Windsurf SKILL.md with name and description frontmatter
	input := []byte("---\nname: deploy-to-production\ndescription: Guides the deployment process\n---\n\n## Steps\n\n1. Run pre-deployment checks\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "name: deploy-to-production")
	assertContains(t, out, "description: Guides the deployment process")
	assertContains(t, out, "pre-deployment checks")
}

func TestWindsurfSkillRoundTrip(t *testing.T) {
	input := []byte("---\nname: deploy-to-production\ndescription: Guides the deployment process\n---\n\n## Steps\n\n1. Run pre-deployment checks\n2. Build the release artifact\n")

	conv := &SkillsConverter{}

	// Windsurf -> canonical
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical -> Windsurf
	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: deploy-to-production")
	assertContains(t, out, "description: Guides the deployment process")
	assertContains(t, out, "pre-deployment checks")
	assertContains(t, out, "release artifact")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestSkillAllowedToolsCommaSeparated(t *testing.T) {
	// Comma-separated string in frontmatter: "Read, Grep, Glob"
	input := []byte("---\nname: test\nallowed-tools: \"Read, Grep, Glob\"\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	if len(meta.AllowedTools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(meta.AllowedTools), meta.AllowedTools)
	}
	assertEqual(t, "Read", meta.AllowedTools[0])
	assertEqual(t, "Grep", meta.AllowedTools[1])
	assertEqual(t, "Glob", meta.AllowedTools[2])
}

func TestSkillAllowedToolsSpaceDelimited(t *testing.T) {
	// Space-delimited string: "Read Grep Glob" (Agent Skills spec format)
	input := []byte("---\nname: test\nallowed-tools: \"Read Grep Glob\"\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	if len(meta.AllowedTools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(meta.AllowedTools), meta.AllowedTools)
	}
	assertEqual(t, "Read", meta.AllowedTools[0])
	assertEqual(t, "Grep", meta.AllowedTools[1])
	assertEqual(t, "Glob", meta.AllowedTools[2])
}

func TestSkillAllowedToolsYAMLList(t *testing.T) {
	// YAML list: already 3 elements, should be unchanged
	input := []byte("---\nname: test\nallowed-tools:\n  - Read\n  - Grep\n  - Glob\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	if len(meta.AllowedTools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(meta.AllowedTools), meta.AllowedTools)
	}
	assertEqual(t, "Read", meta.AllowedTools[0])
	assertEqual(t, "Grep", meta.AllowedTools[1])
	assertEqual(t, "Glob", meta.AllowedTools[2])
}

func TestSkillAllowedToolsSingle(t *testing.T) {
	// Single tool: "Bash" -> 1 element
	input := []byte("---\nname: test\nallowed-tools: Bash\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, _, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	if len(meta.AllowedTools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %v", len(meta.AllowedTools), meta.AllowedTools)
	}
	assertEqual(t, "Bash", meta.AllowedTools[0])
}

func TestSkillCommaSeparatedRenderTranslatesEach(t *testing.T) {
	// Verify that comma-separated tools get individually translated during render
	input := []byte("---\nname: test\nallowed-tools: \"Read, Grep, Glob\"\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Each tool should be individually translated to Gemini equivalents
	assertContains(t, out, "read_file")
	assertContains(t, out, "grep_search")
	assertContains(t, out, "glob") // Glob -> glob
	// Should NOT contain the original comma-separated string as a blob
	assertNotContains(t, out, "Read, Grep, Glob")
}

func TestSkillWithUserInvocable(t *testing.T) {
	boolTrue := true
	input := []byte("---\nname: test\ndescription: Test skill\nuser-invocable: true\nargument-hint: \"<query>\"\n---\n\nDo things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	_ = boolTrue
	out := string(result.Content)
	assertContains(t, out, "command menu")
	assertContains(t, out, "<query>")
}

// --- Cursor skills ---

func TestClaudeSkillToCursor(t *testing.T) {
	input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\n  - Grep\nmodel: opus\ncontext: fork\n---\n\nUse idiomatic Go patterns.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: Go Expert")
	assertContains(t, out, "description: Go coding guidelines")
	assertContains(t, out, "idiomatic Go")
	// Claude-specific fields should be embedded as prose, not in frontmatter
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertContains(t, out, "Tool restriction")
	assertContains(t, out, "read_file")                // translated tool name
	assertContains(t, out, "isolated context")         // context:fork prose
	assertContains(t, out, "Designed for model: opus") // model as prose note
	assertContains(t, out, "syllago:converted")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestCursorSkillCanonicalize(t *testing.T) {
	// Cursor SKILL.md with supported frontmatter fields
	input := []byte("---\nname: reviewer\ndescription: Code review helper\ndisable-model-invocation: true\n---\n\nReview code carefully.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "name: reviewer")
	assertContains(t, out, "description: Code review helper")
	assertContains(t, out, "disable-model-invocation: true")
	assertContains(t, out, "Review code carefully.")
}

func TestCursorSkillRoundTrip(t *testing.T) {
	input := []byte("---\nname: helper\ndescription: General helper\ndisable-model-invocation: true\n---\n\nHelp with tasks.\n")

	conv := &SkillsConverter{}

	// Cursor → canonical
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → Cursor
	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "disable-model-invocation: true")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "SKILL.md", result.Filename)
}

// --- Skill hook conversion warnings ---

func TestSkillWithHooks_ToGemini_ActionableWarnings(t *testing.T) {
	input := []byte("---\nname: greeting\ndescription: Greeting skill\nhooks:\n  PreToolUse:\n    - matcher: \"Edit|Write\"\n      command: ./validate.sh\n---\n\nGreet the user.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should have actionable warnings with hook details
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for skill hooks, got none")
	}
	warningText := joinWarnings(result.Warnings)
	assertContains(t, warningText, "greeting")
	assertContains(t, warningText, "PreToolUse")
	assertContains(t, warningText, "Edit|Write")
	assertContains(t, warningText, "./validate.sh")
	assertContains(t, warningText, ".gemini/settings.json")
	assertContains(t, warningText, "skill scoping will be lost")

	// Hooks should NOT be embedded as prose in the content
	out := string(result.Content)
	assertNotContains(t, out, "Hooks:")
	assertNotContains(t, out, "skill-scoped hook support")
}

func TestSkillWithHooks_ToCursor_ActionableWarnings(t *testing.T) {
	input := []byte("---\nname: validator\ndescription: Validation skill\nhooks:\n  PostToolUse:\n    - command: ./check.sh\n---\n\nValidate outputs.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for skill hooks, got none")
	}
	warningText := joinWarnings(result.Warnings)
	assertContains(t, warningText, "validator")
	assertContains(t, warningText, "PostToolUse")
	assertContains(t, warningText, "./check.sh")
	assertContains(t, warningText, ".cursor/settings.json")
}

func TestSkillWithHooks_ToWindsurf_ActionableWarnings(t *testing.T) {
	input := []byte("---\nname: linter\ndescription: Lint skill\nhooks:\n  PreToolUse:\n    - matcher: Bash\n      command: ./lint.sh\n---\n\nLint code.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for skill hooks, got none")
	}
	warningText := joinWarnings(result.Warnings)
	assertContains(t, warningText, "linter")
	assertContains(t, warningText, "PreToolUse")
	assertContains(t, warningText, "./lint.sh")
	assertContains(t, warningText, ".windsurf/hooks.json")
}

func TestSkillWithHooks_ToKiro_ActionableWarnings(t *testing.T) {
	input := []byte("---\nname: guard\ndescription: Guard skill\nhooks:\n  SessionStart:\n    - command: ./setup.sh\n---\n\nGuard the session.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for skill hooks, got none")
	}
	warningText := joinWarnings(result.Warnings)
	assertContains(t, warningText, "guard")
	assertContains(t, warningText, "SessionStart")
	assertContains(t, warningText, "./setup.sh")
	assertContains(t, warningText, ".kiro/")
}

func TestSkillWithHooks_ToOpenCode_ProseEmbedding(t *testing.T) {
	// OpenCode is hookless — hooks should be embedded as prose, no warnings
	input := []byte("---\nname: checker\ndescription: Check skill\nhooks:\n  PreToolUse:\n    - command: ./check.sh\n---\n\nCheck things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// No warnings for hookless provider
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for hookless provider, got %v", result.Warnings)
	}

	// Hooks should be embedded as prose
	out := string(result.Content)
	assertContains(t, out, "Hooks:")
	assertContains(t, out, "skill-scoped hook support")
}

func TestSkillWithHooks_ToClaude_NoWarnings(t *testing.T) {
	// Claude Code round-trip: hooks preserved in frontmatter, no warnings needed
	input := []byte("---\nname: test\nhooks:\n  PreToolUse:\n    - command: ./test.sh\n---\n\nTest.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for Claude round-trip, got %v", result.Warnings)
	}

	out := string(result.Content)
	assertContains(t, out, "hooks:")
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "./test.sh")
}

func TestSkillNoHooks_ToGemini_NoWarnings(t *testing.T) {
	// Skill without hooks should produce no warnings
	input := []byte("---\nname: simple\ndescription: Simple skill\n---\n\nDo simple things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings for hookless skill, got %v", result.Warnings)
	}
}

// joinWarnings concatenates all warnings into a single string for assertion convenience.
func joinWarnings(warnings []string) string {
	return strings.Join(warnings, "\n")
}
