package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestCheckOrphanedMerges_NoOrphans(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Empty installed.json and no providers with settings
	inst := &Installed{}
	SaveInstalled(projectRoot, inst)

	orphans, err := CheckOrphanedMerges(projectRoot, nil)
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected no orphans, got %d", len(orphans))
	}
}

func TestCheckOrphanedMerges_DetectsOrphanedHook(t *testing.T) {
	// Not parallel — uses HOME override via provider config
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	// Create a provider config directory with hooks in settings.json
	configDir := filepath.Join(home, ".syllago-test-orphan-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	settingsJSON := `{
  "hooks": {
    "PreToolUse": [
      {"matcher":"Bash","hooks":[{"type":"command","command":"echo tracked"}]},
      {"matcher":"Edit","hooks":[{"type":"command","command":"echo orphan"}]}
    ]
  }
}`
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settingsJSON), 0644)

	// Only the first hook is tracked in installed.json
	trackedEntry := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo tracked"}]}`
	trackedHash := computeGroupHash([]byte(trackedEntry))

	inst := &Installed{
		Hooks: []InstalledHook{
			{Name: "tracked-hook", Event: "PreToolUse", GroupHash: trackedHash, Command: "echo tracked"},
		},
	}
	SaveInstalled(projectRoot, inst)

	prov := provider.Provider{
		Name:      "Test",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
		Detected:  true,
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}

	// Should find one orphan (the "echo orphan" entry)
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].Type != "hook" {
		t.Errorf("orphan type = %q, want hook", orphans[0].Type)
	}
	if orphans[0].Key != "PreToolUse" {
		t.Errorf("orphan key = %q, want PreToolUse", orphans[0].Key)
	}
	if orphans[0].Index != 1 {
		t.Errorf("orphan index = %d, want 1", orphans[0].Index)
	}
}

func TestCheckOrphanedMerges_DetectsOrphanedMCP(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	configDir := filepath.Join(home, ".syllago-test-orphan-mcp-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() { os.RemoveAll(configDir) })

	settingsJSON := `{
  "mcpServers": {
    "tracked-server": {"command": "node", "args": ["s.js"]},
    "orphan-server": {"command": "node", "args": ["evil.js"]}
  }
}`
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(settingsJSON), 0644)

	inst := &Installed{
		MCP: []InstalledMCP{
			{Name: "tracked", ServerKey: "tracked-server"},
		},
	}
	SaveInstalled(projectRoot, inst)

	prov := provider.Provider{
		Name:      "Test",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
		Detected:  true,
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].Type != "mcp" {
		t.Errorf("orphan type = %q, want mcp", orphans[0].Type)
	}
	if orphans[0].Key != "orphan-server" {
		t.Errorf("orphan key = %q, want orphan-server", orphans[0].Key)
	}
}

func TestCheckOrphanedMerges_SkipsUndetectedProviders(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	inst := &Installed{}
	SaveInstalled(projectRoot, inst)

	// Provider not detected — should be skipped entirely
	prov := provider.Provider{
		Name:      "Undetected",
		Slug:      "undetected",
		ConfigDir: ".undetected",
		Detected:  false,
	}

	orphans, err := CheckOrphanedMerges(projectRoot, []provider.Provider{prov})
	if err != nil {
		t.Fatalf("CheckOrphanedMerges: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected no orphans for undetected provider, got %d", len(orphans))
	}
}
