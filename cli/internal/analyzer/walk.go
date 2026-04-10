package analyzer

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// defaultExcludeDirs are directories always skipped during analysis walks.
var defaultExcludeDirs = []string{
	"node_modules", "vendor", ".git", "dist", "build",
	"__pycache__", ".venv", ".tox", ".mypy_cache",
}

// binaryExtensions are file extensions always skipped (binary content).
var binaryExtensions = map[string]bool{
	".exe": true, ".bin": true, ".so": true, ".dylib": true, ".dll": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".ico": true,
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".whl": true,
	".pyc": true, ".class": true,
}

// walkLimits are per-repo limits to prevent resource exhaustion.
const (
	walkMaxFiles = 50_000
	walkMaxDepth = 30
)

// testMaxFiles allows tests to override the file limit. Zero means use walkMaxFiles.
var testMaxFiles int

// WalkResult holds all file paths collected during a walk.
type WalkResult struct {
	Paths    []string // all file paths relative to root
	Warnings []string
}

// Walk collects all non-excluded file paths under root.
// extraExcludeDirs are appended to defaultExcludeDirs.
// root must already be filepath.EvalSymlinks-resolved.
func Walk(root string, extraExcludeDirs []string) WalkResult {
	excluded := buildExcludeSet(extraExcludeDirs)
	var result WalkResult

	maxFiles := walkMaxFiles
	if testMaxFiles > 0 {
		maxFiles = testMaxFiles
	}

	depth := func(path string) int {
		rel, _ := filepath.Rel(root, path)
		return strings.Count(rel, string(filepath.Separator))
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Warnings = append(result.Warnings, "walk error at "+path+": "+err.Error())
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if excluded[name] {
				return filepath.SkipDir
			}
			if depth(path) > walkMaxDepth {
				result.Warnings = append(result.Warnings, "max depth reached at "+path)
				return filepath.SkipDir
			}
			return nil
		}
		if binaryExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		if len(result.Paths) >= maxFiles {
			result.Warnings = append(result.Warnings, "file limit reached; truncating walk")
			return fs.SkipAll
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		result.Paths = append(result.Paths, rel)
		return nil
	})
	return result
}

func buildExcludeSet(extra []string) map[string]bool {
	set := make(map[string]bool, len(defaultExcludeDirs)+len(extra))
	for _, d := range defaultExcludeDirs {
		set[d] = true
	}
	for _, d := range extra {
		set[filepath.Base(d)] = true
	}
	return set
}
