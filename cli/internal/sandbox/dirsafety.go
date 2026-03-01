package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sensitiveBlocklist contains paths that must never be used as the sandbox project root.
var sensitiveBlocklist = []string{
	"/",
	"/tmp",
	"/etc",
	"/var",
	"/opt",
}

// projectMarkers are files/dirs that indicate a valid project root.
var projectMarkers = []string{
	".git",
	".syllago",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"Makefile",
	"CMakeLists.txt",
	".project-root",
}

// DirSafetyError is returned when the CWD fails a safety check.
type DirSafetyError struct {
	Reason string
}

func (e *DirSafetyError) Error() string {
	return fmt.Sprintf("directory safety check failed: %s", e.Reason)
}

// ValidateDir checks whether dir is safe to use as a sandbox project root.
// Pass forceDir=true to skip validation (--force-dir flag).
func ValidateDir(dir string, forceDir bool) error {
	if forceDir {
		return nil
	}

	// Resolve symlinks first — prevents ~/code/proj → / bypass.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return &DirSafetyError{Reason: fmt.Sprintf("cannot resolve path: %s", err)}
	}
	dir = resolved

	// Get $HOME for relative checks.
	home, err := os.UserHomeDir()
	if err != nil {
		return &DirSafetyError{Reason: "cannot determine home directory"}
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return &DirSafetyError{Reason: "cannot resolve home directory"}
	}

	// Rule 1: block sensitive explicit paths (including $HOME itself).
	blocked := append(sensitiveBlocklist, home,
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".config"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".aws"),
	)
	for _, b := range blocked {
		if dir == b {
			return &DirSafetyError{Reason: fmt.Sprintf("path %q is explicitly blocked for sandbox use", dir)}
		}
	}

	// Rule 2: depth check — must be at least 2 levels below $HOME.
	rel, err := filepath.Rel(home, dir)
	if err != nil {
		return &DirSafetyError{Reason: "cannot compute path depth relative to home"}
	}
	// rel = "projects/syllago" → depth 2 (OK), "projects" → depth 1 (fail)
	if !strings.Contains(rel, string(filepath.Separator)) {
		return &DirSafetyError{Reason: fmt.Sprintf("directory must be at least 2 levels below $HOME (e.g. ~/projects/syllago). Got: %s", rel)}
	}

	// Rule 3: project marker.
	if !hasProjectMarker(dir) {
		return &DirSafetyError{Reason: "no project marker found (.git, go.mod, package.json, etc.). Use --force-dir to override"}
	}

	return nil
}

// hasProjectMarker returns true if any project marker exists in dir.
func hasProjectMarker(dir string) bool {
	for _, m := range projectMarkers {
		target := filepath.Join(dir, m)
		if m == ".git" {
			// Must be a directory, not a file (gitdir worktrees use a file).
			info, err := os.Stat(target)
			if err == nil && info.IsDir() {
				return true
			}
			continue
		}
		if _, err := os.Stat(target); err == nil {
			return true
		}
	}
	return false
}
