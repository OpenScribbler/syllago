package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestPlacement_InstallFilesystemMethods(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)

	prov := testProvider("test")

	t.Run("symlink", func(t *testing.T) {
		sourcePath := filepath.Join(repoRoot, "rules", "test", "placement-symlink")
		if err := os.MkdirAll(sourcePath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# Symlink"), 0644); err != nil {
			t.Fatal(err)
		}
		item := catalog.ContentItem{Name: "placement-symlink", Type: catalog.Rules, Path: sourcePath}

		placement, err := Install(item, prov, repoRoot, MethodSymlink, "")
		if err != nil {
			t.Fatalf("Install symlink: %v", err)
		}

		expectedTarget := filepath.Join(tmp, ".testprovider", "rules", "placement-symlink")
		if placement.Mechanism != MechanismSymlink {
			t.Errorf("Mechanism = %q, want %q", placement.Mechanism, MechanismSymlink)
		}
		if placement.Path != expectedTarget {
			t.Errorf("Path = %q, want %q", placement.Path, expectedTarget)
		}
		if placement.String() != expectedTarget {
			t.Errorf("String() = %q, want %q", placement.String(), expectedTarget)
		}
	})

	t.Run("copy", func(t *testing.T) {
		sourcePath := filepath.Join(repoRoot, "rules", "test", "placement-copy")
		if err := os.MkdirAll(sourcePath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "rule.md"), []byte("# Copy"), 0644); err != nil {
			t.Fatal(err)
		}
		item := catalog.ContentItem{Name: "placement-copy", Type: catalog.Rules, Path: sourcePath}

		placement, err := Install(item, prov, repoRoot, MethodCopy, "")
		if err != nil {
			t.Fatalf("Install copy: %v", err)
		}

		expectedTarget := filepath.Join(tmp, ".testprovider", "rules", "placement-copy")
		if placement.Mechanism != MechanismCopy {
			t.Errorf("Mechanism = %q, want %q", placement.Mechanism, MechanismCopy)
		}
		if placement.Path != expectedTarget {
			t.Errorf("Path = %q, want %q", placement.Path, expectedTarget)
		}
		if placement.String() != expectedTarget {
			t.Errorf("String() = %q, want %q", placement.String(), expectedTarget)
		}
	})
}

func TestPlacement_UninstallSymlink(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)

	prov := testProvider("test")
	sourcePath := filepath.Join(repoRoot, "rules", "test", "placement-remove")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "placement-remove", Type: catalog.Rules, Path: sourcePath}

	targetPath := filepath.Join(tmp, ".testprovider", "rules", "placement-remove")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sourcePath, targetPath); err != nil {
		t.Fatal(err)
	}

	placement, err := Uninstall(item, prov, repoRoot)
	if err != nil {
		t.Fatalf("Uninstall symlink: %v", err)
	}
	if placement.Mechanism != MechanismSymlink {
		t.Errorf("Mechanism = %q, want %q", placement.Mechanism, MechanismSymlink)
	}
	if placement.Path != targetPath {
		t.Errorf("Path = %q, want %q", placement.Path, targetPath)
	}
	if placement.String() != targetPath {
		t.Errorf("String() = %q, want %q", placement.String(), targetPath)
	}
}

func TestPlacement_HookMergeInstallAndUninstall(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755); err != nil {
		t.Fatal(err)
	}

	hookDir := filepath.Join(projectRoot, "hooks", "placement-hook")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatal(err)
	}
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"PreToolUse","matcher":"Bash","handler":{"type":"command","command":"echo check"}}]}`
	if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "placement-hook", Type: catalog.Hooks, Path: hookDir}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	overrideHookSettingsPath(t, settingsPath)

	placement, err := installHook(item, provider.ClaudeCode, projectRoot)
	if err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if placement.Mechanism != MechanismHookMerge {
		t.Errorf("install Mechanism = %q, want %q", placement.Mechanism, MechanismHookMerge)
	}
	if placement.Path != settingsPath {
		t.Errorf("install Path = %q, want %q", placement.Path, settingsPath)
	}
	assertStringSet(t, placement.Keys, []string{"hooks.PreToolUse"})
	expectedInstall := "hooks.PreToolUse in " + settingsPath
	if placement.String() != expectedInstall {
		t.Errorf("install String() = %q, want %q", placement.String(), expectedInstall)
	}

	placement, err = uninstallHook(item, provider.ClaudeCode, projectRoot)
	if err != nil {
		t.Fatalf("uninstallHook: %v", err)
	}
	if placement.Mechanism != MechanismHookMerge {
		t.Errorf("uninstall Mechanism = %q, want %q", placement.Mechanism, MechanismHookMerge)
	}
	if placement.Path != settingsPath {
		t.Errorf("uninstall Path = %q, want %q", placement.Path, settingsPath)
	}
	assertStringSet(t, placement.Keys, []string{"hooks.PreToolUse"})
	expectedUninstall := "hooks.PreToolUse from " + settingsPath
	if placement.String() != expectedUninstall {
		t.Errorf("uninstall String() = %q, want %q", placement.String(), expectedUninstall)
	}
}

func TestPlacement_MCPMergeInstallAndUninstall(t *testing.T) {
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "placement-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON := `{
		"mcpServers": {
			"server-a": {"command": "node", "args": ["a.js"]},
			"server-b": {"url": "https://b.example.com"}
		}
	}`
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "placement-mcp", Type: catalog.MCP, Path: itemDir}

	cfgPath := filepath.Join(tmpDir, ".claude.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	originalFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, repoRoot string) (string, error) {
		return cfgPath, nil
	}
	t.Cleanup(func() { mcpConfigPath = originalFunc })

	prov := provider.Provider{Slug: "claude-code", Name: "Claude Code"}

	placement, err := installMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	if placement.Mechanism != MechanismMCPMerge {
		t.Errorf("install Mechanism = %q, want %q", placement.Mechanism, MechanismMCPMerge)
	}
	if placement.Path != cfgPath {
		t.Errorf("install Path = %q, want %q", placement.Path, cfgPath)
	}
	assertStringSet(t, placement.Keys, []string{"mcpServers.server-a", "mcpServers.server-b"})
	expectedInstall := "mcpServers in " + cfgPath
	if placement.String() != expectedInstall {
		t.Errorf("install String() = %q, want %q", placement.String(), expectedInstall)
	}

	placement, err = uninstallMCP(item, prov, tmpDir)
	if err != nil {
		t.Fatalf("uninstallMCP: %v", err)
	}
	if placement.Mechanism != MechanismMCPMerge {
		t.Errorf("uninstall Mechanism = %q, want %q", placement.Mechanism, MechanismMCPMerge)
	}
	if placement.Path != cfgPath {
		t.Errorf("uninstall Path = %q, want %q", placement.Path, cfgPath)
	}
	assertStringSet(t, placement.Keys, []string{"mcpServers.server-a", "mcpServers.server-b"})
	expectedUninstall := "mcpServers from " + cfgPath
	if placement.String() != expectedUninstall {
		t.Errorf("uninstall String() = %q, want %q", placement.String(), expectedUninstall)
	}
}

func assertStringSet(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("keys length = %d, want %d; got %v", len(got), len(want), got)
	}
	seen := make(map[string]int, len(got))
	for _, key := range got {
		seen[key]++
	}
	for _, key := range want {
		if seen[key] == 0 {
			t.Fatalf("missing key %q in %v", key, got)
		}
		seen[key]--
	}
	for key, count := range seen {
		if count != 0 {
			t.Fatalf("unexpected key %q in %v", key, got)
		}
	}
}
