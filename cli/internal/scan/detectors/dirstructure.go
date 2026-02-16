package detectors

import (
	"os"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// DirectoryStructure detects top-level project directories and classifies them.
type DirectoryStructure struct{}

func (d DirectoryStructure) Name() string { return "directory-structure" }

func (d DirectoryStructure) Detect(root string) ([]model.Section, error) {
	s := &model.DirectoryStructureSection{
		Origin: model.OriginAuto,
		Title:  "Directory Structure",
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !e.IsDir() {
			continue
		}

		entry := model.DirectoryEntry{
			Path:       name + "/",
			Convention: classifyDir(name),
		}
		s.Entries = append(s.Entries, entry)
	}

	if len(s.Entries) == 0 {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

func classifyDir(name string) string {
	switch strings.ToLower(name) {
	case "src", "lib", "pkg", "internal", "app":
		return "source"
	case "test", "tests", "__tests__", "spec", "specs":
		return "test"
	case "cmd", "bin":
		return "entrypoint"
	case "docs", "doc", "documentation":
		return "documentation"
	case "config", "conf", "cfg":
		return "config"
	case "build", "dist", "out", "target":
		return "build-output"
	case "scripts":
		return "scripts"
	case "vendor", "node_modules", "third_party":
		return "vendor"
	case "assets", "static", "public":
		return "assets"
	case "migrations", "db":
		return "database"
	default:
		return ""
	}
}
