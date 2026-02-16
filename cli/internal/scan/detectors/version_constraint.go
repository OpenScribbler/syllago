package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// VersionConstraint detects when code uses language features that require a
// newer version than what the project declares. Currently checks Go generics
// (requires Go 1.18+) against the go directive in go.mod.
//
// Why this matters: the code will compile on the developer's machine (which
// has a newer Go) but fail in CI or for contributors using the declared version.
type VersionConstraint struct{}

func (d VersionConstraint) Name() string { return "version-constraint" }

func (d VersionConstraint) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := detectGoGenericsConstraint(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

// genericsPattern matches Go type parameter syntax: func Foo[T any], type Bar[T comparable], etc.
// Looks for an identifier followed by a bracket containing a type parameter list.
var genericsPattern = regexp.MustCompile(`\[(\w+)\s+(any|comparable|~?\w+)`)

func detectGoGenericsConstraint(root string) *model.TextSection {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}

	goVer := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			goVer = strings.TrimPrefix(line, "go ")
			break
		}
	}
	if goVer == "" {
		return nil
	}

	major, minor := parseGoVersion(goVer)
	if major > 1 || (major == 1 && minor >= 18) {
		return nil // version supports generics, no constraint violation
	}

	// Walk .go files looking for generics syntax.
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if found {
			return filepath.SkipAll
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if genericsPattern.Match(src) {
			found = true
		}
		return nil
	})

	if !found {
		return nil
	}

	return &model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Go Version vs Generics Usage",
		Body:     fmt.Sprintf("go.mod declares Go %s, but source files use generics (type parameters) which require Go 1.18+. The code may not compile with the declared version.", goVer),
		Source:   "version-constraint",
	}
}

// parseGoVersion extracts major and minor from a Go version string like "1.17" or "1.22.5".
func parseGoVersion(ver string) (int, int) {
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) < 2 {
		return 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	return major, minor
}
