package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// GoCGO detects CGO usage in a Go project by scanning for `import "C"` or
// `//go:build cgo` directives. CGO is a surprise because it introduces C
// compiler dependencies, breaks simple cross-compilation (`GOOS=linux go
// build` won't work without a cross-compiler toolchain), and can affect
// reproducibility.
type GoCGO struct{}

func (d GoCGO) Name() string { return "go-cgo" }

func (d GoCGO) Detect(root string) ([]model.Section, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, nil
	}

	var cgoFiles []string

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)

		if hasCGOUsage(content) {
			rel, _ := filepath.Rel(root, path)
			cgoFiles = append(cgoFiles, rel)
		}
		return nil
	})

	if len(cgoFiles) == 0 {
		return nil, nil
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "CGO Usage Detected",
			Body:     fmt.Sprintf("CGO is used in %d file(s): %s. This requires a C compiler toolchain and affects cross-compilation (CGO_ENABLED=0 may be needed for static builds).", len(cgoFiles), strings.Join(cgoFiles, ", ")),
			Source:   d.Name(),
		},
	}, nil
}

// hasCGOUsage checks whether a Go source file uses CGO via import "C" or
// a //go:build cgo directive.
func hasCGOUsage(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == `import "C"` {
			return true
		}
		if strings.HasPrefix(trimmed, "//go:build ") && strings.Contains(trimmed, "cgo") {
			return true
		}
		// Legacy build tag format.
		if strings.HasPrefix(trimmed, "// +build ") && strings.Contains(trimmed, "cgo") {
			return true
		}
	}
	return false
}
