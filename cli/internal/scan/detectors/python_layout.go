package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// PythonLayout determines whether a Python project uses "src layout"
// (src/package/__init__.py) or "flat layout" (package/__init__.py at root).
//
// Why this matters: the two layouts have different import semantics during
// development. Src layout forces you to install the package (preventing
// accidental imports from the source tree), while flat layout lets you
// import directly. This is informational context that helps AI tools
// understand the project structure.
//
// How it works: checks for src/ directory containing a Python package
// (a dir with __init__.py). If not found, looks for a top-level directory
// (not a known non-package dir) containing __init__.py.
type PythonLayout struct{}

func (d PythonLayout) Name() string { return "python-layout" }

func (d PythonLayout) Detect(root string) ([]model.Section, error) {
	if !isPythonProject(root) {
		return nil, nil
	}

	// Check for src layout: src/<package>/__init__.py
	srcDir := filepath.Join(root, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		pkgs := findPythonPackagesIn(srcDir)
		if len(pkgs) > 0 {
			return []model.Section{model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Python Project Layout",
				Body:     fmt.Sprintf("Uses src layout (src/%s/). Imports require package installation — run `pip install -e .` for development.", strings.Join(pkgs, ", src/")),
				Source:   d.Name(),
			}}, nil
		}
	}

	// Check for flat layout: <package>/__init__.py at root
	pkgs := findFlatLayoutPackages(root)
	if len(pkgs) > 0 {
		return []model.Section{model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Python Project Layout",
			Body:     fmt.Sprintf("Uses flat layout (%s/ at project root). Package is importable directly from the source tree.", strings.Join(pkgs, ", ")),
			Source:   d.Name(),
		}}, nil
	}

	return nil, nil
}

// findPythonPackagesIn returns names of subdirectories within dir that
// contain an __init__.py file (i.e., are Python packages).
func findPythonPackagesIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var pkgs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		initPath := filepath.Join(dir, e.Name(), "__init__.py")
		if _, err := os.Stat(initPath); err == nil {
			pkgs = append(pkgs, e.Name())
		}
	}
	return pkgs
}

// knownNonPackageDirs are top-level directories that should not be
// considered Python packages even if they contain __init__.py.
var knownNonPackageDirs = map[string]bool{
	"tests":     true,
	"test":      true,
	"docs":      true,
	"doc":       true,
	"scripts":   true,
	"bin":       true,
	"examples":  true,
	"benchmarks": true,
	"src":       true,
}

// findFlatLayoutPackages returns names of top-level directories at root
// that look like Python packages (contain __init__.py) and aren't known
// non-package directories.
func findFlatLayoutPackages(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var pkgs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		if shouldSkipDir(name) {
			continue
		}
		if knownNonPackageDirs[name] {
			continue
		}
		initPath := filepath.Join(root, name, "__init__.py")
		if _, err := os.Stat(initPath); err == nil {
			pkgs = append(pkgs, name)
		}
	}
	return pkgs
}
