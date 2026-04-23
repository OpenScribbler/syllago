package loadout

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// requireGolden compares actual against testdata/<name>.golden.
// Run with -update-golden to regenerate golden files.
func requireGolden(t *testing.T, name string, actual string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")

	if *updateGolden {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found -- run with -update-golden to create it: %v", goldenPath, err)
	}
	if actual != string(expected) {
		t.Errorf("golden file mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, actual, string(expected))
	}
}

// TestGolden_HookMerge tests that hook merge produces the expected settings.json
// output when merging a loadout's hook into a file with existing hooks.
//
// Why golden tests: JSON merge output depends on sjson's formatting behavior.
// A golden file catches unintended changes to the output format, which matters
// because settings.json is a user-facing file.
func TestGolden_HookMerge(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	projectRoot := t.TempDir()

	// Create .syllago dir
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Provider directories
	os.MkdirAll(filepath.Join(homeDir, ".claude", "rules"), 0755)

	// Create settings.json with pre-existing hooks
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	existingSettings := `{
  "userPref": "dark-mode",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*.go",
        "hooks": [
          {
            "type": "command",
            "command": "go vet"
          }
        ]
      }
    ]
  }
}`
	os.WriteFile(settingsPath, []byte(existingSettings), 0644)

	// Create a hook that will be merged
	hookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "golden-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{
  "spec": "hooks/0.1",
  "hooks": [
    {
      "event": "PostToolUse",
      "matcher": ".*",
      "handler": {"type": "command", "command": "echo golden-test"}
    }
  ]
}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest := &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "golden-test",
		Hooks:    []ItemRef{{Name: "golden-hook"}},
	}

	cat := &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "golden-hook", Type: catalog.Hooks, Provider: "claude-code", Path: hookDir},
		},
	}

	prov := provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Hooks {
				return "__json_merge__"
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Hooks
		},
	}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Read the resulting settings.json and compare against golden file
	result, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	requireGolden(t, "settings-hook-merge", string(result))
}
