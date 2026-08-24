package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeAt materializes the registry clone's tree at sha in a temporary
// directory using `git worktree add --detach`. The returned cleanup removes
// the worktree (best-effort) and must always be called.
func WorktreeAt(name, sha string) (dir string, cleanup func(), err error) {
	if err := checkGit(); err != nil {
		return "", nil, err
	}
	cloneDir, err := CloneDir(name)
	if err != nil {
		return "", nil, err
	}
	if _, err := os.Stat(cloneDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, fmt.Errorf("registry clone %q does not exist at %s", name, cloneDir)
		}
		return "", nil, fmt.Errorf("checking registry clone %q at %s: %w", name, cloneDir, err)
	}

	tmp, err := os.MkdirTemp("", "syllago-rollback-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating rollback temp dir: %w", err)
	}
	worktreeDir := filepath.Join(tmp, "tree")
	cleanup = func() {
		_ = runGitWorktreeCommand(cloneDir, "worktree", "remove", "--force", worktreeDir)
		_ = runGitWorktreeCommand(cloneDir, "worktree", "prune")
		_ = os.RemoveAll(tmp)
	}

	out, err := runGitWorktreeCommandOutput(cloneDir,
		"worktree", "add", "--detach", "--quiet", worktreeDir, sha,
	)
	if err != nil {
		cleanup()
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return "", nil, fmt.Errorf("git worktree add failed for %q at %s: %s", sha, cloneDir, msg)
		}
		return "", nil, fmt.Errorf("git worktree add failed for %q at %s: %w", sha, cloneDir, err)
	}
	return worktreeDir, cleanup, nil
}

func runGitWorktreeCommand(cloneDir string, args ...string) error {
	_, err := runGitWorktreeCommandOutput(cloneDir, args...)
	return err
}

func runGitWorktreeCommandOutput(cloneDir string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", cloneDir, "-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	return cmd.CombinedOutput()
}
