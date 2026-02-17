package detectors

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// DeprecatedPattern detects signs of accumulated technical debt: @deprecated
// annotations, // DEPRECATED comments, TODO: migrate markers, and legacy/
// directories. Flags when the total count exceeds 5, which suggests the project
// has deferred cleanup work worth surfacing.
type DeprecatedPattern struct{}

func (d DeprecatedPattern) Name() string { return "deprecated-pattern" }

// deprecatedMarkers are the strings we search for inside file contents.
var deprecatedMarkers = []string{
	"@deprecated",
	"// DEPRECATED",
	"TODO: migrate",
}

func (d DeprecatedPattern) Detect(root string) ([]model.Section, error) {
	count := 0
	var details []string

	// Check for legacy/ directories
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			if name == "legacy" {
				rel, _ := filepath.Rel(root, path)
				details = append(details, fmt.Sprintf("legacy/ directory at %s", rel))
				count++
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan text-like files by extension
		if !isScannable(d.Name()) {
			return nil
		}

		hits := scanFileForMarkers(path)
		count += hits
		return nil
	})

	if count <= 5 {
		return nil, nil
	}

	body := fmt.Sprintf("Found %d deprecation/migration markers across the project", count)
	if len(details) > 0 {
		body += " (" + strings.Join(details, ", ") + ")"
	}
	body += ". Consider scheduling cleanup."

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Accumulated Deprecation Markers",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// scanFileForMarkers counts how many deprecated marker lines appear in a file.
func scanFileForMarkers(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		for _, marker := range deprecatedMarkers {
			if strings.Contains(line, marker) {
				count++
				break // one hit per line max
			}
		}
	}
	return count
}

// isScannable returns true for common source/config file extensions.
func isScannable(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".rb", ".java",
		".rs", ".c", ".cpp", ".h", ".cs", ".swift", ".kt",
		".yaml", ".yml", ".toml", ".json", ".md", ".txt":
		return true
	}
	return false
}
