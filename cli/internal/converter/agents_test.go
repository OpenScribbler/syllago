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
	assertContains(t, out, "glob")
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

func TestAgentEffortHooksColor(t *testing.T) {
	input := []byte("---\nname: careful\ndescription: Careful agent\neffort: high\ncolor: \"#ff6600\"\nhooks:\n  preToolUse:\n    - matcher: Bash\n      script: echo checking\n---\n\nBe careful.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify canonical preserves all three fields
	canonStr := string(canonical.Content)
	assertContains(t, canonStr, "effort: high")
	assertContains(t, canonStr, `color: '#ff6600'`)
	assertContains(t, canonStr, "hooks:")
	assertContains(t, canonStr, "preToolUse")

	// Render back to Claude Code — fields should survive in frontmatter
	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "effort: high")
	assertContains(t, out, "color:")
	assertContains(t, out, "hooks:")

	// Render to Gemini — fields should appear as conversion notes
	gemResult, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render to Gemini: %v", err)
	}
	gemOut := string(gemResult.Content)
	assertContains(t, gemOut, "Effort level: high")
	assertContains(t, gemOut, "hooks configured")
	assertContains(t, gemOut, "Agent color: #ff6600")
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
	assertContains(t, out, "create")
	assertContains(t, out, "model: opus")
	assertContains(t, out, "Limit to 20 turns")
	// Filename should be <name>.agent.md
	assertEqual(t, "helper.agent.md", result.Filename)
}

func TestCopilotAgentModelAndMCPServers(t *testing.T) {
	input := []byte("---\nname: deployer\ndescription: Deploy agent\ntools:\n  - Bash\nmodel: gpt-4o\nmcpServers:\n  - github\n  - jira\n---\n\nDeploy the application.\n")

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
	// Model should be in frontmatter (not conversion notes)
	assertContains(t, out, "model: gpt-4o")
	assertNotContains(t, out, "Designed for model")
	// MCP servers in frontmatter with Copilot's "mcp-servers" key
	assertContains(t, out, "mcp-servers:")
	assertContains(t, out, "github")
	assertContains(t, out, "jira")
	// Filename
	assertEqual(t, "deployer.agent.md", result.Filename)
}

func TestCopilotAgentTargetFromIsolation(t *testing.T) {
	input := []byte("---\nname: isolator\ndescription: Isolated agent\nisolation: worktree\n---\n\nWork in isolation.\n")

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
	// isolation: worktree maps to target: workspace in Copilot
	assertContains(t, out, "target: workspace")
	assertEqual(t, "isolator.agent.md", result.Filename)
}

func TestCopilotAgentFilenameNoName(t *testing.T) {
	// Agent without a name should get "agent.agent.md"
	input := []byte("---\ndescription: No name agent\n---\n\nInstructions.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	assertEqual(t, "agent.agent.md", result.Filename)
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
	// Output is now markdown with YAML frontmatter (not JSON)
	assertContains(t, out, "---")
	assertContains(t, out, "name: AWS Expert")
	assertContains(t, out, "description: AWS Rust development specialist")
	assertContains(t, out, "model: claude-sonnet-4")
	// Tool names translated to Kiro in YAML frontmatter
	assertContains(t, out, "read")
	assertContains(t, out, "fs_write")
	assertContains(t, out, "shell")
	// Prompt body is in the markdown body after frontmatter
	assertContains(t, out, "You are an expert in AWS and Rust development.")
	assertEqual(t, "aws-expert.md", result.Filename)

	// maxTurns should warn
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped maxTurns")
	}
}

func TestKiroAgentCanonicalize(t *testing.T) {
	// Kiro agents are markdown with YAML frontmatter.
	// Use unambiguous tool names: fs_write→Write/Edit, shell→Bash
	// ("read" is ambiguous — maps to Read, Glob, and Grep)
	input := []byte("---\nname: AWS Expert\ndescription: AWS and Rust specialist\nmodel: claude-sonnet-4\ntools:\n  - fs_write\n  - shell\nmcpServers:\n  - github\n---\n\nYou are an AWS and Rust specialist.\n")

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
	// mcpServers should be preserved in canonical
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "github")
	// Body should be preserved
	assertContains(t, out, "You are an AWS and Rust specialist.")
}

func TestKiroAgentYAMLFrontmatterStructure(t *testing.T) {
	// Verify that rendered Kiro agents have proper YAML frontmatter structure
	input := []byte("---\nname: Test Agent\ndescription: A test agent\ntools:\n  - Read\n  - Bash\nmodel: claude-sonnet-4\nmcpServers:\n  - github\n---\n\nDo test things.\n")

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

	// Must start with YAML frontmatter delimiter
	if !strings.HasPrefix(out, "---\n") {
		t.Fatal("expected output to start with YAML frontmatter delimiter '---'")
	}

	// Must have closing frontmatter delimiter
	rest := out[4:] // skip opening "---\n"
	if !strings.Contains(rest, "---\n") {
		t.Fatal("expected closing YAML frontmatter delimiter '---'")
	}

	// Frontmatter should contain Kiro-translated tool names
	assertContains(t, out, "- read")
	assertContains(t, out, "- shell")
	// mcpServers should be passed through
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "github")
	// Body after frontmatter
	assertContains(t, out, "Do test things.")
	// Filename should be .md
	assertEqual(t, "test-agent.md", result.Filename)
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
	assertContains(t, codexStr, "developer_instructions")
	assertNotContains(t, codexStr, "[agent.instructions]")
	assertContains(t, codexStr, "Plan the implementation strategy")
	// Tools in Codex vocabulary
	assertContains(t, codexStr, "read_file") // Read → read_file
	assertContains(t, codexStr, "shell")     // Bash → shell
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
		{"cursor", provider.Cursor},
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

// --- Cursor agents ---

func TestClaudeAgentToCursor(t *testing.T) {
	input := []byte("---\nname: Explorer\ndescription: Codebase explorer\ntools:\n  - Read\n  - Grep\n  - Bash\nmodel: sonnet\nmaxTurns: 30\npermissionMode: plan\nbackground: true\n---\n\nExplore the codebase and summarize.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cursor)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Cursor-supported frontmatter fields
	assertContains(t, out, "name: Explorer")
	assertContains(t, out, "description: Codebase explorer")
	assertContains(t, out, "model: sonnet")
	assertContains(t, out, "readonly: true")       // permissionMode:plan → readonly
	assertContains(t, out, "is_background: true")  // background → is_background
	// Unsupported fields embedded as prose
	assertContains(t, out, "read_file")            // tool name translated
	assertContains(t, out, "Limit to 30 turns")    // maxTurns as prose
	assertContains(t, out, "syllago:converted")
	// Body preserved
	assertContains(t, out, "Explore the codebase and summarize.")
	assertEqual(t, "explorer.md", result.Filename)
}

func TestCursorAgentCanonicalize(t *testing.T) {
	input := []byte("---\nname: Reviewer\ndescription: Code review agent\nmodel: gpt-4\nreadonly: true\nis_background: true\n---\n\nReview all code changes carefully.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: Reviewer")
	assertContains(t, out, "description: Code review agent")
	assertContains(t, out, "model: gpt-4")
	assertContains(t, out, "permissionMode: plan") // readonly → permissionMode:plan
	assertContains(t, out, "background: true")     // is_background → background
	assertContains(t, out, "Review all code changes carefully.")
}

func TestCursorAgentRoundTrip(t *testing.T) {
	input := []byte("---\nname: Helper\ndescription: General helper\nmodel: gpt-4\nreadonly: true\nis_background: true\n---\n\nHelp with tasks.\n")

	conv := &AgentsConverter{}

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
	assertContains(t, out, "name: Helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "model: gpt-4")
	assertContains(t, out, "readonly: true")
	assertContains(t, out, "is_background: true")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "helper.md", result.Filename)
}

func TestCursorAgentNoFrontmatter(t *testing.T) {
	input := []byte("Just a plain agent with instructions.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Just a plain agent with instructions.")
}
