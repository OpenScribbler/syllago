package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// E2E tests for the "add" pathway (provider → library).
//
// These tests verify that syllago correctly reads content from provider
// locations and writes it to the library. This is the IMPORT direction,
// which is the opposite of the INSTALL direction tested in the installer
// package. Both hooks and MCP configs use JSON merge (not filesystem),
// so they need special handling in the add pathway.

// --- Fixture builders ---

// settingsWithHooksAndMCP is a settings.json with both hooks and MCP.
const settingsWithHooksAndMCP = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo validate"}]
      }
    ]
  },
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {"GITHUB_TOKEN": "placeholder"}
    }
  }
}`

// globalSettingsWithMCP is a global settings.json with a different MCP server.
const globalSettingsWithMCP = `{
  "hooks": {
    "Stop": [
      {
        "hooks": [{"type": "command", "command": "echo done"}]
      }
    ]
  },
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    }
  }
}`

// collisionSettingsGlobal has an "obsidian" server at global scope.
const collisionSettingsGlobal = `{
  "mcpServers": {
    "obsidian": {
      "command": "npx",
      "args": ["-y", "obsidian-mcp"],
      "env": {"VAULT": "/global/vault"}
    }
  }
}`

// collisionSettingsProject has an "obsidian" server at project scope.
const collisionSettingsProject = `{
  "mcpServers": {
    "obsidian": {
      "command": "npx",
      "args": ["-y", "obsidian-mcp"],
      "env": {"VAULT": "/project/vault"}
    }
  }
}`

// setupDualScopeProject creates a project root and a fake home directory
// with both global and project settings.json files.
// Returns (projectRoot, fakeHome).
func setupDualScopeProject(t *testing.T, globalSettings, projectSettings string) (string, string) {
	t.Helper()
	projectRoot := t.TempDir()
	fakeHome := t.TempDir()

	// Global: fakeHome/.claude/settings.json
	globalDir := filepath.Join(fakeHome, ".claude")
	os.MkdirAll(globalDir, 0755)
	os.WriteFile(filepath.Join(globalDir, "settings.json"), []byte(globalSettings), 0644)

	// Project: projectRoot/.claude/settings.json
	projectDir := filepath.Join(projectRoot, ".claude")
	os.MkdirAll(projectDir, 0755)
	os.WriteFile(filepath.Join(projectDir, "settings.json"), []byte(projectSettings), 0644)

	return projectRoot, fakeHome
}

// --- E2E: Hooks add pathway ---

func TestAddHooksE2E_BothScopes(t *testing.T) {
	projectRoot, fakeHome := setupDualScopeProject(t,
		globalSettingsWithMCP,   // global has Stop hook
		settingsWithHooksAndMCP, // project has PreToolUse hook
	)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "all")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("base-dir", fakeHome)
	t.Cleanup(func() { addCmd.Flags().Set("base-dir", "") })

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks (both scopes) failed: %v", err)
	}

	out := stdout.String()

	// Should have added hooks from both scopes.
	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected hooks directory: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 hooks (from both scopes), got %d", len(entries))
	}

	// Verify we have hooks with different scopes in metadata.
	scopes := make(map[string]bool)
	for _, entry := range entries {
		m, _ := metadata.Load(filepath.Join(hooksBase, entry.Name()))
		if m != nil {
			scopes[m.SourceScope] = true
		}
	}
	if !scopes["global"] {
		t.Error("expected at least one hook with source_scope=global")
	}
	if !scopes["project"] {
		t.Error("expected at least one hook with source_scope=project")
	}

	// Output should mention both scopes.
	if !strings.Contains(out, "global") {
		t.Error("expected 'global' in output")
	}
	if !strings.Contains(out, "project") {
		t.Error("expected 'project' in output")
	}
}

func TestAddHooksE2E_ScopeFilterGlobal(t *testing.T) {
	projectRoot, fakeHome := setupDualScopeProject(t,
		globalSettingsWithMCP,
		settingsWithHooksAndMCP,
	)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "global")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("base-dir", fakeHome)
	t.Cleanup(func() {
		addCmd.Flags().Set("base-dir", "")
		addCmd.Flags().Set("scope", "all")
	})

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks --scope=global failed: %v", err)
	}

	// Only global hooks should exist.
	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, _ := os.ReadDir(hooksBase)
	for _, entry := range entries {
		m, _ := metadata.Load(filepath.Join(hooksBase, entry.Name()))
		if m != nil && m.SourceScope == "project" {
			t.Errorf("found project-scoped hook %q when --scope=global was used", entry.Name())
		}
	}
}

// --- E2E: MCP add pathway ---

func TestAddMcpE2E_BothScopes(t *testing.T) {
	projectRoot, fakeHome := setupDualScopeProject(t,
		globalSettingsWithMCP,   // global has "filesystem"
		settingsWithHooksAndMCP, // project has "github"
	)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "all")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("base-dir", fakeHome)
	t.Cleanup(func() { addCmd.Flags().Set("base-dir", "") })

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp (both scopes) failed: %v", err)
	}

	out := stdout.String()

	// Should have both servers.
	mcpBase := filepath.Join(globalDir, "mcp", "claude-code")
	entries, err := os.ReadDir(mcpBase)
	if err != nil {
		t.Fatalf("expected mcp directory: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 MCP servers (from both scopes), got %d", len(entries))
	}

	// Verify server names.
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names["github"] {
		t.Error("expected 'github' MCP server from project scope")
	}
	if !names["filesystem"] {
		t.Error("expected 'filesystem' MCP server from global scope")
	}

	// Verify config.json format for each.
	for _, name := range []string{"github", "filesystem"} {
		data, err := os.ReadFile(filepath.Join(mcpBase, name, "config.json"))
		if err != nil {
			t.Errorf("reading %s config.json: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), `"mcpServers"`) {
			t.Errorf("%s config.json missing nested mcpServers wrapper", name)
		}
	}

	// Verify scope metadata.
	githubMeta, _ := metadata.Load(filepath.Join(mcpBase, "github"))
	if githubMeta == nil || githubMeta.SourceScope != "project" {
		t.Errorf("expected github source_scope=project, got %v", githubMeta)
	}
	fsMeta, _ := metadata.Load(filepath.Join(mcpBase, "filesystem"))
	if fsMeta == nil || fsMeta.SourceScope != "global" {
		t.Errorf("expected filesystem source_scope=global, got %v", fsMeta)
	}

	if !strings.Contains(out, "github") || !strings.Contains(out, "filesystem") {
		t.Errorf("expected both servers in output, got: %s", out)
	}
}

func TestAddMcpE2E_NameCollision(t *testing.T) {
	// Both global and project have "obsidian" — should get unique paths.
	projectRoot, fakeHome := setupDualScopeProject(t,
		collisionSettingsGlobal,
		collisionSettingsProject,
	)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "all")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("base-dir", fakeHome)
	t.Cleanup(func() { addCmd.Flags().Set("base-dir", "") })

	if err := addCmd.RunE(addCmd, []string{"mcp"}); err != nil {
		t.Fatalf("add mcp (collision) failed: %v", err)
	}

	mcpBase := filepath.Join(globalDir, "mcp", "claude-code")
	entries, _ := os.ReadDir(mcpBase)

	// Find the obsidian entries specifically (test may also pick up real
	// ~/.claude.json servers, which is expected behavior).
	obsidianDirs := make(map[string]string) // dir name → scope
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "obsidian") {
			continue
		}
		m, _ := metadata.Load(filepath.Join(mcpBase, entry.Name()))
		if m != nil {
			obsidianDirs[entry.Name()] = m.SourceScope
		}
	}
	if len(obsidianDirs) < 2 {
		t.Fatalf("expected at least 2 obsidian directories (collision), got %d: %v", len(obsidianDirs), obsidianDirs)
	}

	// Verify we have one global and one project among the obsidian entries.
	hasGlobal, hasProject := false, false
	for _, scope := range obsidianDirs {
		if scope == "global" {
			hasGlobal = true
		}
		if scope == "project" {
			hasProject = true
		}
	}
	if !hasGlobal || !hasProject {
		t.Errorf("expected both global and project scopes in obsidian entries, got: %v", obsidianDirs)
	}

	// Verify config.json contents differ (different VAULT paths).
	for dirName, scope := range obsidianDirs {
		data, _ := os.ReadFile(filepath.Join(mcpBase, dirName, "config.json"))
		if scope == "global" && !strings.Contains(string(data), "/global/vault") {
			t.Errorf("global obsidian should have /global/vault path, got: %s", data)
		}
		if scope == "project" && !strings.Contains(string(data), "/project/vault") {
			t.Errorf("project obsidian should have /project/vault path, got: %s", data)
		}
	}
}

// --- E2E: Discovery pathway ---

func TestAddDiscoveryE2E_ShowsAllTypes(t *testing.T) {
	// Create a project with rules, hooks, and MCP.
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	// Rules
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security"), 0644)

	// Hooks + MCP in settings.json
	os.WriteFile(filepath.Join(projectRoot, ".claude", "settings.json"), []byte(settingsWithHooksAndMCP), 0644)

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	// Parse JSON and verify all types present.
	var result struct {
		Provider string `json:"provider"`
		Groups   []struct {
			Type  string `json:"type"`
			Count int    `json:"count"`
			Items []struct {
				Name  string `json:"name"`
				Scope string `json:"scope,omitempty"`
			} `json:"items"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, stdout.String())
	}

	typesSeen := make(map[string]bool)
	for _, g := range result.Groups {
		typesSeen[g.Type] = true
	}

	if !typesSeen["rules"] {
		t.Error("expected 'rules' in discovery output")
	}
	if !typesSeen["hooks"] {
		t.Error("expected 'hooks' in discovery output")
	}
	if !typesSeen["mcp"] {
		t.Error("expected 'mcp' in discovery output")
	}

	// Verify MCP items have scope.
	for _, g := range result.Groups {
		if g.Type == "mcp" {
			for _, item := range g.Items {
				if item.Scope == "" {
					t.Errorf("MCP item %q missing scope in discovery JSON", item.Name)
				}
			}
		}
	}

	// No files should be written in discovery mode.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Error("discovery should not write files")
	}
}

// --- E2E: --all flag ---

func TestAddAllE2E_IncludesHooksAndMcp(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()

	// Rules
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security"), 0644)

	// Hooks + MCP
	os.WriteFile(filepath.Join(projectRoot, ".claude", "settings.json"), []byte(settingsWithHooksAndMCP), 0644)

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "true")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { addCmd.Flags().Set("all", "false") })

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add --all failed: %v", err)
	}

	// Should have rules.
	rulesBase := filepath.Join(globalDir, "rules", "claude-code")
	if _, err := os.Stat(rulesBase); err != nil {
		t.Error("expected rules directory in library")
	}

	// Should have hooks.
	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	if _, err := os.Stat(hooksBase); err != nil {
		t.Error("expected hooks directory in library")
	}

	// Should have MCP.
	mcpBase := filepath.Join(globalDir, "mcp", "claude-code")
	if _, err := os.Stat(mcpBase); err != nil {
		t.Error("expected mcp directory in library")
	}
}
