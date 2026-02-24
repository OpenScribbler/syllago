package parse

import (
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/model"
)

// GenericParser handles providers that use standard markdown files.
type GenericParser struct {
	ProviderSlug string
}

func (p GenericParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:" + p.ProviderSlug + ":" + file.Path,
		},
	}, nil
}

// ParserForProvider returns the appropriate parser for a provider slug.
func ParserForProvider(slug string) Parser {
	switch slug {
	case "claude-code":
		return ClaudeParser{}
	case "cursor":
		return CursorParser{}
	default:
		return GenericParser{ProviderSlug: slug}
	}
}
