package analyzer

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestIntegration_PAIStyleLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skills := []string{
		"Packs/redteam-skill/SKILL.md",
		"Packs/coding-skill/SKILL.md",
		"Packs/research-skill/SKILL.md",
		"Packs/writing-skill/SKILL.md",
		"Packs/analysis-skill/SKILL.md",
	}
	for _, s := range skills {
		setupFile(t, root, s, "---\nname: A Skill\ndescription: Does things\n---\nContent.\n")
	}

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	var skillCount int
	for _, item := range result.AllItems() {
		if item.Type == catalog.Skills {
			skillCount++
		}
	}
	if skillCount < 5 {
		t.Errorf("PAI-style: expected ≥5 skills detected, got %d", skillCount)
	}
	for _, item := range result.Auto {
		if item.Provider == "content-signal" {
			t.Errorf("content-signal item %q must not be in Auto bucket", item.Name)
		}
	}
}

func TestIntegration_BMADStyleLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agents := []string{
		"src/bmm/agents/orchestrator.agent.yaml",
		"src/bmm/agents/analyst.agent.yaml",
		"src/bmm/agents/writer.agent.yaml",
		"src/bmm/agents/reviewer.agent.yaml",
		"src/bmm/agents/planner.agent.yaml",
	}
	for _, a := range agents {
		setupFile(t, root, a, "name: Agent\ndescription: Does work\n")
	}

	an := New(DefaultConfig())
	result, err := an.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	var agentCount int
	for _, item := range result.AllItems() {
		if item.Type == catalog.Agents {
			agentCount++
		}
	}
	if agentCount < 5 {
		t.Errorf("BMAD-style: expected ≥5 agents detected, got %d", agentCount)
	}
}

func TestIntegration_NoFalsePositivesOnKnownGoodLayouts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, ".claude/agents/my-agent.md", "---\nname: My Agent\n---\nAgent body.\n")
	setupFile(t, root, ".claude/skills/my-skill/SKILL.md", "---\nname: My Skill\n---\nContent.\n")
	setupFile(t, root, ".claude/commands/run.md", "---\nallowed-tools: [Bash]\n---\nRun tests.\n")
	setupFile(t, root, "README.md", "# Project\nDocumentation.\n")
	setupFile(t, root, "CHANGELOG.md", "## v1.0.0\nInitial release.\n")

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}

	for _, item := range result.AllItems() {
		if item.Name == "README" || item.Name == "CHANGELOG" {
			t.Errorf("false positive: %q should not be classified as %v", item.Name, item.Type)
		}
	}

	seen := make(map[string][]string)
	for _, item := range result.AllItems() {
		seen[item.Name] = append(seen[item.Name], item.Provider)
	}
	for name, providers := range seen {
		if len(providers) > 1 {
			t.Errorf("item %q detected by multiple providers: %v", name, providers)
		}
	}
}

func TestIntegration_MCPAndHookSignals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "config/mcp-servers.json",
		`{"mcpServers": {"myserver": {"command": "npx", "args": ["-y", "@myserver/mcp"]}}}`)
	setupFile(t, root, "hooks/wiring.json",
		`{"hooks": {"PreToolUse": [{"command": "bash hooks/lint.sh"}], "PostToolUse": [{"command": "echo done"}]}}`)

	a := New(DefaultConfig())
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	var mcpCount, hookCount int
	for _, item := range result.AllItems() {
		switch item.Type {
		case catalog.MCP:
			mcpCount++
		case catalog.Hooks:
			hookCount++
		}
	}
	if mcpCount == 0 {
		t.Error("expected MCP item from non-standard mcp-servers.json")
	}
	if hookCount == 0 {
		t.Error("expected Hooks item from non-standard hooks wiring JSON")
	}
}

func TestIntegration_ScanAsPathBypass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	setupFile(t, root, "library/catalog/item-one.md", "---\nname: Item One\n---\nContent.\n")
	setupFile(t, root, "library/catalog/item-two.md", "---\nname: Item Two\n---\nContent.\n")

	cfg := DefaultConfig()
	cfg.ScanAsPaths = map[string]catalog.ContentType{
		"library/catalog/": catalog.Skills,
	}
	a := New(cfg)
	result, err := a.Analyze(root)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	var found int
	for _, item := range result.AllItems() {
		if item.Type == catalog.Skills {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected ≥2 skills from user-directed scan, got %d", found)
	}
	for _, item := range result.AllItems() {
		if item.Provider == "content-signal" && item.Confidence <= 0.55 {
			t.Errorf("user-directed item %q confidence %.2f should be elevated", item.Name, item.Confidence)
		}
	}
}
