package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
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

	if err := InitAndCommit(tmp, "Initial commit"); err != nil {
		t.Fatalf("InitAndCommit: %v", err)
	}

	if !IsInsideGitRepo(tmp) {
		t.Error("should be inside a git repo after InitAndCommit")
	}
}
