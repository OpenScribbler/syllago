package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
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
	assertContains(t, out, "syllago:converted")
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

// --- Kiro agents ---

func TestClaudeAgentToKiro(t *testing.T) {
	input := []byte("---\nname: AWS Expert\ndescription: AWS Rust development specialist\ntools:\n  - Read\n  - Write\n  - Bash\nmodel: claude-sonnet-4\nmaxTurns: 20\n---\n\nYou are an expert in AWS and Rust development.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, `"name": "AWS Expert"`)
	assertContains(t, out, `"description"`)
	assertContains(t, out, `"prompt": "You are an expert in AWS and Rust development."`)
	assertContains(t, out, `"model": "claude-sonnet-4"`)
	// Tool names translated to Kiro
	assertContains(t, out, `"read"`)
	assertContains(t, out, `"fs_write"`)
	assertContains(t, out, `"shell"`)
	assertEqual(t, "aws-expert.json", result.Filename)

	// Prompt body is inlined — no ExtraFiles
	if result.ExtraFiles != nil {
		t.Errorf("expected no ExtraFiles (prompt inlined), got %d", len(result.ExtraFiles))
	}

	// maxTurns should warn
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped maxTurns")
	}
}

func TestKiroAgentCanonicalize(t *testing.T) {
	// Use unambiguous tool names: fs_write→Write/Edit, shell→Bash
	// ("read" is ambiguous — maps to Read, Glob, and Grep)
	input := []byte(`{
		"name": "AWS Expert",
		"description": "AWS and Rust specialist",
		"prompt": "file://./prompts/aws-expert.md",
		"model": "claude-sonnet-4",
		"tools": ["fs_write", "shell"]
	}`)

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: AWS Expert")
	assertContains(t, out, "model: claude-sonnet-4")
	// fs_write → Write (or Edit, both map to fs_write)
	// shell → Bash
	assertContains(t, out, "Bash")
	// Prompt file reference should be preserved
	assertContains(t, out, "kiro:prompt-file")
}

// --- OpenCode agents ---

func TestClaudeAgentToOpenCode(t *testing.T) {
	input := []byte("---\nname: Refactor Bot\ndescription: Refactoring assistant\npermissionMode: bypassPermissions\n---\n\nYou help refactor code.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Refactor Bot")
	assertContains(t, out, "refactor code")
	assertNotContains(t, out, "permissionMode")
	assertEqual(t, "refactor-bot.md", result.Filename)

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped permissionMode")
	}
}

// --- Roo Code agents ---

func TestAgentToRooCode(t *testing.T) {
	input := []byte("---\nname: Explorer\ndescription: Explore codebase\ntools:\n  - Read\n  - Glob\n  - Grep\n  - Bash\n---\n\nExplore and summarize the codebase.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// YAML output with Roo Code custom mode fields
	assertContains(t, out, "slug: explorer")
	assertContains(t, out, "name: Explorer")
	assertContains(t, out, "roleDefinition:")
	assertContains(t, out, "whenToUse: Explore codebase")
	// Tool groups mapped from canonical tool names
	assertContains(t, out, "read")
	assertContains(t, out, "command")
	// Filename should be slugified
	assertContains(t, result.Filename, "explorer.yaml")
}

func TestAgentToRooCodeToolGroupDedup(t *testing.T) {
	// Read, Glob, Grep all map to "read" — should appear only once
	input := []byte("---\nname: Reader\ndescription: Read things\ntools:\n  - Read\n  - Glob\n  - Grep\n---\n\nRead everything.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "read")
	// "read" should appear in groups but only once (deduped via map)
	assertNotContains(t, out, "edit")
	assertNotContains(t, out, "command")
}

func TestAgentToRooCodeDroppedFieldWarnings(t *testing.T) {
	input := []byte("---\nname: Full Agent\ndescription: Has everything\nmodel: opus\nmaxTurns: 50\npermissionMode: plan\nmemory: project\nbackground: true\nisolation: worktree\nskills:\n  - coding\nmcpServers:\n  - github\ndisallowedTools:\n  - Bash\n---\n\nFull instructions.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should have warnings for all unsupported fields
	if len(result.Warnings) < 5 {
		t.Fatalf("expected at least 5 warnings for dropped fields, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

// --- Cross-provider integration roundtrip tests ---

func TestClaudeToOpenCodeRoundtrip(t *testing.T) {
	input := []byte("---\nname: reviewer\ndescription: Code review agent\ntools:\n  - Read\n  - Grep\n  - Bash\nmodel: sonnet\n---\n\nReview all changed files.\n")

	conv := &AgentsConverter{}

	// Claude → canonical
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → OpenCode
	opencode, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render to OpenCode: %v", err)
	}

	out := string(opencode.Content)
	// OpenCode uses plain markdown (no frontmatter) with tool names translated
	assertContains(t, out, "Review all changed files")

	// OpenCode → canonical
	backToCanonical, err := conv.Canonicalize(opencode.Content, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize from OpenCode: %v", err)
	}

	// canonical → Claude
	backToClaude, err := conv.Render(backToCanonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}

	final := string(backToClaude.Content)
	assertContains(t, final, "Review all changed files")
}

func TestGeminiToCodexRoundtrip(t *testing.T) {
	input := []byte("---\nname: planner\ndescription: Planning agent\ntools:\n  - read_file\n  - run_shell_command\nmodel: gemini-pro\ntemperature: 0.5\n---\n\nPlan the implementation strategy.\n")

	conv := &AgentsConverter{}

	// Gemini → canonical
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize from Gemini: %v", err)
	}

	canonicalStr := string(canonical.Content)
	// Tools should be in canonical form
	assertContains(t, canonicalStr, "Read")
	assertContains(t, canonicalStr, "Bash")

	// canonical → Codex
	codex, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render to Codex: %v", err)
	}

	codexStr := string(codex.Content)
	assertContains(t, codexStr, "[agent]")
	assertContains(t, codexStr, "name = 'planner'")
	assertContains(t, codexStr, "[agent.instructions]")
	assertContains(t, codexStr, "Plan the implementation strategy")
	// Tools in Codex vocabulary (same as Copilot CLI)
	assertContains(t, codexStr, "view")  // Read → view
	assertContains(t, codexStr, "shell") // Bash → shell
}

func TestAgentAcrossAllNewProviders(t *testing.T) {
	// Verify a Claude agent can render to all new providers without error
	input := []byte("---\nname: universal\ndescription: Works everywhere\ntools:\n  - Read\n  - Bash\nmodel: sonnet\n---\n\nDo your best work.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	targets := []struct {
		name string
		prov provider.Provider
	}{
		{"opencode", provider.OpenCode},
		{"kiro", provider.Kiro},
		{"roo-code", provider.RooCode},
		{"codex", provider.Codex},
		{"copilot-cli", provider.CopilotCLI},
		{"gemini-cli", provider.GeminiCLI},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.prov)
			if err != nil {
				t.Fatalf("Render to %s: %v", tt.name, err)
			}
			if result.Content == nil || len(result.Content) == 0 {
				t.Fatalf("expected non-empty content for %s", tt.name)
			}
			out := string(result.Content)
			if !strings.Contains(out, "Do your best work") && !strings.Contains(out, "universal") {
				t.Errorf("expected agent content to be preserved for %s", tt.name)
			}
		})
	}
}
