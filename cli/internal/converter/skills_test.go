package converter

import (
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/provider"
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
	assertContains(t, out, "nesco:converted")
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
