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
	input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\nmodel: opus\ncontext: fork\n---\n\nUse idiomatic Go patterns.\n")

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
	// OpenCode now uses YAML frontmatter (Agent Skills spec)
	assertContains(t, out, "name: Go Expert")
	assertContains(t, out, "description: Go coding guidelines")
	assertContains(t, out, "idiomatic Go")
	// CC-specific fields should be embedded as prose, not in frontmatter
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertContains(t, out, "Tool restriction")         // allowed-tools as prose
	assertContains(t, out, "file_read")                // tool name in prose (neutral canonical)
	assertContains(t, out, "isolated context")         // context:fork prose
	assertContains(t, out, "Designed for model: opus") // model as prose
	assertContains(t, out, "syllago:converted")
	assertEqual(t, "SKILL.md", result.Filename) // SKILL.md, not slug.md
}

func TestOpenCodeSkillCanonicalize(t *testing.T) {
	// OpenCode SKILL.md with YAML frontmatter (Agent Skills spec)
	input := []byte("---\nname: deploy\ndescription: Deploy helper\nlicense: MIT\n---\n\nDeploy to production.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Should preserve frontmatter fields
	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "deploy", meta.Name)
	assertEqual(t, "Deploy helper", meta.Description)
	assertEqual(t, "MIT", meta.License)
	assertContains(t, body, "Deploy to production.")
}

func TestOpenCodeSkillCanonicalize_AllFields(t *testing.T) {
	input := []byte("---\nname: reviewer\ndescription: Code review\nlicense: Apache-2.0\ncompatibility: \">=1.0\"\nmetadata:\n  author: bob\n  team: platform\n---\n\nReview carefully.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "reviewer", meta.Name)
	assertEqual(t, "Code review", meta.Description)
	assertEqual(t, "Apache-2.0", meta.License)
	assertEqual(t, ">=1.0", meta.Compatibility)
	assertEqual(t, "bob", meta.Metadata["author"])
	assertEqual(t, "platform", meta.Metadata["team"])
	assertContains(t, body, "Review carefully.")
}

func TestOpenCodeSkillCanonicalize_PlainMarkdown(t *testing.T) {
	// Backward compat: OpenCode skill with no frontmatter should still work
	input := []byte("# My Skill\n\nDo useful things.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	// No frontmatter fields should be set
	assertEqual(t, "", meta.Name)
	assertEqual(t, "", meta.Description)
	assertContains(t, body, "My Skill")
	assertContains(t, body, "Do useful things.")
}

func TestOpenCodeSkillRenderFrontmatter(t *testing.T) {
	// Render to OpenCode should produce YAML frontmatter with supported fields
	input := []byte("---\nname: helper\ndescription: General helper\nlicense: MIT\ncompatibility: \">=2.0\"\nmetadata:\n  author: alice\n---\n\nHelp with tasks.\n")

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
	assertContains(t, out, "name: helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "license: MIT")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "author: alice")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestOpenCodeSkillRoundTrip(t *testing.T) {
	input := []byte("---\nname: deploy\ndescription: Deploy helper\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: bob\n---\n\nDeploy to production.\n")

	conv := &SkillsConverter{}

	// OpenCode -> canonical
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical -> OpenCode
	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: deploy")
	assertContains(t, out, "description: Deploy helper")
	assertContains(t, out, "license: MIT")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "author: bob")
	assertContains(t, out, "Deploy to production.")
	assertEqual(t, "SKILL.md", result.Filename)

	// Re-canonicalize to verify full round-trip
	canonical2, err := conv.Canonicalize(result.Content, "opencode")
	if err != nil {
		t.Fatalf("Re-canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical2.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "deploy", meta.Name)
	assertEqual(t, "Deploy helper", meta.Description)
	assertEqual(t, "MIT", meta.License)
	assertEqual(t, "bob", meta.Metadata["author"])
	assertContains(t, body, "Deploy to production.")
}

func TestOpenCodeSkillCCFieldsAsProseNotes(t *testing.T) {
	// CC-specific fields should become prose notes in OpenCode output
	input := []byte("---\nname: safe\nallowed-tools:\n  - Read\ncontext: fork\nagent: code\nmodel: opus\n---\n\nDo safe things.\n")

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
	assertContains(t, out, "Tool restriction")         // allowed-tools as prose
	assertContains(t, out, "isolated context")         // context:fork as prose
	assertContains(t, out, "code-focused approach")    // agent as prose
	assertContains(t, out, "Designed for model: opus") // model as prose
	assertContains(t, out, "syllago:converted")
	// These should NOT be in frontmatter
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertNotContains(t, out, "agent: code")
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
	// Kiro now uses YAML frontmatter (Agent Skills spec)
	assertContains(t, out, "name: Go Expert")
	assertContains(t, out, "description: Go coding guidelines")
	assertContains(t, out, "idiomatic Go")
	assertNotContains(t, out, "allowed-tools")  // CC-specific, not in Kiro frontmatter
	assertContains(t, out, "Tool restriction")  // allowed-tools embedded as prose
	assertContains(t, out, "command menu")      // user-invocable embedded as prose
	assertEqual(t, "SKILL.md", result.Filename) // SKILL.md, not slug.md
}

func TestKiroSkillCanonicalize(t *testing.T) {
	// Kiro SKILL.md with YAML frontmatter (Agent Skills spec)
	input := []byte("---\nname: deploy\ndescription: Deploy helper\nlicense: MIT\n---\n\nDeploy to production.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Should preserve frontmatter fields (not strip them like plain markdown path did)
	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "deploy", meta.Name)
	assertEqual(t, "Deploy helper", meta.Description)
	assertEqual(t, "MIT", meta.License)
	assertContains(t, body, "Deploy to production.")
}

func TestKiroSkillCanonicalize_AllFields(t *testing.T) {
	input := []byte("---\nname: reviewer\ndescription: Code review\nlicense: Apache-2.0\ncompatibility: \">=1.0\"\nmetadata:\n  author: bob\n  team: platform\n---\n\nReview carefully.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "reviewer", meta.Name)
	assertEqual(t, "Code review", meta.Description)
	assertEqual(t, "Apache-2.0", meta.License)
	assertEqual(t, ">=1.0", meta.Compatibility)
	assertEqual(t, "bob", meta.Metadata["author"])
	assertEqual(t, "platform", meta.Metadata["team"])
	assertContains(t, body, "Review carefully.")
}

func TestKiroSkillRenderFrontmatter(t *testing.T) {
	// Render to Kiro should produce YAML frontmatter with supported fields
	input := []byte("---\nname: helper\ndescription: General helper\nlicense: MIT\ncompatibility: \">=2.0\"\nmetadata:\n  author: alice\n---\n\nHelp with tasks.\n")

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
	assertContains(t, out, "name: helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "license: MIT")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "author: alice")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "SKILL.md", result.Filename)
}

func TestKiroSkillRoundTrip(t *testing.T) {
	input := []byte("---\nname: deploy\ndescription: Deploy helper\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: bob\n---\n\nDeploy to production.\n")

	conv := &SkillsConverter{}

	// Kiro -> canonical
	canonical, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical -> Kiro
	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: deploy")
	assertContains(t, out, "description: Deploy helper")
	assertContains(t, out, "license: MIT")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "author: bob")
	assertContains(t, out, "Deploy to production.")
	assertEqual(t, "SKILL.md", result.Filename)

	// Re-canonicalize to verify full round-trip
	canonical2, err := conv.Canonicalize(result.Content, "kiro")
	if err != nil {
		t.Fatalf("Re-canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical2.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "deploy", meta.Name)
	assertEqual(t, "Deploy helper", meta.Description)
	assertEqual(t, "MIT", meta.License)
	assertEqual(t, "bob", meta.Metadata["author"])
	assertContains(t, body, "Deploy to production.")
}

func TestKiroSkillCCFieldsAsProseNotes(t *testing.T) {
	// CC-specific fields should become prose notes in Kiro output
	input := []byte("---\nname: safe\nallowed-tools:\n  - Read\ncontext: fork\nagent: code\nmodel: opus\n---\n\nDo safe things.\n")

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
	assertContains(t, out, "Tool restriction")         // allowed-tools as prose
	assertContains(t, out, "isolated context")         // context:fork as prose
	assertContains(t, out, "code-focused approach")    // agent as prose
	assertContains(t, out, "Designed for model: opus") // model as prose
	assertContains(t, out, "syllago:converted")
	// These should NOT be in frontmatter
	assertNotContains(t, out, "allowed-tools:")
	assertNotContains(t, out, "context: fork")
	assertNotContains(t, out, "agent: code")
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
	assertEqual(t, "file_read", meta.AllowedTools[0])
	assertEqual(t, "search", meta.AllowedTools[1])
	assertEqual(t, "find", meta.AllowedTools[2])
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
	assertEqual(t, "file_read", meta.AllowedTools[0])
	assertEqual(t, "search", meta.AllowedTools[1])
	assertEqual(t, "find", meta.AllowedTools[2])
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
	assertEqual(t, "file_read", meta.AllowedTools[0])
	assertEqual(t, "search", meta.AllowedTools[1])
	assertEqual(t, "find", meta.AllowedTools[2])
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
	assertEqual(t, "shell", meta.AllowedTools[0])
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

// --- License, Compatibility, Metadata fields ---

func TestSkillMetaFields_Canonicalize(t *testing.T) {
	input := []byte("---\nname: review\ndescription: Code review\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: alice\n  category: testing\n---\n\nReview code.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "MIT", meta.License)
	assertEqual(t, ">=1.0", meta.Compatibility)
	assertEqual(t, "alice", meta.Metadata["author"])
	assertEqual(t, "testing", meta.Metadata["category"])
	assertContains(t, body, "Review code.")
}

func TestSkillMetaFields_RenderClaude(t *testing.T) {
	input := []byte("---\nname: review\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: alice\n---\n\nReview code.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "license: MIT")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "author: alice")
	assertContains(t, out, "Review code.")
}

func TestSkillMetaFields_RenderCursor(t *testing.T) {
	input := []byte("---\nname: review\nlicense: Apache-2.0\ncompatibility: \">=2.0\"\nmetadata:\n  team: backend\n---\n\nReview code.\n")

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
	assertContains(t, out, "license: Apache-2.0")
	assertContains(t, out, "compatibility:")
	assertContains(t, out, "team: backend")
	assertContains(t, out, "Review code.")
}

func TestSkillMetaFields_RenderGemini_NotInFrontmatter(t *testing.T) {
	input := []byte("---\nname: review\ndescription: Code review\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: alice\n---\n\nReview code.\n")

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
	assertContains(t, out, "description: Code review")
	// These fields should NOT be in the Gemini output
	assertNotContains(t, out, "license:")
	assertNotContains(t, out, "compatibility:")
	assertNotContains(t, out, "author: alice")
}

func TestSkillMetaFields_RenderWindsurf_NotInFrontmatter(t *testing.T) {
	input := []byte("---\nname: review\ndescription: Code review\nlicense: MIT\nmetadata:\n  author: alice\n---\n\nReview code.\n")

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
	assertContains(t, out, "name: review")
	assertContains(t, out, "description: Code review")
	assertNotContains(t, out, "license:")
	assertNotContains(t, out, "author: alice")
}

func TestSkillMetaFields_ClaudeRoundTrip(t *testing.T) {
	input := []byte("---\nname: review\nlicense: MIT\ncompatibility: \">=1.0\"\nmetadata:\n  author: alice\n  category: testing\n---\n\nReview code.\n")

	conv := &SkillsConverter{}

	// Canonicalize
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Render to Claude
	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Re-canonicalize the rendered output
	canonical2, err := conv.Canonicalize(result.Content, "claude-code")
	if err != nil {
		t.Fatalf("Re-canonicalize: %v", err)
	}

	meta, body, err := parseSkillCanonical(canonical2.Content)
	if err != nil {
		t.Fatalf("parseSkillCanonical: %v", err)
	}

	assertEqual(t, "MIT", meta.License)
	assertEqual(t, ">=1.0", meta.Compatibility)
	assertEqual(t, "alice", meta.Metadata["author"])
	assertEqual(t, "testing", meta.Metadata["category"])
	assertContains(t, body, "Review code.")
}

// joinWarnings concatenates all warnings into a single string for assertion convenience.
func joinWarnings(warnings []string) string {
	return strings.Join(warnings, "\n")
}
