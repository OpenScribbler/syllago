package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// addConflictProviders injects a pair of conflicting test providers into AllProviders.
// installerSlug's InstallDir for Skills = sharedDir.
// readerSlug's InstallDir for Skills = readerDir, GlobalSharedReadPaths = [sharedDir].
// Both are detected. Returns cleanup via t.Cleanup.
func addConflictProviders(t *testing.T, installerSlug, readerSlug, sharedDir, readerDir string) {
	t.Helper()
	orig := append([]provider.Provider(nil), provider.AllProviders...)

	mkProv := func(slug, name, installDir string, sharedPath string) provider.Provider {
		var globalShared func(string, catalog.ContentType) []string
		if sharedPath != "" {
			sp := sharedPath
			globalShared = func(_ string, ct catalog.ContentType) []string {
				if ct == catalog.Skills {
					return []string{sp}
				}
				return nil
			}
		}
		dir := installDir
		return provider.Provider{
			Slug:     slug,
			Name:     name,
			Detected: true,
			Detect:   func(string) bool { return true },
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return dir
				}
				return ""
			},
			GlobalSharedReadPaths: globalShared,
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills
			},
			SymlinkSupport: map[catalog.ContentType]bool{
				catalog.Skills: true,
			},
		}
	}

	provider.AllProviders = append(provider.AllProviders,
		mkProv(installerSlug, installerSlug, sharedDir, ""),
		mkProv(readerSlug, readerSlug, readerDir, sharedDir),
	)
	t.Cleanup(func() { provider.AllProviders = orig })
}

// withResolveConflict overrides the resolveConflict injectable for the duration of the test.
func withResolveConflict(t *testing.T, fn func([]installer.Conflict, io.Writer, io.Writer) (installer.ConflictResolution, error)) {
	t.Helper()
	orig := resolveConflict
	resolveConflict = fn
	t.Cleanup(func() { resolveConflict = orig })
}

// TestInstallToAll_ConflictWarning_NoInput: with --no-input, the command should
// print a conflict warning but proceed without prompting (using ResolutionAll).
func TestInstallToAll_ConflictWarning_NoInput(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withFakeRepoRoot(t, t.TempDir())

	sharedDir := t.TempDir()
	readerDir := t.TempDir()
	addConflictProviders(t, "ci-installer", "ci-reader", sharedDir, readerDir)

	stdout, stderr := output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")
	installCmd.Flags().Set("no-input", "true")
	defer installCmd.Flags().Set("no-input", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("--to-all --no-input with conflicts should not error: %v", err)
	}

	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "conflict") && !strings.Contains(combined, "shared") {
		t.Errorf("expected conflict warning in output\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
}

// TestInstallToAll_ConflictResolution_SharedOnly: when the user picks SharedOnly,
// only the installer provider's dir receives files; the reader's own dir stays empty.
func TestInstallToAll_ConflictResolution_SharedOnly(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withFakeRepoRoot(t, t.TempDir())

	sharedDir := t.TempDir()
	readerDir := t.TempDir()
	addConflictProviders(t, "so-installer", "so-reader", sharedDir, readerDir)

	withResolveConflict(t, func(_ []installer.Conflict, _, _ io.Writer) (installer.ConflictResolution, error) {
		return installer.ResolutionSharedOnly, nil
	})

	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	if err := installCmd.RunE(installCmd, []string{}); err != nil {
		t.Fatalf("SharedOnly resolution should not error: %v", err)
	}

	// Installer's shared dir should contain the skill (installed via Codex-like provider).
	entries, _ := os.ReadDir(sharedDir)
	if len(entries) == 0 {
		t.Error("SharedOnly: expected installer dir to have content, got none")
	}

	// Reader's own dir should be empty — it was removed from active list.
	entries, _ = os.ReadDir(readerDir)
	if len(entries) > 0 {
		t.Errorf("SharedOnly: expected reader dir to be empty (reader skipped), got %v",
			func() []string {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				return names
			}())
	}
}

// TestInstallToAll_ConflictResolution_OwnDirsOnly: when the user picks OwnDirsOnly,
// the reader installs to its own dir; the installer (shared path owner) is skipped.
func TestInstallToAll_ConflictResolution_OwnDirsOnly(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withFakeRepoRoot(t, t.TempDir())

	sharedDir := t.TempDir()
	readerDir := t.TempDir()
	addConflictProviders(t, "od-installer", "od-reader", sharedDir, readerDir)

	withResolveConflict(t, func(_ []installer.Conflict, _, _ io.Writer) (installer.ConflictResolution, error) {
		return installer.ResolutionOwnDirsOnly, nil
	})

	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	if err := installCmd.RunE(installCmd, []string{}); err != nil {
		t.Fatalf("OwnDirsOnly resolution should not error: %v", err)
	}

	// Shared dir should be empty — the installer was removed from active list.
	entries, _ := os.ReadDir(sharedDir)
	if len(entries) > 0 {
		t.Errorf("OwnDirsOnly: expected shared dir to be empty (installer skipped), got entries")
	}

	// Reader's own dir should have the skill.
	entries, _ = os.ReadDir(readerDir)
	if len(entries) == 0 {
		t.Error("OwnDirsOnly: expected reader dir to have content, got none")
	}
}

// TestInstallToAll_NoConflict_NoPrompt: when there are no conflicts, resolveConflict
// is never called and the install proceeds normally.
// We replace AllProviders entirely so real conflicting providers on the system
// don't pollute the test.
func TestInstallToAll_NoConflict_NoPrompt(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withFakeRepoRoot(t, t.TempDir())

	installBase := t.TempDir()

	orig := provider.AllProviders
	provider.AllProviders = []provider.Provider{
		{
			Slug:     "nc-prov",
			Name:     "No Conflict Prov",
			Detected: true,
			Detect:   func(string) bool { return true },
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
			SymlinkSupport: map[catalog.ContentType]bool{
				catalog.Skills: true,
			},
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	called := false
	withResolveConflict(t, func(_ []installer.Conflict, _, _ io.Writer) (installer.ConflictResolution, error) {
		called = true
		return installer.ResolutionAll, nil
	})

	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	if err := installCmd.RunE(installCmd, []string{}); err != nil {
		t.Fatalf("no-conflict install should not error: %v", err)
	}

	if called {
		t.Error("resolveConflict should not be called when there are no conflicts")
	}
}

// TestInstallToAll_ConflictResolution_All: when the user picks All, both dirs
// receive files (current behavior, duplicates accepted).
func TestInstallToAll_ConflictResolution_All(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withFakeRepoRoot(t, t.TempDir())

	sharedDir := t.TempDir()
	readerDir := t.TempDir()
	addConflictProviders(t, "all-installer", "all-reader", sharedDir, readerDir)

	withResolveConflict(t, func(_ []installer.Conflict, _, _ io.Writer) (installer.ConflictResolution, error) {
		return installer.ResolutionAll, nil
	})

	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	if err := installCmd.RunE(installCmd, []string{}); err != nil {
		t.Fatalf("All resolution should not error: %v", err)
	}

	// Both dirs should have files.
	sharedEntries, _ := os.ReadDir(sharedDir)
	readerEntries, _ := os.ReadDir(readerDir)

	if len(sharedEntries) == 0 {
		t.Error("All: expected shared dir to have content")
	}
	if len(readerEntries) == 0 {
		t.Error("All: expected reader dir to have content")
	}
}

// TestInstallConflict_FlagRegistered verifies --no-input is registered on installCmd.
func TestInstallConflict_FlagRegistered(t *testing.T) {
	if installCmd.Flags().Lookup("no-input") == nil {
		t.Error("expected --no-input flag to be registered on installCmd")
	}
}

// resolveConflictPath helper for SharedOnly/OwnDirsOnly: reads a temp dir
// for symlinks created under a content type subdir.
func skillsIn(dir string) []os.DirEntry {
	entries, _ := os.ReadDir(filepath.Join(dir))
	return entries
}
