package converter

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

func TestCanonicalizeCodexAgent_Single(t *testing.T) {
	input := []byte(`[features]
multi_agent = true

[agents.reviewer]
model = "o4-mini"
prompt = "You are a code reviewer. Check for bugs and style issues."
tools = ["shell", "view"]
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
	// Tools should be reverse-translated: shell → Bash, view → Read
	assertContains(t, out, "Bash")
	assertContains(t, out, "Read")
	// Verify the YAML tools list contains canonical names not Codex ones
	assertContains(t, out, "- Bash")
	assertContains(t, out, "- Read")
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
  - Read
  - Bash
  - Grep
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
	// Should contain TOML structure
	assertContains(t, out, "[features]")
	assertContains(t, out, "multi_agent = true")
	assertContains(t, out, "[agents.my-agent]")
	assertContains(t, out, `model = "claude-sonnet-4-6"`)
	// Tools translated: Read→view, Bash→shell, Grep→rg
	assertContains(t, out, "view")
	assertContains(t, out, "shell")
	assertContains(t, out, "rg")
	assertNotContains(t, out, "\"Read\"")
	assertNotContains(t, out, "\"Bash\"")
	// Prompt from body
	assertContains(t, out, "reviews code")
	// Filename
	assertEqual(t, "config.toml", result.Filename)

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
	// Start with Codex TOML → canonicalize → render back to Codex
	input := []byte(`[features]
multi_agent = true

[agents.coder]
model = "o4-mini"
prompt = "Write clean, tested code."
tools = ["shell", "apply_patch"]
`)

	conv := &AgentsConverter{}

	// Canonicalize
	canonical, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Verify canonical form has correct tool names
	canonicalStr := string(canonical.Content)
	assertContains(t, canonicalStr, "Bash")  // shell → Bash
	// apply_patch maps to both Write and Edit in Copilot CLI; reverse translation
	// picks one (map iteration order). Either is correct.
	if !strings.Contains(canonicalStr, "Write") && !strings.Contains(canonicalStr, "Edit") {
		t.Fatal("expected either Write or Edit for apply_patch reverse translation")
	}

	// Render back to Codex
	result, err := conv.Render(canonical.Content, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Should have TOML structure
	assertContains(t, out, "[features]")
	assertContains(t, out, "multi_agent = true")
	assertContains(t, out, "[agents.coder]")
	assertContains(t, out, `model = "o4-mini"`)
	// Tools should be back in Codex vocabulary
	assertContains(t, out, "shell")
	assertContains(t, out, "apply_patch")
	// Prompt preserved
	assertContains(t, out, "Write clean, tested code.")
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

	// Should have warnings for all unsupported fields
	expectedWarnings := []string{
		"maxTurns", "permissionMode", "skills", "mcpServers",
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
}
