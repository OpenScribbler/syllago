package loadout

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// TestApply_TryModeKeepsASettingsFileItCouldNotWrite: when the session-end
// hook cannot be written, the apply created no settings file, so remove
// must leave alone one that appears there later.
func TestApply_TryModeKeepsASettingsFileItCouldNotWrite(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Hooks = nil
	cat.Items = cat.Items[:1]

	claudeDir := filepath.Join(homeDir, ".claude")
	os.Chmod(claudeDir, 0555)
	t.Cleanup(func() { os.Chmod(claudeDir, 0755) })

	result, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "try", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.AutoRevertArmed {
		t.Fatal("the session-end hook was written into an unwritable directory")
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	sm, err := snapshot.ReadManifest(result.SnapshotDir)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(sm.CreatedFiles, settingsPath) {
		t.Errorf("CreatedFiles lists %s, which the apply never wrote", settingsPath)
	}

	os.Chmod(claudeDir, 0755)
	if err := os.WriteFile(settingsPath, []byte(`{"theme":"dark"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("remove deleted a settings file the apply never created: %v", err)
	}
}
