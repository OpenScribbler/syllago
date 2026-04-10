package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// writeSnapshot creates a snapshot manifest under projectRoot/.syllago/snapshots/<timestamp>/manifest.json.
func writeSnapshot(t *testing.T, projectRoot string, manifest *snapshot.SnapshotManifest) {
	t.Helper()
	ts := manifest.CreatedAt.UTC().Format("20060102T150405")
	dir := filepath.Join(projectRoot, ".syllago", "snapshots", ts)
	os.MkdirAll(dir, 0755)

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
}

func TestRunLoadoutStatus(t *testing.T) {
	fixedTime := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		setup      func(t *testing.T, root string)
		jsonOutput bool
		wantErr    bool
		check      func(t *testing.T, out string)
	}{
		{
			name:  "no snapshot shows no active loadout",
			setup: func(t *testing.T, root string) {},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "No active loadout") {
					t.Errorf("expected 'No active loadout', got: %s", out)
				}
			},
		},
		{
			name:       "no snapshot JSON output",
			setup:      func(t *testing.T, root string) {},
			jsonOutput: true,
			check: func(t *testing.T, out string) {
				t.Helper()
				var result loadoutStatusResult
				if err := json.Unmarshal([]byte(out), &result); err != nil {
					t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
				}
				if result.Active {
					t.Error("expected active=false")
				}
				if result.Name != "" {
					t.Errorf("expected empty name, got %q", result.Name)
				}
			},
		},
		{
			name: "active snapshot shows loadout info",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "my-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
					Symlinks: []snapshot.SymlinkRecord{
						{Path: "/home/user/.claude/rules/my-rule", Target: "/repo/rules/my-rule"},
					},
				})
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "my-loadout") {
					t.Error("expected loadout name")
				}
				if !strings.Contains(out, "keep") {
					t.Error("expected mode")
				}
				if !strings.Contains(out, "2026-03-25") {
					t.Error("expected date")
				}
				if !strings.Contains(out, "Installed symlinks") {
					t.Error("expected symlinks section")
				}
				if !strings.Contains(out, "my-rule") {
					t.Error("expected symlink path in output")
				}
			},
		},
		{
			name: "active snapshot JSON output",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "test-loadout",
					Mode:        "try",
					CreatedAt:   fixedTime,
				})
			},
			jsonOutput: true,
			check: func(t *testing.T, out string) {
				t.Helper()
				var result loadoutStatusResult
				if err := json.Unmarshal([]byte(out), &result); err != nil {
					t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
				}
				if !result.Active {
					t.Error("expected active=true")
				}
				if result.Name != "test-loadout" {
					t.Errorf("name = %q, want %q", result.Name, "test-loadout")
				}
				if result.Mode != "try" {
					t.Errorf("mode = %q, want %q", result.Mode, "try")
				}
				if result.AppliedAt != "2026-03-25 14:30:00" {
					t.Errorf("appliedAt = %q, want %q", result.AppliedAt, "2026-03-25 14:30:00")
				}
			},
		},
		{
			name: "active snapshot with hooks shows hooks section",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "hooks-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
					HookScripts: []string{"pre-commit.sh", "post-checkout.sh"},
				})
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "Installed hooks") {
					t.Error("expected hooks section")
				}
				if !strings.Contains(out, "pre-commit.sh") {
					t.Error("expected hook script name")
				}
				if !strings.Contains(out, "post-checkout.sh") {
					t.Error("expected second hook script")
				}
			},
		},
		{
			name: "corrupt snapshot returns error",
			setup: func(t *testing.T, root string) {
				// Create a snapshot dir with invalid manifest.json
				dir := filepath.Join(root, ".syllago", "snapshots", "20260325T143000")
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("not json"), 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel — mutates globals (findProjectRoot, output.JSON)
			root := t.TempDir()
			stdout, _ := output.SetForTest(t)
			withFakeRepoRoot(t, root)

			tt.setup(t, root)

			if tt.jsonOutput {
				output.JSON = true
			}

			err := loadoutStatusCmd.RunE(loadoutStatusCmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.check(t, stdout.String())
		})
	}
}
