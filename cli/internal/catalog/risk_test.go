package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRiskIndicators_Hook_Command(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Real Claude Code hook format: entries use "command" field, not "type"
	hookJSON := `{"hooks":{"PostToolUse":[{"matcher":"Write|Edit","command":"echo hi"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Hooks,
		Path:  dir,
		Files: []string{"hook.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Runs commands" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Runs commands")
	}
	if risks[0].Level != RiskHigh {
		t.Errorf("Level = %d, want RiskHigh (%d)", risks[0].Level, RiskHigh)
	}
	if len(risks[0].Lines) == 0 {
		t.Error("expected Lines to be populated for command hook")
	} else if risks[0].Lines[0].File != "hook.json" {
		t.Errorf("Lines[0].File = %q, want %q", risks[0].Lines[0].File, "hook.json")
	}
}

func TestRiskIndicators_Hook_URL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookJSON := `{"hooks":{"PreToolUse":[{"matcher":"Bash","url":"https://example.com/hook"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Hooks,
		Path:  dir,
		Files: []string{"hook.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Network access" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Network access")
	}
	if risks[0].Level != RiskMedium {
		t.Errorf("Level = %d, want RiskMedium (%d)", risks[0].Level, RiskMedium)
	}
	if len(risks[0].Lines) == 0 {
		t.Error("expected Lines to be populated for URL hook")
	} else if risks[0].Lines[0].File != "hook.json" {
		t.Errorf("Lines[0].File = %q, want %q", risks[0].Lines[0].File, "hook.json")
	}
}

func TestRiskIndicators_Hook_CommandAndURL_Deduped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Two hooks: one with command, one with url → two distinct risk labels
	hookJSON := `{"hooks":{"PostToolUse":[{"matcher":"Write","command":"echo hi"},{"matcher":"Read","url":"https://example.com"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Hooks,
		Path:  dir,
		Files: []string{"hook.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 2 {
		t.Fatalf("expected 2 risks (Runs commands + Network access), got %d: %+v", len(risks), risks)
	}
	for _, r := range risks {
		switch r.Label {
		case "Runs commands":
			if r.Level != RiskHigh {
				t.Errorf("Runs commands: Level = %d, want RiskHigh", r.Level)
			}
		case "Network access":
			if r.Level != RiskMedium {
				t.Errorf("Network access: Level = %d, want RiskMedium", r.Level)
			}
		}
	}
}

func TestRiskIndicators_MCP_WithEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mcpJSON := `{"mcpServers":{"db":{"command":"npx","env":{"DB_URL":"postgres://..."}}}}`
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  MCP,
		Path:  dir,
		Files: []string{"mcp.json"},
	}
	risks := RiskIndicators(item)
	labels := make(map[string]bool, len(risks))
	for _, r := range risks {
		labels[r.Label] = true
	}
	if !labels["Network access"] {
		t.Error("expected risk label 'Network access'")
	}
	if !labels["Environment variables"] {
		t.Error("expected risk label 'Environment variables'")
	}
	for _, r := range risks {
		switch r.Label {
		case "Network access":
			if r.Level != RiskMedium {
				t.Errorf("Network access: Level = %d, want RiskMedium", r.Level)
			}
		case "Environment variables":
			if r.Level != RiskMedium {
				t.Errorf("Environment variables: Level = %d, want RiskMedium", r.Level)
			}
			if len(r.Lines) == 0 {
				t.Error("expected Lines for Environment variables risk")
			} else if r.Lines[0].File != "mcp.json" {
				t.Errorf("Lines[0].File = %q, want %q", r.Lines[0].File, "mcp.json")
			}
		}
	}
}

func TestRiskIndicators_MCP_NoEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mcpJSON := `{"mcpServers":{"simple":{"command":"npx","args":["@example/mcp"]}}}`
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte(mcpJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  MCP,
		Path:  dir,
		Files: []string{"mcp.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk (Network access only), got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Network access" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Network access")
	}
	if risks[0].Level != RiskMedium {
		t.Errorf("Level = %d, want RiskMedium (%d)", risks[0].Level, RiskMedium)
	}
}

func TestRiskIndicators_Skill_WithBash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "---\nname: My Skill\n---\n\nUse the Bash tool to run commands.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Skills,
		Path:  dir,
		Files: []string{"SKILL.md"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Bash access" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Bash access")
	}
	if risks[0].Level != RiskHigh {
		t.Errorf("Level = %d, want RiskHigh (%d)", risks[0].Level, RiskHigh)
	}
	if len(risks[0].Lines) == 0 {
		t.Error("expected Lines for Bash access risk")
	} else {
		if risks[0].Lines[0].File != "SKILL.md" {
			t.Errorf("Lines[0].File = %q, want %q", risks[0].Lines[0].File, "SKILL.md")
		}
		// "Bash" appears on line 5: "Use the Bash tool to run commands."
		if risks[0].Lines[0].Line != 5 {
			t.Errorf("Lines[0].Line = %d, want 5", risks[0].Lines[0].Line)
		}
	}
}

func TestRiskIndicators_Agent_WithBash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "# Agent\n\nThis agent uses Bash to execute tasks.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Agents,
		Path:  dir,
		Files: []string{"AGENT.md"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Bash access" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Bash access")
	}
}

func TestRiskIndicators_Skill_NoBash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "---\nname: Safe Skill\n---\n\nUse the Read and Grep tools only.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Skills,
		Path:  dir,
		Files: []string{"SKILL.md"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 0 {
		t.Errorf("expected no risks for skill without Bash, got %d: %+v", len(risks), risks)
	}
}

func TestRiskIndicators_NoRisk(t *testing.T) {
	t.Parallel()
	item := ContentItem{Type: Rules, Path: t.TempDir(), Files: []string{"rule.md"}}
	risks := RiskIndicators(item)
	if len(risks) != 0 {
		t.Errorf("expected no risks, got %d: %+v", len(risks), risks)
	}
}

func TestRiskIndicators_Rules_NoRisk(t *testing.T) {
	t.Parallel()
	item := ContentItem{Type: Rules, Path: t.TempDir(), Files: []string{"rule.md"}}
	risks := RiskIndicators(item)
	if len(risks) != 0 {
		t.Errorf("expected no risks for rules, got %d: %+v", len(risks), risks)
	}
}

func TestRiskIndicators_LinesAccurate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Multi-line hook JSON with "command" on a specific line (line 5).
	hookJSON := `{
  "hooks": {
    "PostToolUse": [
      {
        "command": "echo hello",
        "matcher": "Write"
      }
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Hooks,
		Path:  dir,
		Files: []string{"hooks.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if len(risks[0].Lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(risks[0].Lines), risks[0].Lines)
	}
	rl := risks[0].Lines[0]
	if rl.File != "hooks.json" {
		t.Errorf("File = %q, want %q", rl.File, "hooks.json")
	}
	if rl.Line != 5 {
		t.Errorf("Line = %d, want 5", rl.Line)
	}
}
