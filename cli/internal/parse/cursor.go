package parse

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/model"
	"gopkg.in/yaml.v3"
)

// CursorParser parses Cursor provider files (.mdc format).
type CursorParser struct{}

// CursorFrontmatter represents the YAML frontmatter in .mdc files.
type CursorFrontmatter struct {
	Description string   `yaml:"description"`
	Globs       []string `yaml:"globs,omitempty"`
	AlwaysApply bool     `yaml:"alwaysApply,omitempty"`
}

var errNoFrontmatter = errors.New("no frontmatter found")

func (p CursorParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	if file.ContentType == catalog.Rules && filepath.Ext(file.Path) == ".mdc" {
		return p.parseMDC(file.Path, content)
	}

	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:cursor:" + file.Path,
		},
	}, nil
}

func (p CursorParser) parseMDC(path string, content []byte) ([]model.Section, error) {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".mdc")

	fm, body, err := parseMDCFrontmatter(content)
	if err != nil {
		return []model.Section{
			model.TextSection{
				Category: model.CatConventions,
				Origin:   model.OriginHuman,
				Title:    "Cursor Rule: " + name,
				Body:     string(content),
				Source:   "import:cursor:" + path,
			},
		}, nil
	}

	title := "Cursor Rule: " + name
	if fm.Description != "" {
		title = "Cursor Rule: " + fm.Description
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    title,
			Body:     body,
			Source:   "import:cursor:" + path,
		},
	}, nil
}

func parseMDCFrontmatter(content []byte) (CursorFrontmatter, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return CursorFrontmatter{}, "", errNoFrontmatter
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return CursorFrontmatter{}, "", errNoFrontmatter
	}

	yamlBytes := rest[:closingIdx]
	var fm CursorFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return CursorFrontmatter{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return fm, body, nil
}
