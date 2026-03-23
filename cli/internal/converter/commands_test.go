package converter

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestClaudeCommandToGeminiTOML(t *testing.T) {
	input := []byte("---\nname: review\ndescription: Review code changes\nallowed-tools:\n  - Read\n  - Grep\ncontext: fork\n---\n\nReview the staged changes and provide feedback.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "description = 'Review code changes'")
	assertContains(t, out, "Review the staged changes")
	assertContains(t, out, "read_file")
	assertContains(t, out, "grep_search")
	assertContains(t, out, "isolated context")
	assertEqual(t, "command.toml", result.Filename)
}

func TestGeminiTOMLToClaudeCommand(t *testing.T) {
	input := []byte("name = \"deploy\"\ndescription = \"Deploy to prod\"\nprompt = \"Deploy the current branch to production.\"\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: deploy")
	assertContains(t, out, "description: Deploy to prod")
	assertContains(t, out, "Deploy the current branch")
	assertContains(t, out, "---")
	assertEqual(t, "command.md", result.Filename)
}

func TestClaudeCommandBehavioralEmbedding(t *testing.T) {
	input := []byte("---\nname: explore\ndescription: Explore codebase\nallowed-tools:\n  - Read\n  - Glob\nagent: Explore\nmodel: opus\ncontext: fork\n---\n\nExplore and summarize the codebase.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "read_file")
	assertContains(t, out, "glob")
	assertContains(t, out, "explore-focused approach")
	assertContains(t, out, "model: opus")
	assertContains(t, out, "isolated context")
	assertContains(t, out, "syllago:converted")
}

func TestCodexCommandToClaudeRoundTrip(t *testing.T) {
	input := []byte("Review PR changes and suggest improvements.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Review PR changes")
	assertContains(t, out, "---")
}

func TestArgumentPlaceholderTranslation(t *testing.T) {
	input := []byte("---\nname: greet\n---\n\nGreet $ARGUMENTS warmly.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "{{args}}")
	assertNotContains(t, out, "$ARGUMENTS")
}

func TestGeminiDirectivesWarning(t *testing.T) {
	input := []byte("name = \"diff\"\ndescription = \"Show diff\"\nprompt = \"Show diff for !{git diff --staged}\"\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about Gemini template directives")
	}
	assertContains(t, result.Warnings[0], "Gemini CLI template directives")
}

// --- OpenCode commands ---

func TestClaudeCommandToOpenCode(t *testing.T) {
	input := []byte("---\ndescription: Run the test suite\n---\n\nExecute all tests with coverage.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "test suite")
	assertContains(t, out, "coverage")
}

// --- Effort field tests ---

func TestCommandEffortPreservedInCanonical(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: high\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "effort: high")
}

func TestCommandEffortInClaudeFrontmatter(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: high\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "effort: high")
	assertContains(t, out, "Deploy to production.")
}

func TestCommandEffortAsGeminiBehavioralNote(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: max\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Effort level: max.")
}

func TestCommandEffortAsCodexBehavioralNote(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: low\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Effort level: low.")
}

func TestCommandEffortAsOpenCodeBehavioralNote(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: medium\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Effort level: medium.")
}

func TestCommandEffortRoundTripClaude(t *testing.T) {
	input := []byte("---\nname: deploy\neffort: high\n---\n\nDeploy to production.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	rendered, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Re-canonicalize the rendered output
	canonical2, err := conv.Canonicalize(rendered.Content, "claude-code")
	if err != nil {
		t.Fatalf("Re-canonicalize: %v", err)
	}

	out := string(canonical2.Content)
	assertContains(t, out, "effort: high")
	assertContains(t, out, "Deploy to production.")
}

func TestOpenCodeCommandWithAgentAndModel(t *testing.T) {
	input := []byte("---\nname: explore\ndescription: Explore codebase\nagent: Explore\nmodel: opus\n---\n\nExplore and summarize the codebase.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Agent and model should be in frontmatter, not as behavioral notes
	assertContains(t, out, "agent: Explore")
	assertContains(t, out, "model: opus")
	assertNotContains(t, out, "explore-focused approach")
	assertNotContains(t, out, "Designed for model")
	assertContains(t, out, "Explore and summarize")
}

func TestOpenCodeCommandWithContextFork(t *testing.T) {
	input := []byte("---\nname: isolated\ndescription: Run isolated\ncontext: fork\n---\n\nDo something in isolation.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Context "fork" should map to subtask: true in frontmatter
	assertContains(t, out, "subtask: true")
	// Should NOT have behavioral note about isolated context
	assertNotContains(t, out, "isolated context")
	assertContains(t, out, "Do something in isolation.")
}

func TestOpenCodeCommandMinimalFrontmatter(t *testing.T) {
	input := []byte("---\ndescription: Simple command\n---\n\nJust do a thing.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Only description in frontmatter — no agent, model, or subtask
	assertContains(t, out, "description: Simple command")
	assertNotContains(t, out, "agent:")
	assertNotContains(t, out, "model:")
	assertNotContains(t, out, "subtask:")
	assertNotContains(t, out, "syllago:converted")
	assertContains(t, out, "Just do a thing.")
}

func TestOpenCodeCommandBehavioralNotesForUnsupportedFields(t *testing.T) {
	input := []byte("---\nname: restricted\ndescription: Restricted command\nallowed-tools:\n  - Read\n  - Grep\neffort: high\nagent: Explore\nmodel: opus\ncontext: fork\n---\n\nDo restricted work.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Frontmatter fields for OpenCode-supported metadata
	assertContains(t, out, "agent: Explore")
	assertContains(t, out, "model: opus")
	assertContains(t, out, "subtask: true")
	// Behavioral notes only for unsupported fields
	assertContains(t, out, "Tool restriction")
	assertContains(t, out, "Effort level: high")
	// No behavioral notes for fields in frontmatter
	assertNotContains(t, out, "explore-focused approach")
	assertNotContains(t, out, "Designed for model")
	assertNotContains(t, out, "isolated context")
}

func TestCommandNoFrontmatterToGemini(t *testing.T) {
	input := []byte("Just do the thing.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Just do the thing.")
	assertEqual(t, "command.toml", result.Filename)
}

// --- Codex frontmatter and named args ---

func TestCodexCommandFrontmatterParsed(t *testing.T) {
	// Codex commands can have YAML frontmatter with description and argument-hint.
	input := []byte("---\ndescription: Review a pull request\nargument-hint: <pr-url>\n---\n\nReview the PR at $PR_URL and provide feedback.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	// Frontmatter fields should be preserved in canonical
	assertContains(t, out, "description: Review a pull request")
	assertContains(t, out, "argument-hint: <pr-url>")
	// Body with $NAME pattern should survive
	assertContains(t, out, "$PR_URL")
}

func TestCodexCommandFrontmatterRoundTrip(t *testing.T) {
	// Codex → canonical → Codex should preserve description and argument-hint.
	input := []byte("---\ndescription: Fix an issue\nargument-hint: <issue-number>\n---\n\nFix issue $ISSUE_NUMBER in the codebase.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "description: Fix an issue")
	assertContains(t, out, "argument-hint: <issue-number>")
	assertContains(t, out, "$ISSUE_NUMBER")
}

func TestCodexNamedArgsPreservedInBody(t *testing.T) {
	// $NAME patterns (e.g. $ISSUE_NUMBER, $PR_URL) should survive round-trip
	// through canonical format — they're literal text in the body.
	input := []byte("Review $PR_URL and fix $ISSUE_NUMBER.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify in canonical
	assertContains(t, string(canonical.Content), "$PR_URL")
	assertContains(t, string(canonical.Content), "$ISSUE_NUMBER")

	// Render back to Codex
	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render to Codex: %v", err)
	}
	assertContains(t, string(result.Content), "$PR_URL")
	assertContains(t, string(result.Content), "$ISSUE_NUMBER")

	// Render to Claude Code — $NAME patterns should survive as-is
	result2, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}
	assertContains(t, string(result2.Content), "$PR_URL")
	assertContains(t, string(result2.Content), "$ISSUE_NUMBER")
}

func TestCodexPlainBodyNoFrontmatter(t *testing.T) {
	// Plain body without frontmatter should still work (backwards compatible).
	input := []byte("Just review the code.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(canonical.Content)
	assertContains(t, out, "Just review the code.")
}
