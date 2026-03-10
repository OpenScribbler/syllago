package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsInsideGitRepo returns true if dir is already inside a git repository.
func IsInsideGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// InitAndCommit runs git init, stages all files, and creates the initial commit
// in the given directory. Returns an error if git is not available or any step fails.
func InitAndCommit(dir, message string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required but was not found on PATH")
	}

	run := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("init"); err != nil {
		return err
	}
	if err := run("add", "."); err != nil {
		return err
	}
	if err := run("commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// Username returns the git user.name config value, falling back to $USER.
func Username() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}
	return os.Getenv("USER")
}
