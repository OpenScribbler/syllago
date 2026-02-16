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

// CompetingFrameworks detects when a project has 2+ tools in the same category
// (testing, styling, ORM, HTTP, state management, bundler). This is a common
// source of confusion for new contributors who don't know which tool to use.
//
// Why this approach: we define category "buckets" with known package names,
// then check all project dependencies against them. An ordered slice of
// categories (not a map) ensures deterministic output ordering.
//
// Trade-offs: hardcoded package lists need maintenance as the ecosystem evolves,
// but they're simple and fast. A heuristic approach (e.g., analyzing imports)
// would catch more cases but be much slower and more fragile.
type CompetingFrameworks struct{}

func (d CompetingFrameworks) Name() string { return "competing-frameworks" }

func (d CompetingFrameworks) Detect(root string) ([]model.Section, error) {
	deps := collectAllDeps(root)
	if len(deps) == 0 {
		return nil, nil
	}

	// Each category entry: name -> list of known packages in that category.
	// Ordered slice for deterministic iteration.
	type frameworkCategory struct {
		name     string
		packages []string
	}

	categories := []frameworkCategory{
		{"testing", []string{
			"jest", "vitest", "mocha", "ava", "tape", "jasmine",
			"@testing-library/react", "enzyme", "cypress", "playwright",
			"testing", // Go: "testing" won't appear in deps, but testify/gocheck might
			"github.com/stretchr/testify", "github.com/onsi/ginkgo",
			"pytest", "unittest", "nose2",
		}},
		{"CSS/styling", []string{
			"tailwindcss", "styled-components", "@emotion/react", "@emotion/styled",
			"sass", "less", "styled-jsx", "@vanilla-extract/css", "linaria",
		}},
		{"ORM/database", []string{
			"prisma", "@prisma/client", "typeorm", "sequelize", "knex",
			"drizzle-orm", "mongoose", "mikro-orm",
			"github.com/go-gorm/gorm", "github.com/jmoiron/sqlx",
			"github.com/uptrace/bun",
			"sqlalchemy", "django-orm", "peewee", "tortoise-orm",
		}},
		{"HTTP client", []string{
			"axios", "got", "node-fetch", "ky", "superagent", "undici",
		}},
		{"state management", []string{
			"redux", "@reduxjs/toolkit", "zustand", "jotai", "recoil",
			"mobx", "valtio", "xstate", "@tanstack/react-query",
		}},
		{"bundler", []string{
			"webpack", "vite", "esbuild", "rollup", "parcel", "turbopack",
			"@swc/core", "tsup",
		}},
	}

	var sections []model.Section

	for _, cat := range categories {
		var found []string
		for _, pkg := range cat.packages {
			if _, ok := deps[pkg]; ok {
				found = append(found, pkg)
			}
		}
		if len(found) >= 2 {
			sort.Strings(found)
			sections = append(sections, model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    fmt.Sprintf("Competing %s frameworks", cat.name),
				Body:     fmt.Sprintf("Multiple %s tools found: %s. New contributors may not know which to use.", cat.name, strings.Join(found, ", ")),
				Source:   "competing-frameworks",
			})
		}
	}

	return sections, nil
}

// collectAllDeps reads dependency names from package.json, go.mod,
// pyproject.toml, and Cargo.toml. Returns a set (map[string]bool) of
// dependency names found across all manifest files. Version info is
// discarded since we only need presence checks.
//
// This is a package-level helper so other detectors can reuse it.
func collectAllDeps(root string) map[string]bool {
	deps := make(map[string]bool)

	collectNodeDeps(root, deps)
	collectGoDeps(root, deps)
	collectPythonDeps(root, deps)
	collectCargoDeps(root, deps)

	if len(deps) == 0 {
		return nil
	}
	return deps
}

func collectNodeDeps(root string, deps map[string]bool) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	for name := range pkg.Dependencies {
		deps[name] = true
	}
	for name := range pkg.DevDependencies {
		deps[name] = true
	}
}

func collectGoDeps(root string, deps map[string]bool) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return
	}

	inRequire := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				// Skip indirect deps — they're transitive and not meaningful signals.
				if len(parts) >= 3 && parts[2] == "indirect" {
					continue
				}
				deps[parts[0]] = true
			}
		}
	}
}

func collectPythonDeps(root string, deps map[string]bool) {
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		return
	}

	// Simple line-based parsing for pyproject.toml dependency arrays.
	// A full TOML parser would be more robust, but we're just looking
	// for package names in dependencies = [...] blocks.
	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "dependencies") && strings.Contains(trimmed, "[") {
			inDeps = true
			// Handle inline array: dependencies = ["foo", "bar"]
			if strings.Contains(trimmed, "]") {
				extractQuotedNames(trimmed, deps)
				inDeps = false
				continue
			}
			continue
		}
		if inDeps {
			if strings.Contains(trimmed, "]") {
				extractQuotedNames(trimmed, deps)
				inDeps = false
				continue
			}
			extractQuotedNames(trimmed, deps)
		}
	}
}

// extractQuotedNames pulls package names from quoted strings like "flask>=2.0".
// It strips version specifiers to get just the package name.
func extractQuotedNames(line string, deps map[string]bool) {
	for _, q := range []byte{'"', '\''} {
		parts := strings.Split(line, string(q))
		for i := 1; i < len(parts); i += 2 {
			name := parts[i]
			// Strip version specifiers: "flask>=2.0" -> "flask"
			for _, sep := range []string{">=", "<=", "!=", "==", ">", "<", "~=", "[", " "} {
				if idx := strings.Index(name, sep); idx > 0 {
					name = name[:idx]
				}
			}
			name = strings.TrimSpace(name)
			if name != "" {
				deps[name] = true
			}
		}
	}
}

func collectCargoDeps(root string, deps map[string]bool) {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return
	}

	inDeps := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[dependencies]" || trimmed == "[dev-dependencies]" {
			inDeps = true
			continue
		}
		// Any other section header ends the deps block.
		if strings.HasPrefix(trimmed, "[") {
			inDeps = false
			continue
		}
		if inDeps && strings.Contains(trimmed, "=") {
			name := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
			if name != "" {
				deps[name] = true
			}
		}
	}
}
