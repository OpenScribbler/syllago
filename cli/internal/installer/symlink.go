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

// IsWindowsMount returns true if the given path is on a WSL Windows mount (e.g., /mnt/c/).
// Symlinks don't work reliably on Windows mounts, so callers should use copy instead.
func IsWindowsMount(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	// WSL mounts Windows drives at /mnt/<letter>/ — minimum valid path is "/mnt/c/" (7 chars)
	return len(absPath) > 6 && strings.HasPrefix(absPath, "/mnt/") && absPath[6] == '/'
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

// IsSymlinkedToAny checks if path is a symlink pointing into any of the given roots.
func IsSymlinkedToAny(path string, roots []string) bool {
	for _, root := range roots {
		if IsSymlinkedTo(path, root) {
			return true
		}
	}
	return false
}
