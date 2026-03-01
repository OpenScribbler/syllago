package loadout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/snapshot"
)

func TestCheckStaleSnapshot_NoSnapshot(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	info, err := CheckStaleSnapshot(projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil stale info when no snapshot exists")
	}
}

func TestCheckStaleSnapshot_RecentTry(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a recent "try" snapshot (should NOT be stale)
	_, err := snapshot.Create(projectRoot, "test-loadout", "try", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	info, err := CheckStaleSnapshot(projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for recent try snapshot (not stale yet)")
	}
}

func TestCheckStaleSnapshot_OldTry(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a snapshot, then manually backdate its CreatedAt
	snapshotDir, err := snapshot.Create(projectRoot, "old-loadout", "try", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	// Read manifest, backdate it, write it back
	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var manifest snapshot.SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parsing manifest: %v", err)
	}

	manifest.CreatedAt = time.Now().Add(-48 * time.Hour) // 48 hours ago
	data, _ = json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	info, err := CheckStaleSnapshot(projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected stale info for 48-hour-old try snapshot")
	}
	if info.LoadoutName != "old-loadout" {
		t.Errorf("expected loadout name old-loadout, got %s", info.LoadoutName)
	}
}

func TestCheckStaleSnapshot_KeepMode(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a "keep" snapshot -- should never be considered stale
	snapshotDir, err := snapshot.Create(projectRoot, "keep-loadout", "keep", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	// Even if it's old, keep mode should not be stale
	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	data, _ := os.ReadFile(manifestPath)
	var manifest snapshot.SnapshotManifest
	json.Unmarshal(data, &manifest)
	manifest.CreatedAt = time.Now().Add(-48 * time.Hour)
	data, _ = json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	info, err := CheckStaleSnapshot(projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Error("expected nil for keep-mode snapshot (should never be stale)")
	}
}
