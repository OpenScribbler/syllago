package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

// findProjectRoot walks up from cwd looking for common project markers.
// Declared as a var so tests can override it.
var findProjectRoot = findProjectRootImpl

func findProjectRootImpl() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml"}
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback to cwd with warning
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(output.ErrWriter, "Warning: no project markers found (go.mod, package.json, etc.). Using current directory: %s\n", cwd)
	return cwd, nil
}

// isInteractive reports whether stdin is connected to a terminal.
// Returns false when stdin is piped or redirected (e.g. CI, scripts),
// which lets commands auto-accept prompts instead of hanging.
var isInteractive = isInteractiveImpl

func isInteractiveImpl() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// findProviderBySlug returns a pointer to the matching provider, or nil.
func findProviderBySlug(slug string) *provider.Provider {
	for i := range provider.AllProviders {
		if provider.AllProviders[i].Slug == slug {
			return &provider.AllProviders[i]
		}
	}
	return nil
}
