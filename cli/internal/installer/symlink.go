package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CreateSymlink creates a symlink from target to source.
// Creates parent directories as needed.
// If the target already exists (stale symlink, previous copy, etc.),
// it is removed and replaced with the new symlink.
func CreateSymlink(source, target string) error {
	// Ensure parent directory exists
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	// If target already exists, remove it so we can replace
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("replacing existing target: %w", err)
		}
	}

	return os.Symlink(source, target)
}

// IsSymlinkedTo checks if the given path is a symlink pointing into the repoRoot.
func IsSymlinkedTo(path, repoRoot string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	// Resolve to absolute if relative
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	// Clean both paths for comparison
	target = filepath.Clean(target)
	repoRoot = filepath.Clean(repoRoot)
	return strings.HasPrefix(target, repoRoot+string(filepath.Separator)) || target == repoRoot
}
