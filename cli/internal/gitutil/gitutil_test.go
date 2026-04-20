package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsInsideGitRepo_False(t *testing.T) {
	tmp := t.TempDir()
	if IsInsideGitRepo(tmp) {
		t.Error("fresh temp dir should not be inside a git repo")
	}
}

func TestIsInsideGitRepo_True(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	cmd := exec.Command("git", "init", tmp)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if !IsInsideGitRepo(tmp) {
		t.Error("should be inside a git repo after git init")
	}
}

func TestInitAndCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# test"), 0644)

	// Set git identity via env vars for CI environments where no global config exists.
	// t.Setenv is inherited by child processes and restored after the test.
	t.Setenv("GIT_AUTHOR_NAME", "Test User")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test User")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	if err := InitAndCommit(tmp, "Initial commit"); err != nil {
		t.Fatalf("InitAndCommit: %v", err)
	}

	if !IsInsideGitRepo(tmp) {
		t.Error("should be inside a git repo after InitAndCommit")
	}
}

func TestInitAndCommit_NoGit(t *testing.T) {
	// Override PATH so git cannot be found.
	t.Setenv("PATH", t.TempDir())

	err := InitAndCommit(t.TempDir(), "should fail")
	if err == nil {
		t.Fatal("expected error when git is not on PATH")
	}
	if want := "git is required"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q; want it to contain %q", err, want)
	}
}

func TestInitAndCommit_EmptyDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// An empty directory causes "git commit" to fail because there's nothing to commit.
	t.Setenv("GIT_AUTHOR_NAME", "Test User")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test User")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	err := InitAndCommit(t.TempDir(), "empty commit")
	if err == nil {
		t.Fatal("expected error when committing empty directory")
	}
}

func TestInitAndCommit_InvalidDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	err := InitAndCommit("/nonexistent/path/for/git/test", "should fail")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestUsername_FromGitConfig(t *testing.T) {
	name := Username()
	// Username returns either the git config value or $USER; it should never be empty
	// in a test environment.
	if name == "" {
		t.Error("Username() returned empty string")
	}
}

func TestUsername_FallbackToEnv(t *testing.T) {
	// Override PATH so git cannot be found, forcing the $USER fallback.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("USER", "testfallback")

	name := Username()
	if name != "testfallback" {
		t.Errorf("Username() = %q; want %q", name, "testfallback")
	}
}
