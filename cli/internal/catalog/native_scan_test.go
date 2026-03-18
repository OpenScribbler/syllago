package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanNativeContent_SyllagoStructure(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("name: test"), 0644)
	result := ScanNativeContent(dir)
	if !result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=true when registry.yaml present")
	}
}

func TestScanNativeContent_SyllagoContentDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "rules"), 0755)
	result := ScanNativeContent(dir)
	if !result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=true when content dir present")
	}
}

func TestScanNativeContent_CursorRules(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("should not be syllago structure")
	}
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	if result.Providers[0].ProviderSlug != "cursor" {
		t.Errorf("expected cursor, got %s", result.Providers[0].ProviderSlug)
	}
	items := result.Providers[0].Items["rules"]
	if len(items) != 1 {
		t.Fatalf("expected 1 rules item, got %d", len(items))
	}
	if items[0].Name != ".cursorrules" {
		t.Errorf("expected name '.cursorrules', got %q", items[0].Name)
	}
}

func TestScanNativeContent_MultiProvider(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
	os.WriteFile(filepath.Join(dir, ".windsurfrules"), []byte("# rules"), 0644)
	result := ScanNativeContent(dir)
	if len(result.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result.Providers))
	}
}

func TestScanNativeContent_ClaudeCodeDir(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".claude", "commands")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "deploy.md"), []byte("# deploy"), 0644)
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("should not be syllago structure")
	}
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	p := result.Providers[0]
	if p.ProviderSlug != "claude-code" {
		t.Errorf("expected claude-code, got %s", p.ProviderSlug)
	}
	items := p.Items["commands"]
	if len(items) != 1 {
		t.Fatalf("expected 1 command item, got %d", len(items))
	}
	if items[0].Name != "deploy" {
		t.Errorf("expected name 'deploy', got %q", items[0].Name)
	}
	if items[0].Path != ".claude/commands/deploy.md" {
		t.Errorf("expected path '.claude/commands/deploy.md', got %q", items[0].Path)
	}
}

func TestScanNativeContent_Empty(t *testing.T) {
	dir := t.TempDir()
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure || len(result.Providers) != 0 {
		t.Error("expected empty result for empty directory")
	}
}

func TestScanNativeContent_StructuredItems(t *testing.T) {
	dir := t.TempDir()

	// Skill with frontmatter
	skillDir := filepath.Join(dir, ".claude", "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "my-skill.md"), []byte("---\nname: My Skill\ndescription: Does things\n---\n\nBody."), 0644)

	// Agent with frontmatter
	agentDir := filepath.Join(dir, ".claude", "agents")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "my-agent.md"), []byte("---\nname: My Agent\ndescription: An agent\n---\n\nBody."), 0644)

	result := ScanNativeContent(dir)
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	p := result.Providers[0]
	if p.ProviderSlug != "claude-code" {
		t.Errorf("expected claude-code, got %s", p.ProviderSlug)
	}

	skills := p.Items["skills"]
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("skill Name = %q, want 'my-skill'", skills[0].Name)
	}
	if skills[0].DisplayName != "My Skill" {
		t.Errorf("skill DisplayName = %q, want 'My Skill'", skills[0].DisplayName)
	}
	if skills[0].Description != "Does things" {
		t.Errorf("skill Description = %q, want 'Does things'", skills[0].Description)
	}
	if skills[0].Path != ".claude/skills/my-skill.md" {
		t.Errorf("skill Path = %q, want '.claude/skills/my-skill.md'", skills[0].Path)
	}

	agents := p.Items["agents"]
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "my-agent" {
		t.Errorf("agent Name = %q, want 'my-agent'", agents[0].Name)
	}
	if agents[0].DisplayName != "My Agent" {
		t.Errorf("agent DisplayName = %q, want 'My Agent'", agents[0].DisplayName)
	}
}

func TestScanNativeContent_ProjectScopedHooks(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	settingsJSON := `{
		"hooks": {
			"PostToolUse": [
				{"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "echo lint"}]},
				{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo bash-hook"}]}
			],
			"PreToolUse": [
				{"matcher": "Read", "hooks": [{"type": "command", "command": "echo pre"}]}
			]
		}
	}`
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsJSON), 0644)

	result := ScanNativeContent(dir)
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	p := result.Providers[0]
	if p.ProviderSlug != "claude-code" {
		t.Errorf("expected claude-code, got %s", p.ProviderSlug)
	}

	hooks := p.Items["hooks"]
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hook items (2 PostToolUse + 1 PreToolUse), got %d", len(hooks))
	}

	// Verify all have the settings.json path
	for _, h := range hooks {
		if h.Path != ".claude/settings.json" {
			t.Errorf("hook Path = %q, want '.claude/settings.json'", h.Path)
		}
		if h.HookEvent == "" {
			t.Errorf("hook HookEvent should not be empty, got item: %+v", h)
		}
	}

	// Check we have items from both events
	events := make(map[string]int)
	for _, h := range hooks {
		events[h.HookEvent]++
	}
	if events["PostToolUse"] != 2 {
		t.Errorf("expected 2 PostToolUse hooks, got %d", events["PostToolUse"])
	}
	if events["PreToolUse"] != 1 {
		t.Errorf("expected 1 PreToolUse hook, got %d", events["PreToolUse"])
	}
}

func TestScanNativeContent_ProjectScopedMCP(t *testing.T) {
	dir := t.TempDir()
	mcpDir := filepath.Join(dir, ".copilot")
	os.MkdirAll(mcpDir, 0755)
	mcpJSON := `{
		"mcpServers": {
			"filesystem": {"command": "npx", "args": ["-y", "@mcp/server-filesystem"]},
			"github": {"command": "npx", "args": ["-y", "@mcp/server-github"]}
		}
	}`
	os.WriteFile(filepath.Join(mcpDir, "mcp.json"), []byte(mcpJSON), 0644)

	result := ScanNativeContent(dir)
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	p := result.Providers[0]
	if p.ProviderSlug != "copilot-cli" {
		t.Errorf("expected copilot-cli, got %s", p.ProviderSlug)
	}

	mcpItems := p.Items["mcp"]
	if len(mcpItems) != 2 {
		t.Fatalf("expected 2 MCP server items, got %d", len(mcpItems))
	}

	names := make(map[string]bool)
	for _, item := range mcpItems {
		names[item.Name] = true
		if item.Path != ".copilot/mcp.json" {
			t.Errorf("MCP item Path = %q, want '.copilot/mcp.json'", item.Path)
		}
	}
	if !names["filesystem"] {
		t.Error("expected 'filesystem' MCP server item")
	}
	if !names["github"] {
		t.Error("expected 'github' MCP server item")
	}
}

func TestScanNativeContent_HooksNoContent(t *testing.T) {
	// settings.json with no hooks section should not produce any hook items
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"model": "claude-opus-4"}`), 0644)

	result := ScanNativeContent(dir)
	// Provider should not appear if no items were found
	for _, p := range result.Providers {
		if p.ProviderSlug == "claude-code" {
			if len(p.Items["hooks"]) != 0 {
				t.Errorf("expected 0 hook items for settings.json without hooks, got %d", len(p.Items["hooks"]))
			}
		}
	}
}
