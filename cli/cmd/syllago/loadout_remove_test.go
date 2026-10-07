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

// withNonInteractiveLoadout forces isInteractive to return false for the duration
// of the test, so runLoadoutRemove falls through past the confirmation prompt.
func withNonInteractiveLoadout(t *testing.T) {
	t.Helper()
	orig := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = orig })
}

func TestRunLoadoutRemove(t *testing.T) {
	fixedTime := time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC)
	var outsideCfg string // set by the cases that back up a file outside HOME

	tests := []struct {
		name       string
		auto       bool
		jsonOutput bool
		setup      func(t *testing.T, root string)
		wantErr    bool
		check      func(t *testing.T, stdout, stderr string)
	}{
		{
			name:  "auto mode with no snapshot returns nil silently",
			auto:  true,
			setup: func(t *testing.T, root string) {},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if strings.Contains(stdout, "No active loadout") {
					t.Errorf("auto mode should not print, got: %s", stdout)
				}
			},
		},
		{
			name:  "non-auto non-interactive with no snapshot prints message",
			setup: func(t *testing.T, root string) {},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "No active loadout to remove") {
					t.Errorf("expected 'No active loadout to remove' message, got: %s", stdout)
				}
			},
		},
		{
			name: "auto mode happy path removes snapshot silently",
			auto: true,
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "my-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
				})
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if stdout != "" {
					t.Errorf("auto mode should produce no stdout, got: %q", stdout)
				}
			},
		},
		{
			name: "non-auto non-interactive happy path prints display and success",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "my-loadout",
					Mode:        "try",
					CreatedAt:   fixedTime,
					Symlinks: []snapshot.SymlinkRecord{
						{Path: filepath.Join(root, "already-gone-symlink"), Target: filepath.Join(root, "src")},
					},
				})
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "Active loadout: my-loadout (try)") {
					t.Errorf("expected active-loadout display, got: %s", stdout)
				}
				if !strings.Contains(stdout, "Symlinks to remove:") {
					t.Errorf("expected symlinks section, got: %s", stdout)
				}
				if !strings.Contains(stdout, "already-gone-symlink") {
					t.Errorf("expected symlink path in output, got: %s", stdout)
				}
				if !strings.Contains(stdout, `Loadout "my-loadout" removed`) {
					t.Errorf("expected success message, got: %s", stdout)
				}
			},
		},
		{
			name: "copy-mode apply lists copies to delete rather than symlinks",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "my-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
					Symlinks: []snapshot.SymlinkRecord{
						{Path: filepath.Join(root, "already-gone-copy"), Target: filepath.Join(root, "src"), Copied: true},
					},
				})
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "Copies to delete, with any files added to them:") {
					t.Errorf("expected copies section, got: %s", stdout)
				}
				if strings.Contains(stdout, "Symlinks to remove:") {
					t.Errorf("copies should not be listed as symlinks, got: %s", stdout)
				}
			},
		},
		{
			name: "symlink apply with a hook lists its symlinks and its copy apart",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "my-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
					Symlinks: []snapshot.SymlinkRecord{
						{Path: filepath.Join(root, "already-gone-symlink"), Target: filepath.Join(root, "src")},
						{Path: filepath.Join(root, "already-gone-hook-scripts"), Target: filepath.Join(root, "hook"), Copied: true},
					},
				})
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				links, copies := strings.Index(stdout, "Symlinks to remove:"), strings.Index(stdout, "Copies to delete")
				link, hook := strings.Index(stdout, "already-gone-symlink"), strings.Index(stdout, "already-gone-hook-scripts")
				if links < 0 || copies < 0 || !(links < link && link < copies && copies < hook) {
					t.Errorf("want the symlink under symlinks and the hook scripts under copies, got: %s", stdout)
				}
			},
		},
		{
			name: "non-auto non-interactive JSON happy path prints result as JSON",
			setup: func(t *testing.T, root string) {
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName: "json-loadout",
					Mode:        "keep",
					CreatedAt:   fixedTime,
				})
			},
			jsonOutput: true,
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				// The pre-JSON display block goes to output.Writer too, so JSON is
				// at the tail. Peel it off: find the first '{' and parse from there.
				idx := strings.Index(stdout, "{")
				if idx < 0 {
					t.Fatalf("expected JSON payload in stdout, got: %s", stdout)
				}
				var result struct {
					LoadoutName string `json:"LoadoutName"`
				}
				if err := json.Unmarshal([]byte(stdout[idx:]), &result); err != nil {
					t.Fatalf("failed to parse JSON: %v\nstdout: %s", err, stdout)
				}
				if result.LoadoutName != "json-loadout" {
					t.Errorf("LoadoutName = %q, want %q", result.LoadoutName, "json-loadout")
				}
			},
		},
		{
			name: "non-auto displays backed-up files section when present",
			setup: func(t *testing.T, root string) {
				// Put a fake backup file in the snapshot so Restore succeeds.
				ts := fixedTime.UTC().Format("20060102T150405")
				snapDir := filepath.Join(root, ".syllago", "snapshots", ts)
				filesDir := filepath.Join(snapDir, "files", ".claude")
				if err := os.MkdirAll(filesDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(filesDir, "settings.json"), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
				manifest := &snapshot.SnapshotManifest{
					LoadoutName:   "backups-loadout",
					Mode:          "keep",
					CreatedAt:     fixedTime,
					BackedUpFiles: []string{".claude/settings.json"},
				}
				data, _ := json.Marshal(manifest)
				if err := os.WriteFile(filepath.Join(snapDir, "manifest.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "Files to restore from snapshot:") {
					t.Errorf("expected files-to-restore section, got: %s", stdout)
				}
				if !strings.Contains(stdout, ".claude/settings.json") {
					t.Errorf("expected backed-up file in output, got: %s", stdout)
				}
			},
		},
		{
			name: "non-auto lists a file outside the home directory where it restores",
			setup: func(t *testing.T, root string) {
				outsideCfg = filepath.Join(root, ".cursor", "mcp.json")
				os.MkdirAll(filepath.Dir(outsideCfg), 0755)
				os.WriteFile(outsideCfg, []byte("{}"), 0644)
				if _, err := snapshot.Create(root, "dev", "keep", []string{outsideCfg}, nil, nil); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "  "+outsideCfg+"\n") {
					t.Errorf("expected %s in the files to restore, got: %s", outsideCfg, stdout)
				}
			},
		},
		{
			name: "non-auto leaves out an installed.json an earlier version backed up",
			setup: func(t *testing.T, root string) {
				installedJSON := filepath.Join(root, ".syllago", "installed.json")
				os.MkdirAll(filepath.Dir(installedJSON), 0755)
				os.WriteFile(installedJSON, []byte("{}"), 0644)
				if _, err := snapshot.Create(root, "dev", "keep", []string{installedJSON}, nil, nil); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if strings.Contains(stdout, "installed.json") {
					t.Errorf("installed.json listed, but remove does not restore it: %s", stdout)
				}
			},
		},
		{
			name: "non-auto lists a file the loadout created for deletion",
			setup: func(t *testing.T, root string) {
				outsideCfg = filepath.Join(root, ".cursor", "mcp.json")
				if _, err := snapshot.Create(root, "dev", "keep", []string{outsideCfg}, nil, nil); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, stdout, stderr string) {
				t.Helper()
				if !strings.Contains(stdout, "Files the loadout created, to delete:\n  "+outsideCfg+"\n") {
					t.Errorf("expected %s listed for deletion, got: %s", outsideCfg, stdout)
				}
			},
		},
		{
			name: "corrupt snapshot returns structured error",
			setup: func(t *testing.T, root string) {
				ts := "20260325T143000"
				dir := filepath.Join(root, ".syllago", "snapshots", ts)
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("not json"), 0644)
			},
			wantErr: true,
		},
		{
			name: "loadout.Remove error surfaces as structured error",
			auto: true,
			setup: func(t *testing.T, root string) {
				// Manifest claims a backed-up file but the source is missing from
				// snapshot/files/, so snapshot.Restore fails when copyFile runs.
				writeSnapshot(t, root, &snapshot.SnapshotManifest{
					LoadoutName:   "broken-loadout",
					Mode:          "keep",
					CreatedAt:     fixedTime,
					BackedUpFiles: []string{".claude/nonexistent.json"},
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel — mutates globals (findProjectRoot, output.JSON, isInteractive)
			root := t.TempDir()
			// loadout.Remove calls os.UserHomeDir; redirect so BackedUpFiles restore
			// does not touch the real home.
			t.Setenv("HOME", t.TempDir())

			stdout, stderr := output.SetForTest(t)
			withFakeRepoRoot(t, root)
			withNonInteractiveLoadout(t)

			tt.setup(t, root)

			if tt.jsonOutput {
				output.JSON = true
			}

			loadoutRemoveCmd.Flags().Set("auto", "false")
			if tt.auto {
				loadoutRemoveCmd.Flags().Set("auto", "true")
			}
			t.Cleanup(func() { loadoutRemoveCmd.Flags().Set("auto", "false") })

			err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, stdout.String(), stderr.String())
			}
		})
	}
}

// withFakeStdin replaces os.Stdin with a pipe whose read end yields `input`.
// Cleanup restores the original stdin on test end.
func withFakeStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		r.Close()
	})
}

func TestRunLoadoutRemove_InteractiveConfirmsRemoval(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	stdout, _ := output.SetForTest(t)
	withFakeRepoRoot(t, root)

	// Force interactive branch.
	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	withFakeStdin(t, "y\n")

	writeSnapshot(t, root, &snapshot.SnapshotManifest{
		LoadoutName: "confirmed-loadout",
		Mode:        "keep",
		CreatedAt:   time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC),
	})

	loadoutRemoveCmd.Flags().Set("auto", "false")
	t.Cleanup(func() { loadoutRemoveCmd.Flags().Set("auto", "false") })

	err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Remove loadout?") {
		t.Errorf("expected interactive prompt, got: %s", out)
	}
	if !strings.Contains(out, `Loadout "confirmed-loadout" removed`) {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestRunLoadoutRemove_InteractiveCancelsOnNo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	stdout, _ := output.SetForTest(t)
	withFakeRepoRoot(t, root)

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	withFakeStdin(t, "n\n")

	writeSnapshot(t, root, &snapshot.SnapshotManifest{
		LoadoutName: "cancelled-loadout",
		Mode:        "keep",
		CreatedAt:   time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC),
	})

	loadoutRemoveCmd.Flags().Set("auto", "false")
	t.Cleanup(func() { loadoutRemoveCmd.Flags().Set("auto", "false") })

	err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Cancelled.") {
		t.Errorf("expected 'Cancelled.' message, got: %s", out)
	}
	// Snapshot should still be on disk since we cancelled.
	snapRoot := filepath.Join(root, ".syllago", "snapshots")
	entries, err := os.ReadDir(snapRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Error("snapshot should remain after cancellation")
	}
}

func TestRunLoadoutRemove_InteractiveEOFReturnsError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	stdout, _ := output.SetForTest(t)
	_ = stdout
	withFakeRepoRoot(t, root)

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	// Empty input → Scanner.Scan() returns false immediately.
	withFakeStdin(t, "")

	writeSnapshot(t, root, &snapshot.SnapshotManifest{
		LoadoutName: "eof-loadout",
		Mode:        "keep",
		CreatedAt:   time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC),
	})

	loadoutRemoveCmd.Flags().Set("auto", "false")
	t.Cleanup(func() { loadoutRemoveCmd.Flags().Set("auto", "false") })

	err := loadoutRemoveCmd.RunE(loadoutRemoveCmd, []string{})
	if err == nil {
		t.Fatal("expected error when stdin returns no input, got nil")
	}
}
