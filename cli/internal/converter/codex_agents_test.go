package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestCanonicalizeCodexAgent_Single(t *testing.T) {
	input := []byte(`[features]
multi_agent = true

[agents.reviewer]
model = "o4-mini"
prompt = "You are a code reviewer. Check for bugs and style issues."
tools = ["shell", "read_file"]
`)

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	// Should have canonical frontmatter
	assertContains(t, out, "name: reviewer")
	assertContains(t, out, "model: o4-mini")
	// Tools should be reverse-translated: shell → shell, read_file → file_read
	assertContains(t, out, "shell")
	assertContains(t, out, "file_read")
	// Verify the YAML tools list contains canonical names not Codex ones
	assertContains(t, out, "- shell")
	assertContains(t, out, "- file_read")
	// Body should contain the prompt
	assertContains(t, out, "You are a code reviewer")
}

func TestCanonicalizeCodexAgent_MultiAgent(t *testing.T) {
	input := []byte(`[features]
multi_agent = true

[agents.reviewer]
model = "o4-mini"
prompt = "Review code"
tools = ["view"]

[agents.planner]
model = "o3"
prompt = "Plan the implementation"
tools = ["shell", "view"]
`)

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Should have primary result
	if result.Content == nil {
		t.Fatal("expected primary content")
	}

	// Should have ExtraFiles for second agent
	if result.ExtraFiles == nil {
		t.Fatal("expected ExtraFiles for multi-agent config")
	}
	if len(result.ExtraFiles) != 1 {
		t.Fatalf("expected 1 extra file, got %d", len(result.ExtraFiles))
	}
}

func TestRenderCodexAgent(t *testing.T) {
	input := []byte(`---
name: my-agent
description: A test agent
model: claude-sonnet-4-6
tools:
  - file_read
  - shell
  - search
maxTurns: 10
permissionMode: plan
---

You are a helpful agent that reviews code and suggests improvements.
`)

	conv := &AgentsConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should contain single-agent TOML structure
	assertContains(t, out, "[agent]")
	assertContains(t, out, "name = 'my-agent'")
	assertContains(t, out, "description = 'A test agent'")
	assertContains(t, out, "claude-sonnet-4-6")
	// Tools translated: Read→read_file, Bash→shell, Grep→grep_files
	assertContains(t, out, "read_file")
	assertContains(t, out, "shell")
	assertContains(t, out, "grep_files")
	// Prompt in developer_instructions field (not nested [agent.instructions])
	assertContains(t, out, "developer_instructions")
	assertContains(t, out, "reviews code")
	assertNotContains(t, out, "[agent.instructions]")
	// Filename uses agent slug
	assertEqual(t, "my-agent.toml", result.Filename)

	// Should warn about dropped fields
	hasMaxTurnsWarning := false
	hasPermissionWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "maxTurns") {
			hasMaxTurnsWarning = true
		}
		if strings.Contains(w, "permissionMode") {
			hasPermissionWarning = true
		}
	}
	if !hasMaxTurnsWarning {
		t.Error("expected warning about maxTurns")
	}
	if !hasPermissionWarning {
		t.Error("expected warning about permissionMode")
	}
}

func TestCodexAgentRoundtrip(t *testing.T) {
	// Start with Codex multi-agent TOML → canonicalize → render to single-agent → canonicalize again
	input := []byte(`[features]
multi_agent = true

[agents.coder]
model = "o4-mini"
prompt = "Write clean, tested code."
tools = ["shell", "apply_patch"]
`)

	conv := &AgentsConverter{}

	// Canonicalize from multi-agent format
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify canonical form has correct tool names
	canonicalStr := string(canonical.Content)
	assertContains(t, canonicalStr, "shell")     // shell → shell
	assertContains(t, canonicalStr, "file_edit") // apply_patch → file_edit

	// Render to single-agent Codex format
	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should have single-agent TOML structure
	assertContains(t, out, "[agent]")
	assertContains(t, out, "name = 'coder'")
	assertContains(t, out, "o4-mini")
	assertContains(t, out, "developer_instructions")
	assertNotContains(t, out, "[agent.instructions]")
	// Tools should be back in Codex vocabulary
	assertContains(t, out, "shell")
	assertContains(t, out, "apply_patch")
	// Prompt preserved in instructions
	assertContains(t, out, "Write clean, tested code.")

	// Re-canonicalize from single-agent format (the real round-trip test)
	canonical2, err := conv.Canonicalize(result.Content, "codex")
	if err != nil {
		t.Fatalf("Canonicalize pass 2: %v", err)
	}
	canonical2Str := string(canonical2.Content)
	assertContains(t, canonical2Str, "shell")
	assertContains(t, canonical2Str, "Write clean, tested code.")
}

func TestRenderCodexAgent_DropsAllUnsupportedFields(t *testing.T) {
	input := []byte(`---
name: full-agent
tools:
  - Read
disallowedTools:
  - Bash
model: claude-opus-4-6
maxTurns: 25
permissionMode: bypassPermissions
skills:
  - reviewer
mcpServers:
  - github
memory: project
background: true
isolation: worktree
---

An agent with all the fields.
`)

	conv := &AgentsConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should have warnings for truly unsupported fields
	expectedWarnings := []string{
		"maxTurns", "permissionMode",
		"memory", "background", "isolation", "disallowedTools",
	}
	for _, expected := range expectedWarnings {
		found := false
		for _, w := range result.Warnings {
			if strings.Contains(w, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing warning for field %q", expected)
		}
	}

	// skills and mcpServers ARE supported by Codex — should NOT have warnings
	for _, w := range result.Warnings {
		if strings.Contains(w, "skills") {
			t.Error("unexpected warning for 'skills' — Codex supports skills")
		}
		if strings.Contains(w, "mcpServers") {
			t.Error("unexpected warning for 'mcpServers' — Codex supports mcp_servers")
		}
	}

	// Verify skills and mcp_servers are rendered in the output
	out := string(result.Content)
	assertContains(t, out, "reviewer")
	assertContains(t, out, "github")
}

func TestRenderCodexAgent_DeveloperInstructions(t *testing.T) {
	input := []byte(`---
name: instructor
model: o4-mini
---

Follow these coding standards strictly.
`)

	conv := &AgentsConverter{}
	result, err := conv.Render(input, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// developer_instructions should be a top-level field under [agent], not nested
	assertContains(t, out, "developer_instructions")
	assertContains(t, out, "Follow these coding standards strictly.")
	assertNotContains(t, out, "[agent.instructions]")
}

func TestCanonicalizeCodexAgent_SingleWithNewFields(t *testing.T) {
	// Single-agent format with developer_instructions, mcp_servers, and skills
	input := []byte(`[agent]
name = "smart-agent"
model = "o4-mini"
developer_instructions = "You are a smart agent."
tools = ["shell"]

[agent.mcp_servers.github]

[agent.skills.config.reviewer]
`)

	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: smart-agent")
	assertContains(t, out, "You are a smart agent.")
	// MCP servers and skills should be extracted into canonical form
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "github")
	assertContains(t, out, "skills")
	assertContains(t, out, "reviewer")
}
