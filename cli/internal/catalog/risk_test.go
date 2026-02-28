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
}

func TestRiskIndicators_App_InstallScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/bin/sh\necho installed"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Apps,
		Path:  dir,
		Files: []string{"install.sh", "README.md"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Runs commands" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Runs commands")
	}
}

func TestRiskIndicators_App_SetupScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.sh"), []byte("#!/bin/sh\necho setup"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	item := ContentItem{
		Type:  Apps,
		Path:  dir,
		Files: []string{"setup.sh"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d: %+v", len(risks), risks)
	}
	if risks[0].Label != "Runs commands" {
		t.Errorf("Label = %q, want %q", risks[0].Label, "Runs commands")
	}
}

func TestRiskIndicators_App_NoScript(t *testing.T) {
	t.Parallel()
	item := ContentItem{
		Type:  Apps,
		Path:  t.TempDir(),
		Files: []string{"README.md", "config.json"},
	}
	risks := RiskIndicators(item)
	if len(risks) != 0 {
		t.Errorf("expected no risks for app without install script, got %d: %+v", len(risks), risks)
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
	item := ContentItem{Type: Prompts, Path: t.TempDir(), Files: []string{"PROMPT.md"}}
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
