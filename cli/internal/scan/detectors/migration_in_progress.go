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

// MigrationInProgress detects signs of incomplete migrations:
//   - .js + .ts coexistence (JavaScript-to-TypeScript migration)
//   - React class components + function components (legacy React migration)
//   - Next.js pages/ + app/ coexistence (Next.js App Router migration)
//
// Why this matters: mid-migration projects have two ways of doing the same
// thing. Without knowing which is "old" and which is "new," contributors
// often follow the wrong pattern — extending the legacy approach instead
// of using the new one.
//
// How it works: straightforward filesystem checks. We count file extensions
// and scan for class component patterns. No deep AST parsing — just enough
// signal to flag the migration.
type MigrationInProgress struct{}

func (d MigrationInProgress) Name() string { return "migration-in-progress" }

// classComponentRe matches React class component declarations:
//   class Foo extends React.Component
//   class Foo extends Component
var classComponentRe = regexp.MustCompile(`class\s+\w+\s+extends\s+(React\.)?Component`)

func (d MigrationInProgress) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := detectJsTsMigration(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectReactComponentMigration(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectNextRouterMigration(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

// detectJsTsMigration checks for .js and .ts files coexisting in source dirs,
// which signals an incomplete JS-to-TS migration.
func detectJsTsMigration(root string) *model.TextSection {
	// Must be a TS project (has tsconfig.json) for this to be a migration signal.
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err != nil {
		return nil
	}

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
		return nil
	}

	var jsCount, tsCount int

	filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".js", ".jsx":
			jsCount++
		case ".ts", ".tsx":
			tsCount++
		}
		return nil
	})

	if jsCount > 0 && tsCount > 0 {
		return &model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "JS-to-TypeScript migration in progress",
			Body: fmt.Sprintf(
				"Found %d .js/.jsx and %d .ts/.tsx files in a TypeScript project. New code should likely use TypeScript.",
				jsCount, tsCount,
			),
			Source: "migration-in-progress",
		}
	}
	return nil
}

// detectReactComponentMigration looks for both class components and function
// components in .jsx/.tsx files, suggesting a React modernization migration.
func detectReactComponentMigration(root string) *model.TextSection {
	// Quick check: must have React as a dependency.
	deps := collectAllDeps(root)
	if deps == nil || (!deps["react"] && !deps["react-dom"]) {
		return nil
	}

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
		return nil
	}

	var classCount, functionCount int

	filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".jsx" && ext != ".tsx" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)

		if classComponentRe.MatchString(content) {
			classCount++
		}
		// Function components: files with JSX that don't use class extends Component.
		// A simple heuristic: if the file has a default export function or arrow function.
		if strings.Contains(content, "useState") || strings.Contains(content, "useEffect") ||
			strings.Contains(content, "export default function") {
			functionCount++
		}
		return nil
	})

	if classCount > 0 && functionCount > 0 {
		return &model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "React class-to-function component migration",
			Body: fmt.Sprintf(
				"Found %d file(s) with class components and %d file(s) with function components (hooks). New components should likely use hooks.",
				classCount, functionCount,
			),
			Source: "migration-in-progress",
		}
	}
	return nil
}

// detectNextRouterMigration checks for both pages/ and app/ directories in
// a Next.js project, indicating an App Router migration.
func detectNextRouterMigration(root string) *model.TextSection {
	deps := collectAllDeps(root)
	if deps == nil || !deps["next"] {
		return nil
	}

	pagesDir := filepath.Join(root, "pages")
	appDir := filepath.Join(root, "app")

	// Also check under src/ (common Next.js pattern).
	srcPagesDir := filepath.Join(root, "src", "pages")
	srcAppDir := filepath.Join(root, "src", "app")

	hasPages := dirExists(pagesDir) || dirExists(srcPagesDir)
	hasApp := dirExists(appDir) || dirExists(srcAppDir)

	if hasPages && hasApp {
		return &model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Next.js Pages-to-App Router migration",
			Body:     "Both pages/ and app/ directories exist. This project is migrating from Next.js Pages Router to App Router. New routes should likely use app/.",
			Source:   "migration-in-progress",
		}
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
