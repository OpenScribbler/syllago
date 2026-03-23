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
	// fs_write → file_edit (or file_write, both map to fs_write)
	// shell → shell
	assertContains(t, out, "shell")
	// mcpServers should be preserved in canonical
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "github")
	// Body should be preserved
	assertContains(t, out, "You are an AWS and Rust specialist.")
}

func TestKiroAgentCanonicalize_WarnsDroppedToolAliases(t *testing.T) {
	input := []byte("---\nname: My Agent\ntoolAliases:\n  read_file: cat\n---\n\nDo things.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
	}
	assertContains(t, result.Warnings[0], "toolAliases dropped")
}

func TestKiroAgentCanonicalize_WarnsMultipleDroppedFields(t *testing.T) {
	input := []byte("---\nname: My Agent\ntoolAliases:\n  read_file: cat\ntoolsSettings:\n  shell:\n    timeout: 30\nresources:\n  - res1\nincludeMcpJson: true\nincludePowers: true\nkeyboardShortcut: ctrl+k\nwelcomeMessage: Hello!\n---\n\nDo things.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	if len(result.Warnings) != 7 {
		t.Fatalf("expected 7 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	// Verify each dropped field is mentioned
	expectedFields := []string{"toolAliases", "toolsSettings", "resources", "includeMcpJson", "includePowers", "keyboardShortcut", "welcomeMessage"}
	for i, field := range expectedFields {
		assertContains(t, result.Warnings[i], field+" dropped")
	}
}

func TestKiroAgentCanonicalize_NoWarningsForStandardFields(t *testing.T) {
	input := []byte("---\nname: Clean Agent\ndescription: Standard fields only\nmodel: claude-sonnet-4\ntools:\n  - shell\nmcpServers:\n  - github\n---\n\nStandard agent.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
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
	input := []byte("---\nname: Refactor Bot\ndescription: Refactoring assistant\ntools:\n  - Read\n  - Bash\nmodel: sonnet\nmaxTurns: 20\npermissionMode: bypassPermissions\ncolor: '#ff6600'\n---\n\nYou help refactor code.\n")

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
	assertContains(t, out, "name: Refactor Bot")
	assertContains(t, out, "description: Refactoring assistant")
	assertContains(t, out, "model: sonnet")
	// maxTurns → steps
	assertContains(t, out, "steps: 20")
	assertNotContains(t, out, "maxTurns")
	// tools as map[string]bool
	assertContains(t, out, "read: true")
	assertContains(t, out, "bash: true")
	// color preserved
	assertContains(t, out, "color:")
	// permissionMode not in frontmatter, embedded as prose
	assertContains(t, out, "Bypass all permission checks")
	assertContains(t, out, "syllago:converted")
	// Body preserved
	assertContains(t, out, "refactor code")
	assertEqual(t, "refactor-bot.md", result.Filename)

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about dropped permissionMode")
	}
}

func TestOpenCodeAgentCanonicalize(t *testing.T) {
	input := []byte("---\nname: Code Reviewer\ndescription: Reviews code changes\ntools:\n  read: true\n  bash: true\n  grep: true\n  write: false\nmodel: gpt-4o\nsteps: 15\ncolor: '#00ff00'\ntemperature: 0.3\n---\n\nReview all changed files carefully.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: Code Reviewer")
	assertContains(t, out, "description: Reviews code changes")
	assertContains(t, out, "model: gpt-4o")
	// steps → maxTurns in canonical
	assertContains(t, out, "maxTurns: 15")
	assertNotContains(t, out, "steps:")
	// tools mapped to canonical names (only enabled ones)
	assertContains(t, out, "file_read")
	assertContains(t, out, "shell")
	assertContains(t, out, "search")
	// write: false should be excluded
	assertNotContains(t, out, "file_write") // write→file_write, not included since false
	// color and temperature preserved
	assertContains(t, out, "color:")
	assertContains(t, out, "temperature: 0.3")
	// Body preserved
	assertContains(t, out, "Review all changed files carefully.")
}

func TestOpenCodeAgentRoundTrip(t *testing.T) {
	input := []byte("---\nname: Helper\ndescription: General helper\ntools:\n  read: true\n  bash: true\nmodel: gpt-4o\nsteps: 10\ncolor: '#0099ff'\ntemperature: 0.5\n---\n\nHelp with tasks.\n")

	conv := &AgentsConverter{}

	// OpenCode → canonical
	canonical, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// canonical → OpenCode
	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: Helper")
	assertContains(t, out, "description: General helper")
	assertContains(t, out, "model: gpt-4o")
	assertContains(t, out, "steps: 10")
	assertContains(t, out, "color:")
	assertContains(t, out, "temperature: 0.5")
	// Tools should be map format
	assertContains(t, out, "read: true")
	assertContains(t, out, "bash: true")
	assertContains(t, out, "Help with tasks.")
	assertEqual(t, "helper.md", result.Filename)
}

func TestOpenCodeAgentRenderWarnings(t *testing.T) {
	// Test that unsupported fields produce warnings
	input := []byte("---\nname: Full\ndescription: Has everything\ntools:\n  - Read\npermissionMode: dontAsk\nskills:\n  - coding\nmemory: project\nbackground: true\nisolation: worktree\neffort: high\nhooks:\n  preToolUse:\n    - matcher: Bash\n      script: echo hi\n---\n\nFull agent.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should have warnings for all unsupported fields
	if len(result.Warnings) < 6 {
		t.Fatalf("expected at least 6 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}

	// Verify specific warnings exist
	allWarnings := strings.Join(result.Warnings, " | ")
	assertContains(t, allWarnings, "permissionMode")
	assertContains(t, allWarnings, "skills")
	assertContains(t, allWarnings, "memory")
	assertContains(t, allWarnings, "background")
	assertContains(t, allWarnings, "isolation")
	assertContains(t, allWarnings, "effort")
	assertContains(t, allWarnings, "hooks")
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
	input := []byte("---\nname: reviewer\ndescription: Code review agent\ntools:\n  - Read\n  - Grep\n  - Bash\nmodel: sonnet\nmaxTurns: 25\n---\n\nReview all changed files.\n")

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
	assertContains(t, out, "Review all changed files")
	// Should use OpenCode field names
	assertContains(t, out, "steps: 25")
	assertContains(t, out, "read: true")
	assertContains(t, out, "grep: true")
	assertContains(t, out, "bash: true")

	// OpenCode → canonical
	backToCanonical, err := conv.Canonicalize(opencode.Content, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize from OpenCode: %v", err)
	}

	backCanonicalStr := string(backToCanonical.Content)
	// Should have canonical field names
	assertContains(t, backCanonicalStr, "maxTurns: 25")
	assertContains(t, backCanonicalStr, "name: reviewer")

	// canonical → Claude
	backToClaude, err := conv.Render(backToCanonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}

	final := string(backToClaude.Content)
	assertContains(t, final, "Review all changed files")
	assertContains(t, final, "maxTurns: 25")
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
	assertContains(t, canonicalStr, "file_read")
	assertContains(t, canonicalStr, "shell")

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
	assertContains(t, out, "readonly: true")      // permissionMode:plan → readonly
	assertContains(t, out, "is_background: true") // background → is_background
	// Unsupported fields embedded as prose
	assertContains(t, out, "read_file")         // tool name translated
	assertContains(t, out, "Limit to 30 turns") // maxTurns as prose
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

// --- permissionMode 5-value tests ---

func TestCanonicalizePreservesDontAsk(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: auto\npermissionMode: dontAsk\n---\n\nRun autonomously.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "permissionMode: dontAsk")
}

func TestCanonicalizePreservesBypassPermissions(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: bypass\npermissionMode: bypassPermissions\n---\n\nFull autonomy.\n")

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "permissionMode: bypassPermissions")
}

func TestClaudeAgentAllFivePermissionModes(t *testing.T) {
	t.Parallel()
	modes := []string{"default", "acceptEdits", "dontAsk", "bypassPermissions", "plan"}

	conv := &AgentsConverter{}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			input := []byte("---\nname: test\npermissionMode: " + mode + "\n---\n\nInstructions.\n")

			canonical, err := conv.Canonicalize(input, "claude-code")
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}

			result, err := conv.Render(canonical.Content, provider.ClaudeCode)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			out := string(result.Content)
			assertContains(t, out, "permissionMode: "+mode)
		})
	}
}

func TestCursorAgentDontAskProseNote(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: auto\npermissionMode: dontAsk\n---\n\nRun tasks.\n")

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
	// Should NOT have readonly (that's only for "plan")
	assertNotContains(t, out, "readonly: true")
	// Should have prose note
	assertContains(t, out, "without asking for confirmation")
	// Should have warning
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning about permissionMode not fully supported by Cursor")
	}
}

func TestGeminiAgentBypassPermissionsNote(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: bypass\npermissionMode: bypassPermissions\n---\n\nDo everything.\n")

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
	assertContains(t, out, "Bypass all permission checks")
}

func TestGeminiAgentDefaultPermissionNoNote(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: normal\npermissionMode: default\n---\n\nNormal agent.\n")

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
	// "default" should NOT produce any permission note or conversion notes block
	assertNotContains(t, out, "Permission mode")
	assertNotContains(t, out, "permission checks")
	assertNotContains(t, out, "syllago:converted")
}

func TestEffortMaxRoundTripClaude(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: maxeffort\neffort: max\n---\n\nTry hardest.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "effort: max")
}

func TestEffortMaxGeminiProse(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: maxeffort\neffort: max\n---\n\nTry hardest.\n")

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
	assertContains(t, out, "Effort level: max")
}

func TestCopilotAgentDontAskNote(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: auto\npermissionMode: dontAsk\n---\n\nRun freely.\n")

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
	assertContains(t, out, "without asking for confirmation")
}

func TestCursorAgentDefaultPermissionNoNote(t *testing.T) {
	t.Parallel()
	input := []byte("---\nname: normal\npermissionMode: default\n---\n\nNormal work.\n")

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
	assertNotContains(t, out, "readonly: true")
	assertNotContains(t, out, "Permission mode")
	assertNotContains(t, out, "permission checks")
	// "default" should not trigger a warning about unsupported permissionMode
	for _, w := range result.Warnings {
		if strings.Contains(w, "permissionMode") {
			t.Fatalf("unexpected permissionMode warning for 'default': %s", w)
		}
	}
}
