package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// TestConvention detects mixed .test.* / .spec.* naming in JS/TS projects.
// Inconsistent naming makes test discovery harder and signals missing linting
// conventions. Flags when both patterns have >20% share of test files.
type TestConvention struct{}

func (d TestConvention) Name() string { return "test-convention" }

func (d TestConvention) Detect(root string) ([]model.Section, error) {
	srcDir := filepath.Join(root, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, nil
	}

	var testCount, specCount int

	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		name := info.Name()
		if isTestFile(name) {
			testCount++
		} else if isSpecFile(name) {
			specCount++
		}
		return nil
	})

	total := testCount + specCount
	if total == 0 {
		return nil, nil
	}

	testPct := float64(testCount) / float64(total) * 100
	specPct := float64(specCount) / float64(total) * 100

	// Both patterns must exceed 20% to be considered a real mix
	if testPct <= 20 || specPct <= 20 {
		return nil, nil
	}

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Mixed Test Naming Conventions",
		Body: fmt.Sprintf(
			"Found %d .test.* files (%.0f%%) and %d .spec.* files (%.0f%%) in src/. "+
				"Pick one convention and rename for consistency.",
			testCount, testPct, specCount, specPct,
		),
		Source: d.Name(),
	}}, nil
}

// isTestFile checks for patterns like foo.test.ts, foo.test.js, foo.test.tsx, etc.
func isTestFile(name string) bool {
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return false
	}
	return parts[len(parts)-2] == "test" && isJSTSExt(parts[len(parts)-1])
}

// isSpecFile checks for patterns like foo.spec.ts, foo.spec.js, etc.
func isSpecFile(name string) bool {
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return false
	}
	return parts[len(parts)-2] == "spec" && isJSTSExt(parts[len(parts)-1])
}

func isJSTSExt(ext string) bool {
	switch ext {
	case "js", "jsx", "ts", "tsx", "mjs", "cjs":
		return true
	}
	return false
}
