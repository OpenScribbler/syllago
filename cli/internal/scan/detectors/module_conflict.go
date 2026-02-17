package detectors

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// ModuleConflict detects ESM/CJS mixing in JavaScript projects by walking
// source directories for require() calls vs import/export statements.
//
// Why this matters: mixing module systems causes subtle bugs — default
// exports behave differently, tree-shaking breaks, and bundler configs
// get complicated. New contributors often don't realize the project has
// a module system preference.
//
// How it works: we scan .js/.mjs/.cjs files under src/ (or common JS source
// dirs) looking for CJS patterns (require(), module.exports) and ESM patterns
// (import/export). If both appear, we flag it.
//
// Gotcha: .mjs and .cjs extensions are unambiguous by definition, so we only
// check .js files for the actual pattern conflict.
type ModuleConflict struct{}

func (d ModuleConflict) Name() string { return "module-conflict" }

// Patterns for detecting module systems in .js files.
var (
	// CJS patterns: require('...') or module.exports
	cjsRequireRe = regexp.MustCompile(`\brequire\s*\(`)
	cjsExportsRe = regexp.MustCompile(`\bmodule\.exports\b`)

	// ESM patterns: import ... from or export (default|const|function|class|{)
	esmImportRe = regexp.MustCompile(`^\s*import\s+`)
	esmExportRe = regexp.MustCompile(`^\s*export\s+`)
)

func (d ModuleConflict) Detect(root string) ([]model.Section, error) {
	// Only relevant if this is a JS/TS project.
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		return nil, nil
	}

	// Source directories to scan (in order of commonality).
	srcDirs := []string{"src", "lib", "app", "pages", "components"}

	var scanRoot string
	for _, dir := range srcDirs {
		candidate := filepath.Join(root, dir)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			scanRoot = candidate
			break
		}
	}
	if scanRoot == "" {
		return nil, nil
	}

	var cjsFiles, esmFiles []string

	filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)

		// .mjs is always ESM, .cjs is always CJS — no ambiguity.
		switch ext {
		case ".mjs":
			rel, _ := filepath.Rel(root, path)
			esmFiles = append(esmFiles, rel)
			return nil
		case ".cjs":
			rel, _ := filepath.Rel(root, path)
			cjsFiles = append(cjsFiles, rel)
			return nil
		case ".js":
			// Fall through to content analysis.
		default:
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		rel, _ := filepath.Rel(root, path)

		hasCJS := cjsRequireRe.MatchString(content) || cjsExportsRe.MatchString(content)
		hasESM := false
		for _, line := range strings.Split(content, "\n") {
			if esmImportRe.MatchString(line) || esmExportRe.MatchString(line) {
				hasESM = true
				break
			}
		}

		if hasCJS {
			cjsFiles = append(cjsFiles, rel)
		}
		if hasESM {
			esmFiles = append(esmFiles, rel)
		}

		return nil
	})

	if len(cjsFiles) > 0 && len(esmFiles) > 0 {
		body := fmt.Sprintf(
			"Project mixes CommonJS and ES modules. Found %d file(s) using require()/module.exports and %d file(s) using import/export. This can cause subtle interop issues.",
			len(cjsFiles), len(esmFiles),
		)
		return []model.Section{
			model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Mixed ESM/CJS module systems",
				Body:     body,
				Source:   "module-conflict",
			},
		}, nil
	}

	return nil, nil
}
