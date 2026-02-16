package detectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// TechStack detects the primary technology stack from manifest files.
type TechStack struct{}

func (d TechStack) Name() string { return "tech-stack" }

func (d TechStack) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := detectGo(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectNode(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectPython(root); s != nil {
		sections = append(sections, *s)
	}
	if s := detectRust(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

func detectGo(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Go",
		Language: "Go",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			s.LanguageVersion = strings.TrimPrefix(line, "go ")
			break
		}
	}

	return s
}

func detectNode(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Engines      map[string]string `json:"engines"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: JavaScript/TypeScript",
		Language: "JavaScript",
	}

	if _, ok := pkg.Dependencies["typescript"]; ok {
		s.Language = "TypeScript"
	}
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		s.Language = "TypeScript"
	}

	if v, ok := pkg.Engines["node"]; ok {
		s.RuntimeVersion = v
		s.Runtime = "Node.js"
	}

	// Ordered by specificity: meta-frameworks before their underlying libraries.
	// Slice gives deterministic iteration (Go maps randomize order).
	frameworks := []struct{ dep, name string }{
		{"next", "Next.js"}, {"astro", "Astro"}, {"svelte", "Svelte"},
		{"vue", "Vue"}, {"react", "React"},
		{"express", "Express"}, {"fastify", "Fastify"},
	}
	for _, fw := range frameworks {
		if v, ok := pkg.Dependencies[fw.dep]; ok {
			s.Framework = fw.name
			s.FrameworkVersion = strings.TrimPrefix(v, "^")
			break
		}
	}

	return s
}

func detectPython(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		if _, err2 := os.Stat(filepath.Join(root, "setup.py")); err2 != nil {
			if _, err3 := os.Stat(filepath.Join(root, "requirements.txt")); err3 != nil {
				return nil
			}
		}
		return &model.TechStackSection{
			Origin:   model.OriginAuto,
			Title:    "Tech Stack: Python",
			Language: "Python",
		}
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Python",
		Language: "Python",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "requires-python") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				s.LanguageVersion = strings.Trim(strings.TrimSpace(parts[1]), "\"'>= ")
			}
		}
	}

	return s
}

func detectRust(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Rust",
		Language: "Rust",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "edition") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				s.LanguageVersion = "edition " + strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
	}

	return s
}
