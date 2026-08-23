package registryops

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestGitSyncDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	cacheDir := t.TempDir()
	oldCacheDir := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = oldCacheDir })

	repoDir := filepath.Join(cacheDir, "test-reg")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	gitDiffTestGit(t, repoDir, "init")
	gitDiffTestGit(t, repoDir, "config", "user.email", "test@example.com")
	gitDiffTestGit(t, repoDir, "config", "user.name", "Test User")
	gitDiffTestGit(t, repoDir, "config", "commit.gpgsign", "false")

	gitDiffTestWriteFile(t, repoDir, "skills/alpha/SKILL.md", "# Alpha\n\nv1\n")
	gitDiffTestGit(t, repoDir, "add", "-A")
	gitDiffTestGit(t, repoDir, "commit", "-m", "initial")
	oldHead := gitDiffTestGit(t, repoDir, "rev-parse", "HEAD")

	gitDiffTestWriteFile(t, repoDir, "skills/alpha/SKILL.md", "# Alpha\n\nv2\n")
	gitDiffTestWriteFile(t, repoDir, "skills/beta/SKILL.md", "# Beta\n\nv1\n")
	gitDiffTestGit(t, repoDir, "add", "-A")
	gitDiffTestGit(t, repoDir, "commit", "-m", "update content")
	newHead := gitDiffTestGit(t, repoDir, "rev-parse", "HEAD")

	got := GitSyncDiff("test-reg", oldHead, newHead)
	if got == nil {
		t.Fatal("GitSyncDiff = nil; want diff")
	}
	wantChanges := []regdiff.ItemChange{
		{Type: "skills", Name: "alpha", Kind: regdiff.KindModified, Paths: []string{"skills/alpha/SKILL.md"}, LogLines: []string{"update content"}},
		{Type: "skills", Name: "beta", Kind: regdiff.KindAdded, Paths: []string{"skills/beta/SKILL.md"}, LogLines: []string{"update content"}},
	}
	if !reflect.DeepEqual(got.Changes, wantChanges) {
		t.Fatalf("Changes = %#v; want %#v", got.Changes, wantChanges)
	}
	if got.Registry != "test-reg" || got.OldRef != oldHead || got.NewRef != newHead {
		t.Fatalf("refs = (%q, %q, %q); want (%q, %q, %q)", got.Registry, got.OldRef, got.NewRef, "test-reg", oldHead, newHead)
	}
	if len(got.OtherPaths) != 0 {
		t.Fatalf("OtherPaths = %#v; want empty", got.OtherPaths)
	}
}

func TestGitSyncDiffMissingRegistry(t *testing.T) {
	cacheDir := t.TempDir()
	oldCacheDir := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = oldCacheDir })

	if got := GitSyncDiff("missing-reg", "old", "new"); got != nil {
		t.Fatalf("GitSyncDiff missing registry = %#v; want nil", got)
	}
}

func gitDiffTestWriteFile(t *testing.T, repoDir, rel, contents string) {
	t.Helper()
	path := filepath.Join(repoDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func gitDiffTestGit(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", cmdArgs, err, out)
	}
	return strings.TrimSpace(string(out))
}
