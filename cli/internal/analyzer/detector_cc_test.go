package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestClaudeCodeDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &ClaudeCodeDetector{}
	if got := d.ProviderSlug(); got != "claude-code" {
		t.Errorf("ProviderSlug() = %q, want %q", got, "claude-code")
	}
}

func TestClaudeCodeDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &ClaudeCodeDetector{}
	pats := d.Patterns()
	if len(pats) != 9 {
		t.Fatalf("Patterns() returned %d, want 9", len(pats))
	}
}

func TestClaudeCodeDetector_ClassifySettings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	settings := `{
  "hooks": {
    "PreToolUse": [{"matcher":"Bash","command":"bun hooks/validate.ts $FILE"}],
    "PostToolUse": [{"command":"echo done"}]
  }
}`
	setupFile(t, root, ".claude/settings.json", settings)
	setupFile(t, root, "hooks/validate.ts", "// validation script\n")

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".claude/settings.json", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 hook items, got %d", len(items))
	}

	// First hook should reference the script.
	if len(items[0].Scripts) != 1 || items[0].Scripts[0] != "hooks/validate.ts" {
		t.Errorf("items[0].Scripts = %v, want [hooks/validate.ts]", items[0].Scripts)
	}
	if items[0].Type != catalog.Hooks {
		t.Errorf("items[0].Type = %q, want %q", items[0].Type, catalog.Hooks)
	}
	if items[0].ConfigSource != ".claude/settings.json" {
		t.Errorf("items[0].ConfigSource = %q, want %q", items[0].ConfigSource, ".claude/settings.json")
	}

	// Second hook is inline — no scripts.
	if len(items[1].Scripts) != 0 {
		t.Errorf("items[1].Scripts = %v, want empty", items[1].Scripts)
	}
}

func TestClaudeCodeDetector_ClassifyHookScript(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".claude/hooks/lint.sh", "#!/bin/sh\neslint .\n")

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".claude/hooks/lint.sh", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].InternalLabel != "hook-script" {
		t.Errorf("InternalLabel = %q, want %q", items[0].InternalLabel, "hook-script")
	}
	if items[0].Type != catalog.Hooks {
		t.Errorf("Type = %q, want %q", items[0].Type, catalog.Hooks)
	}
}

func TestClaudeCodeDetector_ClassifyMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".mcp.json", `{
  "mcpServers": {
    "github": {"command": "gh", "args": ["copilot"]},
    "slack": {"command": "slack-mcp", "args": ["--token", "xxx"]}
  }
}`)

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".mcp.json", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 MCP items, got %d", len(items))
	}

	names := map[string]bool{}
	for _, item := range items {
		names[item.Name] = true
		if item.Type != catalog.MCP {
			t.Errorf("Type = %q, want %q for %s", item.Type, catalog.MCP, item.Name)
		}
	}
	if !names["github"] {
		t.Error("expected MCP item named 'github'")
	}
	if !names["slack"] {
		t.Error("expected MCP item named 'slack'")
	}
}

func TestClaudeCodeDetector_ClassifyAgent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".claude/agents/reviewer.md",
		"---\nname: Code Reviewer\ndescription: Reviews PRs\n---\nBody.\n")

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".claude/agents/reviewer.md", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != catalog.Agents {
		t.Errorf("Type = %q, want %q", items[0].Type, catalog.Agents)
	}
	if items[0].DisplayName != "Code Reviewer" {
		t.Errorf("DisplayName = %q, want %q", items[0].DisplayName, "Code Reviewer")
	}
}

func TestClaudeCodeDetector_SettingsInlineCommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [{"command":"echo foo"}]
  }
}`)

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".claude/settings.json", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Scripts) != 0 {
		t.Errorf("Scripts = %v, want empty for inline command", items[0].Scripts)
	}
}

func TestClaudeCodeDetector_MalformedSettings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".claude/settings.json", "not json at all {{{")

	d := &ClaudeCodeDetector{}
	items, err := d.Classify(".claude/settings.json", root)
	if err != nil {
		t.Fatalf("Classify should not error on malformed JSON: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items for malformed settings, got %d", len(items))
	}
}
