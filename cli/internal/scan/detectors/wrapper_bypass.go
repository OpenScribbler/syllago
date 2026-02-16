package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// WrapperBypass detects internal wrapper modules in util/, lib/, common/, or
// helpers/ directories that re-export popular libraries. These wrappers exist
// so the team can swap implementations, add logging, or configure defaults —
// but new contributors often bypass them and import the library directly.
//
// Why this matters: if someone wraps axios in lib/http.ts with custom error
// handling, and a new contributor imports axios directly, they skip that error
// handling. The wrapper exists for a reason, but it's invisible unless you
// know to look for it.
//
// How it works: we scan known wrapper directories for files that import
// popular libraries and re-export or wrap them. We flag these as conventions
// that contributors should be aware of.
type WrapperBypass struct{}

func (d WrapperBypass) Name() string { return "wrapper-bypass" }

// popularLibraries that are commonly wrapped internally.
var popularLibraries = []string{
	"axios", "fetch", "got", "node-fetch", "ky",
	"lodash", "underscore", "ramda",
	"moment", "dayjs", "date-fns", "luxon",
	"winston", "pino", "bunyan", "log4js",
	"joi", "yup", "zod",
	"redis", "ioredis",
	"pg", "mysql2", "better-sqlite3",
	"fs-extra",
	"chalk", "picocolors",
}

// reExportRe matches common re-export patterns:
//   export { something } from '...'
//   export default something
//   module.exports = wrappedThing
var reExportRe = regexp.MustCompile(`(?m)^\s*(export\s+(default\s+|{))|module\.exports\s*=`)

func (d WrapperBypass) Detect(root string) ([]model.Section, error) {
	// Only relevant for JS/TS projects.
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		return nil, nil
	}

	wrapperDirs := []string{"util", "utils", "lib", "common", "helpers", "shared"}

	// Build a set of popular library names for fast lookup.
	libSet := make(map[string]bool, len(popularLibraries))
	for _, lib := range popularLibraries {
		libSet[lib] = true
	}

	type wrapperHit struct {
		file    string
		library string
	}
	var hits []wrapperHit

	for _, dir := range wrapperDirs {
		// Check both root-level and src/-level wrapper dirs.
		candidates := []string{
			filepath.Join(root, dir),
			filepath.Join(root, "src", dir),
		}
		for _, candidate := range candidates {
			if !dirExists(candidate) {
				continue
			}

			filepath.Walk(candidate, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				ext := filepath.Ext(path)
				if ext != ".js" && ext != ".ts" && ext != ".mjs" && ext != ".cjs" {
					return nil
				}

				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil
				}
				content := string(data)

				// Check if this file imports a popular library and re-exports something.
				for _, lib := range popularLibraries {
					// Look for import patterns: import ... from 'lib' or require('lib')
					importPattern := fmt.Sprintf(`['"]%s['"]`, regexp.QuoteMeta(lib))
					matched, _ := regexp.MatchString(importPattern, content)
					if !matched {
						continue
					}

					// Check if it also exports (wrapper pattern).
					if reExportRe.MatchString(content) {
						rel, _ := filepath.Rel(root, path)
						hits = append(hits, wrapperHit{file: rel, library: lib})
						break // One hit per file is enough.
					}
				}

				return nil
			})
		}
	}

	if len(hits) == 0 {
		return nil, nil
	}

	// Build a readable summary.
	var lines []string
	for _, h := range hits {
		lines = append(lines, fmt.Sprintf("- %s wraps %s", h.file, h.library))
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Internal library wrappers detected",
			Body: fmt.Sprintf(
				"Found %d internal wrapper(s) that re-export popular libraries. Use these instead of importing the library directly:\n%s",
				len(hits), strings.Join(lines, "\n"),
			),
			Source: "wrapper-bypass",
		},
	}, nil
}
