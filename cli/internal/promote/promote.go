package promote

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/installer"
	"github.com/holdenhewett/nesco/cli/internal/metadata"
)

// Result holds the outcome of a promote operation.
type Result struct {
	Branch     string
	PRUrl      string // empty if gh not available
	CompareURL string // fallback browser URL
}

// Promote copies a local item to the shared directory, creates a git branch, commits, pushes, and opens a PR.
func Promote(repoRoot string, item catalog.ContentItem) (*Result, error) {
	// 1. Check clean tree
	dirty, err := isTreeDirty(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("checking git status: %w", err)
	}
	if dirty {
		return nil, fmt.Errorf("working tree has uncommitted changes. Commit or stash them first")
	}

	// 2. Validate
	if item.Meta == nil {
		return nil, fmt.Errorf("item has no .nesco.yaml metadata")
	}
	errs := metadata.Validate(item.Path, string(item.Type), repoRoot)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.String())
		}
		return nil, fmt.Errorf("validation failed:\n  %s", strings.Join(msgs, "\n  "))
	}

	// 3. Determine shared destination
	sharedDir := sharedPath(repoRoot, item)

	// 4. Detect default branch
	defaultBranch := detectDefaultBranch(repoRoot)

	// 5. Create branch
	branchName := fmt.Sprintf("nesco/promote/%s/%s", item.Type, item.Name)
	if err := gitRun(repoRoot, "checkout", "-b", branchName); err != nil {
		// Branch might already exist — append timestamp
		branchName = fmt.Sprintf("%s-%d", branchName, time.Now().Unix())
		if err := gitRun(repoRoot, "checkout", "-b", branchName); err != nil {
			return nil, fmt.Errorf("creating branch: %w", err)
		}
	}

	// On any error after branch creation, return to default branch
	cleanup := func() {
		gitRun(repoRoot, "checkout", defaultBranch)
	}

	// 6. Copy content (exclude LLM-PROMPT.md)
	if err := copyForPromote(item.Path, sharedDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("copying content: %w", err)
	}

	// 7. Update metadata on the shared copy
	now := time.Now()
	sharedMeta := *item.Meta
	sharedMeta.PromotedAt = &now
	sharedMeta.PRBranch = branchName
	if err := metadata.Save(sharedDir, &sharedMeta); err != nil {
		cleanup()
		return nil, fmt.Errorf("writing metadata: %w", err)
	}

	// 8. Git add + commit
	if err := gitRun(repoRoot, "add", sharedDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("staging files: %w", err)
	}
	commitMsg := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
	if err := gitRun(repoRoot, "commit", "-m", commitMsg); err != nil {
		cleanup()
		return nil, fmt.Errorf("committing: %w", err)
	}

	// 9. Push
	if err := gitRun(repoRoot, "push", "-u", "origin", branchName); err != nil {
		cleanup()
		return nil, fmt.Errorf("pushing: %w", err)
	}

	result := &Result{Branch: branchName}

	// 10. PR creation (adaptive)
	if ghPath, err := exec.LookPath("gh"); err == nil && ghPath != "" {
		prTitle := fmt.Sprintf("Add %s: %s", item.Type, item.Name)
		prBody := fmt.Sprintf("Promotes `%s` from my-tools/ to shared.\n\nType: %s\nSource: %s",
			item.Name, item.Type, item.Meta.Source)
		out, err := commandOutput(repoRoot, "gh", "pr", "create",
			"--title", prTitle,
			"--body", prBody,
			"--base", defaultBranch)
		if err == nil {
			result.PRUrl = strings.TrimSpace(out)
		}
	}

	// Construct compare URL as fallback
	result.CompareURL = buildCompareURL(repoRoot, branchName)

	// 11. Return to default branch
	gitRun(repoRoot, "checkout", defaultBranch)

	return result, nil
}

// sharedPath returns the destination path in the shared directory.
func sharedPath(repoRoot string, item catalog.ContentItem) string {
	if item.Type.IsUniversal() {
		return filepath.Join(repoRoot, string(item.Type), item.Name)
	}
	return filepath.Join(repoRoot, string(item.Type), item.Provider, item.Name)
}

// copyForPromote copies content from local to shared, excluding scaffold artifacts.
func copyForPromote(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Skip scaffold artifacts
		base := filepath.Base(relPath)
		if base == "LLM-PROMPT.md" {
			return nil
		}
		targetPath := filepath.Join(dst, relPath)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		return installer.CopyContent(path, targetPath)
	})
}

// isTreeDirty checks if the git working tree has uncommitted changes.
func isTreeDirty(repoRoot string) (bool, error) {
	out, err := commandOutput(repoRoot, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// detectDefaultBranch finds the default branch name from the remote.
func detectDefaultBranch(repoRoot string) string {
	out, err := commandOutput(repoRoot, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Returns something like "refs/remotes/origin/main"
		parts := strings.Split(strings.TrimSpace(out), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return "main" // fallback
}

// buildCompareURL constructs a GitHub compare URL from the remote origin.
func buildCompareURL(repoRoot, branch string) string {
	out, err := commandOutput(repoRoot, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(out)

	// Convert git@github.com:org/repo.git to https://github.com/org/repo
	var baseURL string
	if strings.HasPrefix(remote, "git@github.com:") {
		path := strings.TrimPrefix(remote, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		baseURL = "https://github.com/" + path
	} else if strings.HasPrefix(remote, "https://github.com/") {
		baseURL = strings.TrimSuffix(remote, ".git")
	} else {
		return ""
	}

	return baseURL + "/compare/" + branch + "?expand=1"
}

// gitRun executes a git command in the given directory.
func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// commandOutput executes a command and returns its stdout.
// Unlike gitRun, this takes the command name as a parameter (for git, gh, etc).
func commandOutput(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
