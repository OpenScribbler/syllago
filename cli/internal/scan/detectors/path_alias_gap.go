package detectors

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// PathAliasGap detects when a TypeScript project defines path aliases in
// tsconfig.json but most imports still use relative paths. This suggests
// the aliases were set up but never adopted — a common source of confusion
// for new contributors who don't know which style to use.
type PathAliasGap struct{}

func (d PathAliasGap) Name() string { return "path-alias-gap" }

func (d PathAliasGap) Detect(root string) ([]model.Section, error) {
	tsconfigPath := filepath.Join(root, "tsconfig.json")
	data, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return nil, nil // no tsconfig, not a TS project
	}

	// Extract path alias prefixes from tsconfig.json "compilerOptions.paths"
	paths := gjson.GetBytes(data, "compilerOptions.paths")
	if !paths.Exists() || len(paths.Map()) == 0 {
		return nil, nil // no aliases defined
	}

	// Collect alias prefixes (e.g. "@/" from "@/*")
	var aliasPrefixes []string
	paths.ForEach(func(key, _ gjson.Result) bool {
		prefix := strings.TrimSuffix(key.String(), "*")
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix != "" {
			aliasPrefixes = append(aliasPrefixes, prefix)
		}
		return true
	})

	if len(aliasPrefixes) == 0 {
		return nil, nil
	}

	// Walk .ts/.tsx files counting alias vs relative imports
	var aliasImports, relativeImports int

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != ".ts" && ext != ".tsx" {
			return nil
		}

		a, r := countImports(path, aliasPrefixes)
		aliasImports += a
		relativeImports += r
		return nil
	})

	total := aliasImports + relativeImports
	if total == 0 {
		return nil, nil
	}

	aliasPct := float64(aliasImports) / float64(total) * 100

	// Only flag if aliases are defined but used less than 30% of the time
	if aliasPct >= 30 {
		return nil, nil
	}

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Low Path Alias Adoption",
		Body: fmt.Sprintf(
			"tsconfig.json defines path aliases but only %.0f%% of imports (%d/%d) use them. "+
				"Consider enforcing aliases via linting or removing unused alias config.",
			aliasPct, aliasImports, total,
		),
		Source: d.Name(),
	}}, nil
}

// countImports counts alias and relative imports in a single file.
func countImports(path string, aliasPrefixes []string) (alias, relative int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		spec := extractImportSpec(line)
		if spec == "" {
			continue
		}

		if strings.HasPrefix(spec, ".") {
			relative++
		} else {
			for _, prefix := range aliasPrefixes {
				if strings.HasPrefix(spec, prefix) {
					alias++
					break
				}
			}
			// Non-relative, non-alias imports (e.g. "react") are ignored
		}
	}
	return
}

// extractImportSpec pulls the module specifier from an import or require line.
// Handles: import ... from "spec", import "spec", require("spec").
func extractImportSpec(line string) string {
	// import ... from "spec" or import "spec"
	if strings.HasPrefix(line, "import ") {
		if idx := strings.Index(line, "from "); idx != -1 {
			return unquote(strings.TrimSpace(line[idx+5:]))
		}
		// bare import like: import "./side-effect"
		rest := strings.TrimPrefix(line, "import ")
		return unquote(strings.TrimSpace(rest))
	}

	// require("spec")
	if idx := strings.Index(line, "require("); idx != -1 {
		rest := line[idx+8:]
		return unquote(strings.TrimSpace(rest))
	}

	return ""
}

// unquote strips surrounding quotes and trailing punctuation from a module spec.
func unquote(s string) string {
	s = strings.TrimRight(s, ";)")
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return ""
}
