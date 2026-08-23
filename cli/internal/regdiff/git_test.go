package regdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGitDiff_FullDiff(t *testing.T) {
	t.Parallel()
	repoDir, commit1, commit2 := buildGitScenario(t)

	items := []ItemRef{
		{Type: "skills", Name: "alpha", Dir: "skills/alpha"},
		{Type: "skills", Name: "delta", Dir: "skills/delta"},
		{Type: "rules", Name: "gamma", Dir: "rules/gamma"},
	}
	got, err := GitDiff("example", repoDir, commit1, commit2, items, []string{"skills", "rules"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}

	want := Diff{
		Registry: "example",
		OldRef:   commit1,
		NewRef:   commit2,
		Changes: []ItemChange{
			{Type: "skills", Name: "alpha", Kind: KindModified, Paths: []string{"skills/alpha/SKILL.md"}},
			{Type: "skills", Name: "beta", Kind: KindRemoved, Paths: []string{"skills/beta/SKILL.md"}},
			{Type: "skills", Name: "delta", Kind: KindAdded, Paths: []string{"skills/delta/SKILL.md"}},
		},
		OtherPaths: []string{"README.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GitDiff() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGitDiff_UpToDate(t *testing.T) {
	t.Parallel()

	got, err := GitDiff("example", filepath.Join(t.TempDir(), "missing"), "abc123", "abc123", nil, nil)
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	want := Diff{
		Registry: "example",
		OldRef:   "abc123",
		NewRef:   "abc123",
		UpToDate: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GitDiff() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGitDiff_FirstSyncNoBaseline(t *testing.T) {
	t.Parallel()

	got, err := GitDiff("example", filepath.Join(t.TempDir(), "missing"), "", "new-head", nil, nil)
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	want := Diff{
		Registry: "example",
		OldRef:   "",
		NewRef:   "new-head",
		UpToDate: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GitDiff() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGitDiff_BadSHA(t *testing.T) {
	t.Parallel()
	repoDir, commit1, _ := buildGitScenario(t)

	_, err := GitDiff("example", repoDir, commit1, "not-a-sha", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "git diff") {
		t.Fatalf("error %q does not mention git diff", err.Error())
	}
}

func TestGitDiff_RenameUsesDeleteAndAdd(t *testing.T) {
	t.Parallel()
	repoDir, _, commit2 := buildGitScenario(t)

	if err := os.Rename(filepath.Join(repoDir, "skills", "delta"), filepath.Join(repoDir, "skills", "epsilon")); err != nil {
		t.Fatalf("rename delta to epsilon: %v", err)
	}
	commitAll(t, repoDir, "rename delta")
	commit3 := gitIn(t, repoDir, "rev-parse", "HEAD")

	items := []ItemRef{
		{Type: "skills", Name: "alpha", Dir: "skills/alpha"},
		{Type: "skills", Name: "epsilon", Dir: "skills/epsilon"},
		{Type: "rules", Name: "gamma", Dir: "rules/gamma"},
	}
	got, err := GitDiff("example", repoDir, commit2, commit3, items, []string{"skills", "rules"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}

	wantChanges := []ItemChange{
		{Type: "skills", Name: "delta", Kind: KindRemoved, Paths: []string{"skills/delta/SKILL.md"}},
		{Type: "skills", Name: "epsilon", Kind: KindAdded, Paths: []string{"skills/epsilon/SKILL.md"}},
	}
	if !reflect.DeepEqual(got.Changes, wantChanges) {
		t.Fatalf("Changes mismatch:\n got: %#v\nwant: %#v", got.Changes, wantChanges)
	}
	if len(got.OtherPaths) != 0 {
		t.Fatalf("OtherPaths = %#v; want empty", got.OtherPaths)
	}
}

func buildGitScenario(t *testing.T) (repoDir, commit1, commit2 string) {
	t.Helper()
	repoDir = t.TempDir()
	gitIn(t, repoDir, "init")
	gitIn(t, repoDir, "config", "user.email", "test@example.com")
	gitIn(t, repoDir, "config", "user.name", "Test User")

	writeRepoFile(t, repoDir, "skills/alpha/SKILL.md", "alpha v1\n")
	writeRepoFile(t, repoDir, "skills/beta/SKILL.md", "beta v1\n")
	writeRepoFile(t, repoDir, "rules/gamma/rule.md", "gamma v1\n")
	writeRepoFile(t, repoDir, "README.md", "# registry\n")
	commitAll(t, repoDir, "initial")
	commit1 = gitIn(t, repoDir, "rev-parse", "HEAD")

	writeRepoFile(t, repoDir, "skills/alpha/SKILL.md", "alpha v2\n")
	if err := os.RemoveAll(filepath.Join(repoDir, "skills", "beta")); err != nil {
		t.Fatalf("remove beta: %v", err)
	}
	writeRepoFile(t, repoDir, "skills/delta/SKILL.md", "delta v1\n")
	writeRepoFile(t, repoDir, "README.md", "# registry\n\nupdated\n")
	commitAll(t, repoDir, "second")
	commit2 = gitIn(t, repoDir, "rev-parse", "HEAD")

	return repoDir, commit1, commit2
}

func writeRepoFile(t *testing.T, repoDir, rel, contents string) {
	t.Helper()
	path := filepath.Join(repoDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func commitAll(t *testing.T, repoDir, message string) {
	t.Helper()
	gitIn(t, repoDir, "add", "-A")
	gitIn(t, repoDir, "commit", "-m", message)
}

func gitIn(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	allArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", allArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", allArgs, err, out)
	}
	return strings.TrimSpace(string(out))
}
