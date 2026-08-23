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

	bare, work, branch := createBareGitRegistryForSyncTest(t)
	cfg := &config.Config{
		Providers: []string{"claude-code"},
		Registries: []config.Registry{
			{Name: "git-sync-bookkeeping", URL: bare},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)
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

	updateBareGitRegistryForSyncDiffTest(t, work, bare, branch)
	stdout.Reset()
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"git-sync-bookkeeping"}); err != nil {
		t.Fatalf("second registry sync: %v", err)
	}
	gotOut := stdout.String()
	syncedLine := "Synced: git-sync-bookkeeping\n"
	diffBlock := "Changes since last sync:\n" +
		"  - agents/old-agent\n" +
		"  ~ rules/updated-rule\n" +
		"  + skills/new-thing\n"
	if !strings.Contains(gotOut, syncedLine+diffBlock) {
		t.Fatalf("second sync output = %q; want Synced line followed by diff block %q", gotOut, diffBlock)
	}
}

func createBareGitRegistryForSyncTest(t *testing.T) (bare, work, branch string) {
	t.Helper()
	work = filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	gitRunForSyncTest(t, work, "init")
	gitRunForSyncTest(t, work, "config", "user.email", "test@example.com")
	gitRunForSyncTest(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "registry.yaml"), []byte("name: git-sync-bookkeeping\nversion: \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeRepoFileForSyncTest(t, work, "agents/old-agent/AGENT.md", "# Old Agent\n")
	writeRepoFileForSyncTest(t, work, "rules/claude-code/updated-rule.md", "# Updated Rule\n")
	gitRunForSyncTest(t, work, "add", "-A")
	gitRunForSyncTest(t, work, "commit", "-m", "initial")
	branch = gitOutputForSyncTest(t, work, "branch", "--show-current")

	bare = filepath.Join(t.TempDir(), "registry.git")
	gitRunForSyncTest(t, "", "clone", "--bare", work, bare)
	return bare, work, branch
}

func updateBareGitRegistryForSyncDiffTest(t *testing.T, work, bare, branch string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(work, "agents", "old-agent")); err != nil {
		t.Fatalf("remove old-agent: %v", err)
	}
	writeRepoFileForSyncTest(t, work, "rules/claude-code/updated-rule.md", "# Updated Rule\n\nv2\n")
	writeRepoFileForSyncTest(t, work, "skills/new-thing/SKILL.md", "# New Thing\n")
	gitRunForSyncTest(t, work, "add", "-A")
	gitRunForSyncTest(t, work, "commit", "-m", "update content")
	gitRunForSyncTest(t, work, "push", bare, "HEAD:refs/heads/"+branch)
}

func writeRepoFileForSyncTest(t *testing.T, repoDir, rel, contents string) {
	t.Helper()
	path := filepath.Join(repoDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", rel, err)
	}
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
