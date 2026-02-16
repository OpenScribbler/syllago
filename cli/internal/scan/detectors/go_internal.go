package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// GoInternal finds internal/ directories in a Go project and reports them as
// informational context. Go's internal/ convention enforces import boundaries
// at the compiler level — packages under internal/ can only be imported by
// code in the parent tree. This isn't an error, but it's useful context for
// agents to know which packages are internal and what they provide.
type GoInternal struct{}

func (d GoInternal) Name() string { return "go-internal" }

func (d GoInternal) Detect(root string) ([]model.Section, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, nil
	}

	var internalDirs []string

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			if name == "internal" {
				rel, _ := filepath.Rel(root, path)
				internalDirs = append(internalDirs, rel)
			}
			return nil
		}
		return nil
	})

	if len(internalDirs) == 0 {
		return nil, nil
	}

	body := fmt.Sprintf(
		"Found %d internal/ directory path(s): %s. These packages are only importable by code within their parent module tree (enforced by the Go compiler).",
		len(internalDirs),
		strings.Join(internalDirs, ", "),
	)

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Go Internal Packages",
			Body:     body,
			Source:   d.Name(),
		},
	}, nil
}
