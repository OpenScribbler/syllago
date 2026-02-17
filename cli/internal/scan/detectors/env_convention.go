package detectors

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// EnvConvention detects environment variables referenced in source code that
// aren't documented in .env.example. This is a common onboarding friction
// point — a new developer clones the repo, copies .env.example to .env, and
// then gets runtime errors because some env vars were never documented.
//
// If there's no .env.example, the detector does nothing (we can't know what's
// "documented" without a reference file).
type EnvConvention struct{}

func (d EnvConvention) Name() string { return "env-convention" }

func (d EnvConvention) Detect(root string) ([]model.Section, error) {
	documented, err := parseEnvExample(filepath.Join(root, ".env.example"))
	if err != nil {
		return nil, nil // no .env.example → nothing to compare against
	}

	referenced := findReferencedEnvVars(root)

	var undocumented []string
	for v := range referenced {
		if !documented[v] {
			undocumented = append(undocumented, v)
		}
	}
	if len(undocumented) == 0 {
		return nil, nil
	}

	sort.Strings(undocumented)

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Undocumented Environment Variables",
			Body:     fmt.Sprintf("These env vars are used in code but not listed in .env.example: %s", strings.Join(undocumented, ", ")),
			Source:   "env-convention",
		},
	}, nil
}

// parseEnvExample reads a .env.example file and returns the set of variable
// names it defines. Lines are KEY=value or KEY= (empty value). Comments (#)
// and blank lines are skipped.
func parseEnvExample(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	vars := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) >= 1 {
			key := strings.TrimSpace(parts[0])
			if key != "" {
				vars[key] = true
			}
		}
	}
	return vars, nil
}

// Patterns that capture env var names from different languages:
//   - process.env.VAR_NAME        (JS/TS)
//   - import.meta.env.VAR_NAME    (Vite/Astro)
//   - os.Getenv("VAR_NAME")       (Go)
//   - os.environ["VAR_NAME"]      (Python)
//   - os.getenv("VAR_NAME")       (Python)
var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`import\.meta\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`os\.Getenv\("([A-Z_][A-Z0-9_]*)"\)`),
	regexp.MustCompile(`os\.environ\["([A-Z_][A-Z0-9_]*)"\]`),
	regexp.MustCompile(`os\.getenv\("([A-Z_][A-Z0-9_]*)"\)`),
}

// sourceExtensions is the set of file extensions to scan for env var references.
var sourceExtensions = map[string]bool{
	".ts": true, ".tsx": true,
	".js": true, ".jsx": true,
	".go": true,
	".py": true,
}

func findReferencedEnvVars(root string) map[string]bool {
	vars := make(map[string]bool)

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(d.Name())
		if !sourceExtensions[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		for _, pat := range envPatterns {
			for _, match := range pat.FindAllStringSubmatch(content, -1) {
				if len(match) >= 2 {
					vars[match[1]] = true
				}
			}
		}
		return nil
	})

	return vars
}
