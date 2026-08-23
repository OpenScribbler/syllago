package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestRegistrySync_PersistsGitLastSyncBookkeeping(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	bare := createBareGitRegistryForSyncTest(t)
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "git-sync-bookkeeping", URL: bare},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) {
		return registry.VisibilityPublic, nil
	})

	before := time.Now().Add(-1 * time.Second)
	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-sync-bookkeeping"}); err != nil {
		t.Fatalf("registry sync: %v", err)
	}
	after := time.Now().Add(1 * time.Second)

	reloaded, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("config.LoadGlobal: %v", err)
	}
	if len(reloaded.Registries) != 1 {
		t.Fatalf("loaded %d registries, want 1", len(reloaded.Registries))
	}

	cloneDir, err := registry.CloneDir("git-sync-bookkeeping")
	if err != nil {
		t.Fatalf("registry.CloneDir: %v", err)
	}
	wantHead := gitOutputForSyncTest(t, cloneDir, "rev-parse", "HEAD")

	got := reloaded.Registries[0]
	if got.LastSyncedSHA != wantHead {
		t.Errorf("LastSyncedSHA = %q, want %q", got.LastSyncedSHA, wantHead)
	}
	if got.LastSyncedAt == nil {
		t.Fatal("LastSyncedAt = nil, want sync completion time")
	}
	if got.LastSyncedAt.Before(before) || got.LastSyncedAt.After(after) {
		t.Errorf("LastSyncedAt = %v, want between %v and %v", got.LastSyncedAt, before, after)
	}
}

func createBareGitRegistryForSyncTest(t *testing.T) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	gitRunForSyncTest(t, work, "init")
	gitRunForSyncTest(t, work, "config", "user.email", "test@example.com")
	gitRunForSyncTest(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "registry.yaml"), []byte("name: git-sync-bookkeeping\nversion: \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRunForSyncTest(t, work, "add", "-A")
	gitRunForSyncTest(t, work, "commit", "-m", "initial")

	bare := filepath.Join(t.TempDir(), "registry.git")
	gitRunForSyncTest(t, "", "clone", "--bare", work, bare)
	return bare
}

func gitRunForSyncTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = gitOutputForSyncTest(t, dir, args...)
}

func gitOutputForSyncTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}
