package converter

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/provider"
)

func TestClaudeAgentToGemini(t *testing.T) {
	input := []byte("---\nname: explorer\ndescription: Codebase explorer\ntools:\n  - Read\n  - Glob\n  - Grep\nmodel: sonnet\nmaxTurns: 30\npermissionMode: plan\n---\n\nExplore the codebase and summarize.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: explorer")
	assertContains(t, out, "read_file")
	assertContains(t, out, "list_directory")
	assertContains(t, out, "grep_search")
	assertContains(t, out, "max_turns: 30")
	assertContains(t, out, "read-only exploration mode")
	assertContains(t, out, "nesco:converted")
	assertEqual(t, "agent.md", result.Filename)
}

func TestGeminiAgentToClaude(t *testing.T) {
	input := []byte("---\nname: researcher\ndescription: Research agent\ntools:\n  - read_file\n  - grep_search\ntemperature: 0.7\ntimeout_mins: 10\n---\n\nResearch and summarize findings.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: researcher")
	assertContains(t, out, "Read")
	assertContains(t, out, "Grep")
	assertContains(t, out, "temperature: 0.7")
	assertContains(t, out, "Limit execution to 10 minutes")
}

func TestAgentDisallowedToolsEmbedding(t *testing.T) {
	input := []byte("---\nname: safe\ndescription: Safe agent\ndisallowedTools:\n  - Bash\n  - Write\n---\n\nOperate safely.\n")

	conv := &AgentsConverter{}
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
	assertContains(t, out, "Do not use these tools")
}

func TestAgentBackgroundAndWorktree(t *testing.T) {
	input := []byte("---\nname: bg\ndescription: Background agent\nbackground: true\nisolation: worktree\n---\n\nRun in background.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "background task")
	assertContains(t, out, "separate git worktree")
}

func TestAgentToCopilot(t *testing.T) {
	input := []byte("---\nname: helper\ndescription: Helper agent\ntools:\n  - Read\n  - Write\nmodel: opus\nmaxTurns: 20\n---\n\nHelp with tasks.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: helper")
	assertContains(t, out, "view")
	assertContains(t, out, "apply_patch")
	assertContains(t, out, "model: opus")
	assertContains(t, out, "Limit to 20 turns")
}

func TestAgentNoFrontmatter(t *testing.T) {
	input := []byte("Just instructions for an agent.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Just instructions for an agent.")
}
