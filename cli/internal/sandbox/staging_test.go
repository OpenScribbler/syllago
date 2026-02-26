package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStagingDir_CreatesDir(t *testing.T) {
	s, err := NewStagingDir()
	if err != nil {
		t.Fatalf("NewStagingDir: %v", err)
	}
	defer s.Cleanup()

	if _, err := os.Stat(s.Path); err != nil {
		t.Errorf("staging dir should exist: %v", err)
	}
}

func TestNewStagingDir_UniqueIDs(t *testing.T) {
	s1, err := NewStagingDir()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer s1.Cleanup()

	s2, err := NewStagingDir()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	defer s2.Cleanup()

	if s1.ID == s2.ID {
		t.Error("expected unique IDs, got identical")
	}
}

func TestStagingDir_SocketPath(t *testing.T) {
	s, err := NewStagingDir()
	if err != nil {
		t.Fatalf("NewStagingDir: %v", err)
	}
	defer s.Cleanup()

	sp := s.SocketPath()
	if !strings.HasSuffix(sp, "proxy.sock") {
		t.Errorf("expected SocketPath ending in proxy.sock, got %s", sp)
	}
}

func TestStagingDir_WriteGitconfig(t *testing.T) {
	s, err := NewStagingDir()
	if err != nil {
		t.Fatalf("NewStagingDir: %v", err)
	}
	defer s.Cleanup()

	if err := s.WriteGitconfig("Test User", "test@example.com"); err != nil {
		t.Fatalf("WriteGitconfig: %v", err)
	}

	data, err := os.ReadFile(s.GitconfigPath())
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}
	if !strings.Contains(string(data), "[user]") {
		t.Error("expected [user] section in gitconfig")
	}
	if !strings.Contains(string(data), "Test User") {
		t.Error("expected user name in gitconfig")
	}
}

func TestStagingDir_Cleanup(t *testing.T) {
	s, err := NewStagingDir()
	if err != nil {
		t.Fatalf("NewStagingDir: %v", err)
	}

	path := s.Path
	if err := s.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected staging dir to be removed after Cleanup")
	}
}

func TestCleanStale_RemovesOldDirs(t *testing.T) {
	// Create a fake stale staging dir in /tmp.
	stale := filepath.Join("/tmp", "nesco-sandbox-staletest123")
	if err := os.MkdirAll(stale, 0700); err != nil {
		t.Fatalf("creating stale dir: %v", err)
	}

	CleanStale()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		// Clean up manually if test fails.
		os.RemoveAll(stale)
		t.Error("expected stale dir to be removed by CleanStale")
	}
}
