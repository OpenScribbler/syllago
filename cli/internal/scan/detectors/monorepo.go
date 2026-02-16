package detectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// MonorepoStructure detects workspace/monorepo configuration. Monorepos use a
// single repository for multiple packages, managed by tools like npm/yarn/pnpm
// workspaces, Turborepo, Nx, or Lerna. Knowing the workspace tool and package
// layout is essential context for contributors.
//
// This is factual context (CatConventions) — it describes project structure,
// not something surprising.
type MonorepoStructure struct{}

func (d MonorepoStructure) Name() string { return "monorepo-structure" }

func (d MonorepoStructure) Detect(root string) ([]model.Section, error) {
	var tools []string
	var packages []string

	// Check pnpm workspaces
	if pnpmPkgs := parsePnpmWorkspace(root); len(pnpmPkgs) > 0 {
		tools = append(tools, "pnpm workspaces")
		packages = append(packages, pnpmPkgs...)
	}

	// Check npm/yarn workspaces in package.json
	if npmPkgs := parseNpmWorkspaces(root); len(npmPkgs) > 0 {
		if !containsTool(tools, "pnpm workspaces") {
			tools = append(tools, "npm/yarn workspaces")
		}
		// Only add packages we haven't seen (pnpm and npm can overlap)
		seen := make(map[string]bool)
		for _, p := range packages {
			seen[p] = true
		}
		for _, p := range npmPkgs {
			if !seen[p] {
				packages = append(packages, p)
			}
		}
	}

	// Check for orchestration tools
	for _, marker := range []struct {
		file string
		tool string
	}{
		{"turbo.json", "Turborepo"},
		{"nx.json", "Nx"},
		{"lerna.json", "Lerna"},
	} {
		if _, err := os.Stat(filepath.Join(root, marker.file)); err == nil {
			tools = append(tools, marker.tool)
		}
	}

	if len(tools) == 0 {
		return nil, nil
	}

	// Resolve glob patterns to actual workspace package directories
	resolvedPackages := resolveWorkspacePackages(root, packages)
	sort.Strings(resolvedPackages)

	var body string
	if len(resolvedPackages) > 0 {
		body = fmt.Sprintf("Monorepo managed by %s. Workspace packages: %s.",
			strings.Join(tools, ", "),
			strings.Join(resolvedPackages, ", "),
		)
	} else {
		body = fmt.Sprintf("Monorepo managed by %s.", strings.Join(tools, ", "))
	}

	return []model.Section{model.TextSection{
		Category: model.CatConventions,
		Origin:   model.OriginAuto,
		Title:    "Monorepo Structure",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// parsePnpmWorkspace reads pnpm-workspace.yaml and extracts package patterns.
// pnpm-workspace.yaml format:
//
//	packages:
//	  - 'packages/*'
//	  - 'apps/*'
func parsePnpmWorkspace(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		return nil
	}

	// Simple line-based parsing — avoids a YAML dependency for a straightforward format.
	var patterns []string
	inPackages := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "packages:" {
			inPackages = true
			continue
		}
		// A non-indented line (or another top-level key) ends the packages block
		if inPackages && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && !strings.HasPrefix(trimmed, "-") {
			break
		}
		if inPackages && strings.HasPrefix(trimmed, "-") {
			pattern := strings.TrimPrefix(trimmed, "-")
			pattern = strings.TrimSpace(pattern)
			pattern = strings.Trim(pattern, "'\"")
			if pattern != "" {
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}

// parseNpmWorkspaces reads package.json "workspaces" field.
// Supports both array form: "workspaces": ["packages/*"]
// and object form: "workspaces": { "packages": ["packages/*"] }
func parseNpmWorkspaces(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	// Try array form first
	var pkgArray struct {
		Workspaces []string `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkgArray); err == nil && len(pkgArray.Workspaces) > 0 {
		return pkgArray.Workspaces
	}

	// Try object form
	var pkgObj struct {
		Workspaces struct {
			Packages []string `json:"packages"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkgObj); err == nil && len(pkgObj.Workspaces.Packages) > 0 {
		return pkgObj.Workspaces.Packages
	}

	return nil
}

// resolveWorkspacePackages expands glob patterns like "packages/*" into actual
// directory names that contain a package.json. Returns names relative to root.
func resolveWorkspacePackages(root string, patterns []string) []string {
	var resolved []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			// Check if this directory has a package.json
			if _, err := os.Stat(filepath.Join(match, "package.json")); err != nil {
				continue
			}
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			if !seen[rel] {
				seen[rel] = true
				resolved = append(resolved, rel)
			}
		}
	}

	return resolved
}

func containsTool(tools []string, name string) bool {
	for _, t := range tools {
		if t == name {
			return true
		}
	}
	return false
}
