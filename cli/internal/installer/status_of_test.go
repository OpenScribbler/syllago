package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// StatusOf reports an error whenever the status it returns is a guess, and
// CheckStatus returns the same guess without one.
func TestStatusOf_ReportsUnreadableState(t *testing.T) {
	// Not parallel — uses t.Setenv and mutates package-level mcpConfigPath
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	prov := testProvider("test")

	t.Run("unsearchable install dir", func(t *testing.T) {
		rulesDir := filepath.Join(tmp, ".testprovider", "rules")
		if err := os.MkdirAll(rulesDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(rulesDir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(rulesDir, 0755) })
		item := catalog.ContentItem{Name: "hidden-rule", Type: catalog.Rules, Path: filepath.Join(repoRoot, "hidden-rule")}

		requireGuess(t, item, prov, repoRoot, StatusNotInstalled)
	})

	t.Run("install dir is a symlink loop", func(t *testing.T) {
		agentsDir := filepath.Join(tmp, ".testprovider", "agents")
		if err := os.Symlink(agentsDir, agentsDir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(agentsDir) })
		item := catalog.ContentItem{Name: "loop-agent", Type: catalog.Agents, Path: filepath.Join(repoRoot, "loop-agent")}

		requireGuess(t, item, prov, repoRoot, StatusNotInstalled)
	})

	t.Run("unsearchable legacy install dir", func(t *testing.T) {
		legacyDir := filepath.Join(tmp, ".legacyprovider", "skills")
		if err := os.MkdirAll(legacyDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(legacyDir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(legacyDir, 0755) })
		legacyProv := prov
		legacyProv.LegacyInstallDir = func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, ".legacyprovider", string(ct))
		}
		item := catalog.ContentItem{Name: "old-skill", Type: catalog.Skills, Path: filepath.Join(repoRoot, "old-skill")}

		requireGuess(t, item, legacyProv, repoRoot, StatusNotInstalled)
	})

	t.Run("MCP config is a directory", func(t *testing.T) {
		cfgDir := filepath.Join(tmp, "mcp-config-dir")
		if err := os.MkdirAll(cfgDir, 0755); err != nil {
			t.Fatal(err)
		}
		orig := mcpConfigPath
		mcpConfigPath = func(provider.Provider, string) (string, error) { return cfgDir, nil }
		t.Cleanup(func() { mcpConfigPath = orig })
		item := catalog.ContentItem{Name: "srv", Type: catalog.MCP, ServerKey: "srv"}

		requireGuess(t, item, prov, repoRoot, StatusNotAvailable)
	})

	t.Run("hook manifest missing", func(t *testing.T) {
		item := catalog.ContentItem{Name: "gone", Type: catalog.Hooks, Path: filepath.Join(tmp, "no-such-hook")}

		requireGuess(t, item, prov, repoRoot, StatusNotAvailable)
	})

	cfgFile := filepath.Join(tmp, "mcp.json")
	if err := os.WriteFile(cfgFile, []byte(`{"mcpServers":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	useCfgFile := func(t *testing.T) {
		orig := mcpConfigPath
		mcpConfigPath = func(provider.Provider, string) (string, error) { return cfgFile, nil }
		t.Cleanup(func() { mcpConfigPath = orig })
	}

	t.Run("MCP installed.json corrupt", func(t *testing.T) {
		useCfgFile(t)
		corruptRoot := filepath.Join(tmp, "corrupt-repo")
		writeCorruptInstalled(t, corruptRoot)
		item := catalog.ContentItem{Name: "srv", Type: catalog.MCP}

		requireGuess(t, item, prov, corruptRoot, StatusNotInstalled)
	})

	// The legacy content root's installed.json is checked after the
	// project's, so an unreadable one leaves the status a guess too.
	t.Run("legacy installed.json corrupt", func(t *testing.T) {
		useCfgFile(t)
		writeCorruptInstalled(t, catalog.GlobalContentDir())

		requireGuess(t, catalog.ContentItem{Name: "srv", Type: catalog.MCP}, prov, repoRoot, StatusNotInstalled)
		hook, _ := writeHookItem(t, "shell")
		requireGuess(t, hook, provider.ClaudeCode, repoRoot, StatusNotInstalled)
	})

	t.Run("not installed is not a guess", func(t *testing.T) {
		item := catalog.ContentItem{Name: "absent", Type: catalog.Skills, Path: filepath.Join(repoRoot, "absent")}
		status, err := StatusOf(item, prov, repoRoot)
		if err != nil || status != StatusNotInstalled {
			t.Errorf("StatusOf = %v, %v; want %v, nil", status, err, StatusNotInstalled)
		}
	})
}

func requireGuess(t *testing.T, item catalog.ContentItem, prov provider.Provider, repoRoot string, want Status) {
	t.Helper()
	status, err := StatusOf(item, prov, repoRoot)
	if err == nil {
		t.Errorf("StatusOf err = nil, want an error for unreadable state")
	}
	if status != want {
		t.Errorf("StatusOf status = %v, want %v", status, want)
	}
	if got := CheckStatus(item, prov, repoRoot); got != want {
		t.Errorf("CheckStatus = %v, want %v", got, want)
	}
}

// writeCorruptInstalled writes an installed.json under root that does not
// parse, and removes it when the test ends.
func writeCorruptInstalled(t *testing.T, root string) {
	t.Helper()
	path := installedPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })
}
