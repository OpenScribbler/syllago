package detectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// Dependencies detects project dependencies from manifest files.
type Dependencies struct{}

func (d Dependencies) Name() string { return "dependencies" }

func (d Dependencies) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := detectNodeDeps(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectGoDeps(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

func detectNodeDeps(root string) *model.DependencySection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	s := &model.DependencySection{
		Origin: model.OriginAuto,
		Title:  "Dependencies: Node.js",
	}

	if len(pkg.Dependencies) > 0 {
		group := model.DependencyGroup{Category: "production"}
		for name, ver := range pkg.Dependencies {
			group.Items = append(group.Items, model.Dependency{Name: name, Version: ver})
		}
		s.Groups = append(s.Groups, group)
	}

	if len(pkg.DevDependencies) > 0 {
		group := model.DependencyGroup{Category: "dev"}
		for name, ver := range pkg.DevDependencies {
			group.Items = append(group.Items, model.Dependency{Name: name, Version: ver})
		}
		s.Groups = append(s.Groups, group)
	}

	if len(s.Groups) == 0 {
		return nil
	}
	return s
}

func detectGoDeps(root string) *model.DependencySection {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}

	s := &model.DependencySection{
		Origin: model.OriginAuto,
		Title:  "Dependencies: Go",
	}

	group := model.DependencyGroup{Category: "module"}
	inRequire := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				group.Items = append(group.Items, model.Dependency{
					Name:    parts[0],
					Version: parts[1],
				})
			}
		}
	}

	if len(group.Items) > 0 {
		s.Groups = append(s.Groups, group)
		return s
	}
	return nil
}
