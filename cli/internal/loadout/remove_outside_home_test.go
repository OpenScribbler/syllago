package loadout

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// TestRemove_RestoresAFileOutsideHome: a loadout applied in a project
// outside the home directory restores the project's config where it was
// and reports that path.
func TestRemove_RestoresAFileOutsideHome(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	home, _ := os.UserHomeDir()
	if rel, err := filepath.Rel(home, projectRoot); err == nil && filepath.IsLocal(rel) {
		t.Skip("temp dir is under the home directory")
	}
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	const original = `{"mcpServers":{}}`
	os.WriteFile(cfgPath, []byte(original), 0644)
	if _, err := snapshot.Create(projectRoot, "dev", "keep", []string{cfgPath}, nil, nil); err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !slices.Equal(result.RestoredFiles, []string{cfgPath}) {
		t.Errorf("RestoredFiles: got %q, want %q", result.RestoredFiles, cfgPath)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != original {
		t.Errorf("config: got %s, want %s", got, original)
	}
}

// TestRemove_DeletesAFileTheApplyCreated: removing a loadout deletes a
// config its apply created and reports it.
func TestRemove_DeletesAFileTheApplyCreated(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	if _, err := snapshot.Create(projectRoot, "dev", "keep", []string{cfgPath}, nil, nil); err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !slices.Equal(result.RemovedFiles, []string{cfgPath}) {
		t.Errorf("RemovedFiles: got %q, want %q", result.RemovedFiles, cfgPath)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("created config still there (stat err %v)", err)
	}
}
