package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeAtMaterializesHistoricalTreeAndCleansUp(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	cacheDir := t.TempDir()
	origCache := CacheDirOverride
	CacheDirOverride = cacheDir
	t.Cleanup(func() { CacheDirOverride = origCache })

	const regName = "acme/tools"
	cloneDir, err := CloneDir(regName)
	if err != nil {
		t.Fatalf("CloneDir: %v", err)
	}
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("MkdirAll clone: %v", err)
	}
	gitRunForWorktreeTest(t, cloneDir, "init")
	gitRunForWorktreeTest(t, cloneDir, "config", "user.email", "test@example.com")
	gitRunForWorktreeTest(t, cloneDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(cloneDir, "item.txt"), []byte("old\n"), 0644); err != nil {
		t.Fatalf("WriteFile old: %v", err)
	}
	gitRunForWorktreeTest(t, cloneDir, "add", "-A")
	gitRunForWorktreeTest(t, cloneDir, "commit", "-m", "old")
	oldSHA := gitOutputForWorktreeTest(t, cloneDir, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(cloneDir, "item.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("WriteFile new: %v", err)
	}
	gitRunForWorktreeTest(t, cloneDir, "add", "-A")
	gitRunForWorktreeTest(t, cloneDir, "commit", "-m", "new")

	dir, cleanup, err := WorktreeAt(regName, oldSHA)
	if err != nil {
		t.Fatalf("WorktreeAt: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "item.txt"))
	if err != nil {
		t.Fatalf("ReadFile materialized item: %v", err)
	}
	if string(data) != "old\n" {
		t.Fatalf("materialized content = %q, want old", data)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still exists after cleanup, stat err = %v", err)
	}
}

func TestWorktreeAtBogusSHAErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	cacheDir := t.TempDir()
	origCache := CacheDirOverride
	CacheDirOverride = cacheDir
	t.Cleanup(func() { CacheDirOverride = origCache })

	const regName = "acme/bogus"
	cloneDir, err := CloneDir(regName)
	if err != nil {
		t.Fatalf("CloneDir: %v", err)
	}
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		t.Fatalf("MkdirAll clone: %v", err)
	}
	gitRunForWorktreeTest(t, cloneDir, "init")
	gitRunForWorktreeTest(t, cloneDir, "config", "user.email", "test@example.com")
	gitRunForWorktreeTest(t, cloneDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(cloneDir, "item.txt"), []byte("content\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRunForWorktreeTest(t, cloneDir, "add", "-A")
	gitRunForWorktreeTest(t, cloneDir, "commit", "-m", "initial")

	_, cleanup, err := WorktreeAt(regName, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("WorktreeAt bogus SHA returned nil error")
	}
	if !strings.Contains(err.Error(), "aaaaaaaaaaaa") {
		t.Fatalf("error = %v, want it to name the missing sha", err)
	}
}

func gitRunForWorktreeTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = gitOutputForWorktreeTest(t, dir, args...)
}

func gitOutputForWorktreeTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
