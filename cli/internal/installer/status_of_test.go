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
