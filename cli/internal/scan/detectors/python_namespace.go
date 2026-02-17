package detectors

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// PythonNamespace detects directories that contain .py files but are
// missing __init__.py. This is a common source of confusion: Python 3
// supports "namespace packages" (directories without __init__.py), but
// most projects don't intend to use them. A missing __init__.py usually
// means broken imports or unexpected import resolution behavior.
//
// Excludes: tests/, scripts/, the project root itself, and hidden/venv dirs.
//
// How it works: walks the project tree, tracks which directories contain
// .py files, then checks each for __init__.py. Simple and fast.
//
// Gotchas: intentional namespace packages (used in some plugin architectures)
// will be flagged as false positives. This is acceptable because it's rare
// and worth calling out.
type PythonNamespace struct{}

func (d PythonNamespace) Name() string { return "python-namespace" }

// namespaceDirExclusions are directory names that should not be flagged
// for missing __init__.py.
var namespaceDirExclusions = map[string]bool{
	"tests":   true,
	"test":    true,
	"scripts": true,
	"script":  true,
	"bin":     true,
	"docs":    true,
	"doc":     true,
	"examples": true,
}

func (d PythonNamespace) Detect(root string) ([]model.Section, error) {
	if !isPythonProject(root) {
		return nil, nil
	}

	// Collect directories that contain .py files.
	dirsWithPy := make(map[string]bool)

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) == ".py" {
			dir := filepath.Dir(path)
			dirsWithPy[dir] = true
		}
		return nil
	})

	// Check each directory for missing __init__.py.
	var missing []string
	for dir := range dirsWithPy {
		// Skip the project root itself.
		if dir == root {
			continue
		}

		// Skip excluded directory names (tests/, scripts/, etc.)
		// Check both the immediate dir name and any ancestor within root.
		rel, _ := filepath.Rel(root, dir)
		if shouldExcludeNamespaceDir(rel) {
			continue
		}

		initPath := filepath.Join(dir, "__init__.py")
		if _, err := os.Stat(initPath); os.IsNotExist(err) {
			missing = append(missing, rel)
		}
	}

	if len(missing) == 0 {
		return nil, nil
	}

	// Sort for deterministic output.
	sortStrings(missing)

	body := fmt.Sprintf("Directories with .py files but no __init__.py (namespace package confusion risk):\n  %s\nIf these should be regular packages, add __init__.py. If intentional namespace packages, this can be ignored.",
		strings.Join(missing, "\n  "))

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Missing __init__.py",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// shouldExcludeNamespaceDir checks if a relative path starts with or is
// an excluded directory name.
func shouldExcludeNamespaceDir(rel string) bool {
	// Split the path and check the first component.
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if len(parts) == 0 {
		return false
	}
	return namespaceDirExclusions[parts[0]]
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
