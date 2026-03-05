package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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

// findItemByPath parses a path like "type/name" or "type/provider/name" and
// finds the matching item in the catalog.
func findItemByPath(cat *catalog.Catalog, path string) (*catalog.ContentItem, error) {
	parts := strings.Split(path, "/")

	switch len(parts) {
	case 2:
		// type/name
		typeName, itemName := parts[0], parts[1]
		for i := range cat.Items {
			if string(cat.Items[i].Type) == typeName && cat.Items[i].Name == itemName {
				return &cat.Items[i], nil
			}
		}

	case 3:
		// Try type/provider/name first (provider-specific content).
		typeName, providerOrType, name := parts[0], parts[1], parts[2]
		for i := range cat.Items {
			item := &cat.Items[i]
			if string(item.Type) == typeName && item.Provider == providerOrType && item.Name == name {
				return item, nil
			}
		}
		// Fallback: try registry/type/name (the first part is a registry name).
		for i := range cat.Items {
			item := &cat.Items[i]
			if item.Registry == typeName && string(item.Type) == providerOrType && item.Name == name {
				return item, nil
			}
		}

	default:
		return nil, fmt.Errorf("invalid path format: %q (expected type/name or type/provider/name)", path)
	}

	return nil, fmt.Errorf("item not found: %s", path)
}

// effectiveProvider returns the source provider for an item. For provider-specific
// types (rules, hooks, commands) this comes from the directory structure. For universal
// types (skills, agents, mcp) it comes from .syllago.yaml metadata.
func effectiveProvider(item catalog.ContentItem) string {
	if item.Provider != "" {
		return item.Provider
	}
	if item.Meta != nil && item.Meta.SourceProvider != "" {
		return item.Meta.SourceProvider
	}
	return ""
}

// exportWarnMessage returns a warning string if the item is example or built-in
// content. These items are provided by syllago and may conflict with provider defaults
// or aren't intended for direct use. Returns "" for normal items.
func exportWarnMessage(item catalog.ContentItem) string {
	if item.IsExample() {
		return "example content (for reference, not intended for direct use)"
	}
	if item.IsBuiltin() {
		return "built-in syllago content (may conflict with provider defaults)"
	}
	return ""
}

// filterBySource returns true if the item matches the given source filter.
// Valid source values: "library", "shared", "registry", "builtin", "all".
func filterBySource(item catalog.ContentItem, source string) bool {
	switch source {
	case "library":
		return item.Library
	case "shared":
		return !item.Library && item.Registry == "" && !item.IsBuiltin()
	case "registry":
		return item.Registry != ""
	case "builtin":
		return item.IsBuiltin()
	case "all":
		return true
	default:
		return item.Library
	}
}
