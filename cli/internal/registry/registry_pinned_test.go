package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTestUpstreamRepo(t *testing.T) (string, string) {
	t.Helper()
	workDir := filepath.Join(t.TempDir(), "upstream_work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Helper to run git
	gitRun := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	gitRun("init", "-b", "main")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "Test User")

	// Write initial file and commit
	if err := os.WriteFile(filepath.Join(workDir, "registry.yaml"), []byte("name: test-reg\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRun("add", "registry.yaml")
	gitRun("commit", "-m", "initial commit")

	// Get initial SHA
	initialSHA := gitRun("rev-parse", "HEAD")

	// Create tag "v1.0.0" at initial commit
	gitRun("tag", "v1.0.0")

	// Create a branch "feature-branch"
	gitRun("checkout", "-b", "feature-branch")
	if err := os.WriteFile(filepath.Join(workDir, "feature.txt"), []byte("feature"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gitRun("add", "feature.txt")
	gitRun("commit", "-m", "feature commit")

	// Go back to main
	gitRun("checkout", "main")

	// Clone bare
	bareDir := filepath.Join(t.TempDir(), "bare.git")
	cmdClone := exec.Command("git", "clone", "--bare", workDir, bareDir)
	cmdClone.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmdClone.CombinedOutput(); err != nil {
		t.Fatalf("failed to create bare repo: %v\n%s", err, out)
	}

	return bareDir, initialSHA
}

func TestCloneAndSync_BranchRef(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, _ := createTestUpstreamRepo(t)

	// Clone with branch ref
	err := Clone(bare, "my-reg", "feature-branch")
	if err != nil {
		t.Fatalf("Clone with branch ref failed: %v", err)
	}

	dir, _ := CloneDir("my-reg")

	// Check if we are on feature-branch
	cmdSym := exec.Command("git", "-C", dir, "symbolic-ref", "--quiet", "HEAD")
	cmdSym.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmdSym.CombinedOutput()
	if err != nil {
		t.Fatalf("symbolic-ref HEAD failed: %v\n%s", err, out)
	}
	symRef := strings.TrimSpace(string(out))
	if !strings.HasSuffix(symRef, "refs/heads/feature-branch") {
		t.Errorf("expected symbolic-ref to end with refs/heads/feature-branch, got %s", symRef)
	}

	// Status on branch clone
	statusOutcome, err := Status("my-reg")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if statusOutcome.Pinned {
		t.Error("expected Status on branch clone to not be Pinned")
	}

	// Sync on branch clone
	outcome, err := Sync("my-reg")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if outcome.Pinned {
		t.Error("expected Sync on branch clone to not be Pinned")
	}
}

func TestCloneAndSync_TagRef(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, initialSHA := createTestUpstreamRepo(t)

	// Clone with tag ref
	err := Clone(bare, "my-reg", "v1.0.0")
	if err != nil {
		t.Fatalf("Clone with tag ref failed: %v", err)
	}

	dir, _ := CloneDir("my-reg")

	// symbolic-ref HEAD should fail (detached HEAD)
	cmdSym := exec.Command("git", "-C", dir, "symbolic-ref", "--quiet", "HEAD")
	cmdSym.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if err := cmdSym.Run(); err == nil {
		t.Error("expected symbolic-ref to fail on detached tag HEAD, but it succeeded")
	}

	// HEAD should be at initialSHA
	currHead, err := gitHead(dir)
	if err != nil {
		t.Fatalf("gitHead failed: %v", err)
	}
	if currHead != initialSHA {
		t.Errorf("expected HEAD to be %s, got %s", initialSHA, currHead)
	}

	// Status on tag clone
	statusOutcome, err := Status("my-reg")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !statusOutcome.Pinned {
		t.Error("expected Status on tag clone to be Pinned")
	}

	// Sync on detached clone
	outcome, err := Sync("my-reg")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if !outcome.Pinned {
		t.Error("expected Sync on tag clone to be Pinned")
	}
	if outcome.NewHead != initialSHA {
		t.Errorf("expected Sync on pinned clone to keep HEAD at %s, got %s", initialSHA, outcome.NewHead)
	}
}

func TestCloneAndSync_CommitRef(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, initialSHA := createTestUpstreamRepo(t)

	// Clone with commit SHA
	err := Clone(bare, "my-reg", initialSHA)
	if err != nil {
		t.Fatalf("Clone with commit SHA failed: %v", err)
	}

	dir, _ := CloneDir("my-reg")

	// symbolic-ref HEAD should fail
	cmdSym := exec.Command("git", "-C", dir, "symbolic-ref", "--quiet", "HEAD")
	cmdSym.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if err := cmdSym.Run(); err == nil {
		t.Error("expected symbolic-ref to fail on detached commit HEAD")
	}

	// HEAD should be at initialSHA
	currHead, err := gitHead(dir)
	if err != nil {
		t.Fatalf("gitHead failed: %v", err)
	}
	if currHead != initialSHA {
		t.Errorf("expected HEAD to be %s, got %s", initialSHA, currHead)
	}
}

func TestClone_BogusRef(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, _ := createTestUpstreamRepo(t)

	// Clone with bogus ref
	err := Clone(bare, "my-reg", "bogus-ref")
	if err == nil {
		t.Fatal("expected Clone with bogus ref to fail, but it succeeded")
	}

	if !strings.Contains(err.Error(), `git checkout "bogus-ref" failed`) {
		t.Errorf("expected error message to mention the bad ref, got: %v", err)
	}

	// Clone dir should be removed
	if IsCloned("my-reg") {
		t.Error("expected clone directory to be removed on checkout failure")
	}
}

func TestSync_DetachedRemoteHead(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, initialSHA := createTestUpstreamRepo(t)

	// Clone with tag ref
	err := Clone(bare, "my-reg", "v1.0.0")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Add a commit to bare repository
	workDir := filepath.Join(t.TempDir(), "upstream_work_more")
	cmdClone := exec.Command("git", "clone", bare, workDir)
	cmdClone.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmdClone.CombinedOutput(); err != nil {
		t.Fatalf("git clone bare failed: %v\n%s", err, out)
	}

	cmdConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	cmdConfig1.Dir = workDir
	cmdConfig1.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	cmdConfig1.Run()

	cmdConfig2 := exec.Command("git", "config", "user.name", "Test User")
	cmdConfig2.Dir = workDir
	cmdConfig2.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	cmdConfig2.Run()

	if err := os.WriteFile(filepath.Join(workDir, "newfile.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmdAdd := exec.Command("git", "add", "newfile.txt")
	cmdAdd.Dir = workDir
	cmdAdd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	cmdAdd.Run()

	cmdCommit := exec.Command("git", "commit", "-m", "new commit")
	cmdCommit.Dir = workDir
	cmdCommit.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	cmdCommit.Run()

	cmdPush := exec.Command("git", "push", "origin", "main")
	cmdPush.Dir = workDir
	cmdPush.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmdPush.CombinedOutput(); err != nil {
		t.Fatalf("git push failed: %v\n%s", err, out)
	}

	newUpstreamSHA := gitOutput(t, workDir, "rev-parse", "HEAD")

	// Status on detached clone
	statusOutcome, err := Status("my-reg")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !statusOutcome.Pinned {
		t.Error("expected Status to be pinned")
	}
	if statusOutcome.Head != initialSHA {
		t.Errorf("expected local status HEAD to be %s, got %s", initialSHA, statusOutcome.Head)
	}
	if statusOutcome.RemoteHead != newUpstreamSHA {
		t.Errorf("expected remote status HEAD to be %s, got %s", newUpstreamSHA, statusOutcome.RemoteHead)
	}

	// Sync on detached clone
	outcome, err := Sync("my-reg")
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !outcome.Pinned {
		t.Error("expected Sync outcome to have Pinned = true")
	}
	if outcome.NewHead != initialSHA {
		t.Errorf("expected HEAD to remain unmoved at %s, got %s", initialSHA, outcome.NewHead)
	}
	if outcome.RemoteHead != newUpstreamSHA {
		t.Errorf("expected RemoteHead in Sync outcome to be %s, got %s", newUpstreamSHA, outcome.RemoteHead)
	}
}

func TestFailureHint_RollbackWarning(t *testing.T) {
	requireGit(t)
	setupCacheOverride(t)
	bare, _ := createTestUpstreamRepo(t)

	err := Clone(bare, "my-reg", "")
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	dir, _ := CloneDir("my-reg")

	cmdRemote := exec.Command("git", "-C", dir, "remote", "set-url", "origin", "file:///nonexistent/repo.git")
	cmdRemote.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmdRemote.CombinedOutput(); err != nil {
		t.Fatalf("set-url failed: %v\n%s", err, out)
	}

	// Status should fail and error message must contain rollback-data warning
	_, errStatus := Status("my-reg")
	if errStatus == nil {
		t.Fatal("expected Status to fail due to invalid remote URL")
	}
	expectedWarning := "(deleting the clone also discards local history used for item rollback)"
	if !strings.Contains(errStatus.Error(), expectedWarning) {
		t.Errorf("expected Status error to contain warning %q, got: %v", expectedWarning, errStatus)
	}

	// Sync should fail and error message must contain rollback-data warning
	_, errSync := Sync("my-reg")
	if errSync == nil {
		t.Fatal("expected Sync to fail due to invalid remote URL")
	}
	if !strings.Contains(errSync.Error(), expectedWarning) {
		t.Errorf("expected Sync error to contain warning %q, got: %v", expectedWarning, errSync)
	}
}
