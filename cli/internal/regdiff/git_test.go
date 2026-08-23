package regdiff

import (
	"fmt"
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
			{Type: "skills", Name: "alpha", Kind: KindModified, Paths: []string{"skills/alpha/SKILL.md"}, LogLines: []string{"second"}},
			{Type: "skills", Name: "beta", Kind: KindRemoved, Paths: []string{"skills/beta/SKILL.md"}, LogLines: []string{"second"}},
			{Type: "skills", Name: "delta", Kind: KindAdded, Paths: []string{"skills/delta/SKILL.md"}, LogLines: []string{"second"}},
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
		{Type: "skills", Name: "delta", Kind: KindRemoved, Paths: []string{"skills/delta/SKILL.md"}, LogLines: []string{"rename delta"}},
		{Type: "skills", Name: "epsilon", Kind: KindAdded, Paths: []string{"skills/epsilon/SKILL.md"}, LogLines: []string{"rename delta"}},
	}
	if !reflect.DeepEqual(got.Changes, wantChanges) {
		t.Fatalf("Changes mismatch:\n got: %#v\nwant: %#v", got.Changes, wantChanges)
	}
	if len(got.OtherPaths) != 0 {
		t.Fatalf("OtherPaths = %#v; want empty", got.OtherPaths)
	}
}

func TestGitDiff_LogExcerptCaptured(t *testing.T) {
	t.Parallel()
	repoDir := buildGitRepo(t)

	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v1\n")
	commitAll(t, repoDir, "initial")
	oldHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v2\n")
	commitAll(t, repoDir, "fix foo prompt wording")
	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v3\n")
	commitAll(t, repoDir, "add usage examples")
	newHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	got, err := GitDiff("example", repoDir, oldHead, newHead, []ItemRef{
		{Type: "skills", Name: "foo", Dir: "skills/foo"},
	}, []string{"skills"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	if len(got.Changes) != 1 {
		t.Fatalf("Changes len = %d; want 1: %#v", len(got.Changes), got.Changes)
	}
	want := []string{"add usage examples", "fix foo prompt wording"}
	if !reflect.DeepEqual(got.Changes[0].LogLines, want) {
		t.Fatalf("LogLines = %#v; want %#v", got.Changes[0].LogLines, want)
	}
}

func TestGitDiff_LogExcerptCapsAtThree(t *testing.T) {
	t.Parallel()
	repoDir := buildGitRepo(t)

	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v1\n")
	commitAll(t, repoDir, "initial")
	oldHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	for i := 1; i <= 4; i++ {
		subject := fmt.Sprintf("foo update %d", i)
		writeRepoFile(t, repoDir, "skills/foo/SKILL.md", subject+"\n")
		commitAll(t, repoDir, subject)
	}
	newHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	got, err := GitDiff("example", repoDir, oldHead, newHead, []ItemRef{
		{Type: "skills", Name: "foo", Dir: "skills/foo"},
	}, []string{"skills"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	want := []string{"foo update 4", "foo update 3", "foo update 2"}
	if !reflect.DeepEqual(got.Changes[0].LogLines, want) {
		t.Fatalf("LogLines = %#v; want %#v", got.Changes[0].LogLines, want)
	}
}

func TestGitDiff_LogExcerptExcludesMergeCommits(t *testing.T) {
	t.Parallel()
	repoDir := buildGitRepo(t)

	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v1\n")
	commitAll(t, repoDir, "initial")
	oldHead := gitIn(t, repoDir, "rev-parse", "HEAD")
	baseBranch := gitIn(t, repoDir, "branch", "--show-current")

	gitIn(t, repoDir, "checkout", "-b", "feature")
	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo feature\n")
	commitAll(t, repoDir, "feature updates foo")
	gitIn(t, repoDir, "checkout", baseBranch)
	writeRepoFile(t, repoDir, "README.md", "mainline\n")
	commitAll(t, repoDir, "mainline update")
	gitIn(t, repoDir, "merge", "--no-ff", "feature", "-m", "merge feature branch")
	newHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	got, err := GitDiff("example", repoDir, oldHead, newHead, []ItemRef{
		{Type: "skills", Name: "foo", Dir: "skills/foo"},
	}, []string{"skills"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	want := []string{"feature updates foo"}
	if !reflect.DeepEqual(got.Changes[0].LogLines, want) {
		t.Fatalf("LogLines = %#v; want %#v", got.Changes[0].LogLines, want)
	}
}

func TestGitDiff_RemovedItemGetsDeletingCommitSubject(t *testing.T) {
	t.Parallel()
	repoDir := buildGitRepo(t)

	writeRepoFile(t, repoDir, "skills/foo/SKILL.md", "foo v1\n")
	commitAll(t, repoDir, "initial")
	oldHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	if err := os.RemoveAll(filepath.Join(repoDir, "skills", "foo")); err != nil {
		t.Fatalf("remove foo: %v", err)
	}
	commitAll(t, repoDir, "remove foo skill")
	newHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	got, err := GitDiff("example", repoDir, oldHead, newHead, nil, []string{"skills"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	if len(got.Changes) != 1 {
		t.Fatalf("Changes len = %d; want 1: %#v", len(got.Changes), got.Changes)
	}
	if got.Changes[0].Kind != KindRemoved {
		t.Fatalf("Kind = %q; want %q", got.Changes[0].Kind, KindRemoved)
	}
	want := []string{"remove foo skill"}
	if !reflect.DeepEqual(got.Changes[0].LogLines, want) {
		t.Fatalf("LogLines = %#v; want %#v", got.Changes[0].LogLines, want)
	}
}

func TestGitDiff_LogExcerptSkippedAboveItemLimit(t *testing.T) {
	t.Parallel()
	repoDir := buildGitRepo(t)

	writeRepoFile(t, repoDir, "README.md", "initial\n")
	commitAll(t, repoDir, "initial")
	oldHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	var items []ItemRef
	for i := 1; i <= 21; i++ {
		name := fmt.Sprintf("item-%02d", i)
		dir := "skills/" + name
		writeRepoFile(t, repoDir, dir+"/SKILL.md", name+"\n")
		items = append(items, ItemRef{Type: "skills", Name: name, Dir: dir})
	}
	commitAll(t, repoDir, "add many skills")
	newHead := gitIn(t, repoDir, "rev-parse", "HEAD")

	got, err := GitDiff("example", repoDir, oldHead, newHead, items, []string{"skills"})
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}
	if len(got.Changes) != 21 {
		t.Fatalf("Changes len = %d; want 21", len(got.Changes))
	}
	for _, change := range got.Changes {
		if len(change.LogLines) != 0 {
			t.Fatalf("%s/%s LogLines = %#v; want nil/empty", change.Type, change.Name, change.LogLines)
		}
	}
}

func buildGitScenario(t *testing.T) (repoDir, commit1, commit2 string) {
	t.Helper()
	repoDir = buildGitRepo(t)

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

func buildGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	gitIn(t, repoDir, "init")
	gitIn(t, repoDir, "config", "user.email", "test@example.com")
	gitIn(t, repoDir, "config", "user.name", "Test User")
	return repoDir
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
