package parse

import (
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// ClaudeParser parses Claude Code provider files.
type ClaudeParser struct{}

func (p ClaudeParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	switch file.ContentType {
	case catalog.Rules:
		return p.parseRule(file.Path, content)
	case catalog.MCP:
		return p.parseMCP(file.Path, content)
	case catalog.Hooks:
		return p.parseHooks(file.Path, content)
	default:
		return p.parseGenericMarkdown(file, content)
	}
}

func (p ClaudeParser) parseRule(path string, content []byte) ([]model.Section, error) {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "Rule: " + name,
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseMCP(path string, content []byte) ([]model.Section, error) {
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "MCP Configuration",
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseHooks(path string, content []byte) ([]model.Section, error) {
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "Hooks Configuration",
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseGenericMarkdown(file DiscoveredFile, content []byte) ([]model.Section, error) {
	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:claude-code:" + file.Path,
		},
	}, nil
}
