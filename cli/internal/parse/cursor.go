package parse

import (
	"bytes"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// CursorFrontmatter represents the YAML frontmatter in .mdc files.
// Globs can appear as either a comma-separated string (native Cursor format)
// or a YAML array (canonical format). Both are parsed into []string.
type CursorFrontmatter struct {
	Description string   `yaml:"description"`
	Globs       []string `yaml:"-"`
	AlwaysApply bool     `yaml:"alwaysApply,omitempty"`
}

// cursorFrontmatterRaw is used for initial YAML unmarshaling before
// processing the globs field which can be either a string or array.
type cursorFrontmatterRaw struct {
	Description string    `yaml:"description"`
	Globs       yaml.Node `yaml:"globs,omitempty"`
	AlwaysApply bool      `yaml:"alwaysApply,omitempty"`
}

var errNoFrontmatter = errors.New("no frontmatter found")

// ParseMDCFrontmatter extracts YAML frontmatter and body from .mdc content.
// Returns the parsed frontmatter, the body text, and any error.
func ParseMDCFrontmatter(content []byte) (CursorFrontmatter, string, error) {
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
	var raw cursorFrontmatterRaw
	if err := yaml.Unmarshal(yamlBytes, &raw); err != nil {
		return CursorFrontmatter{}, "", err
	}

	fm := CursorFrontmatter{
		Description: raw.Description,
		AlwaysApply: raw.AlwaysApply,
	}

	// Parse globs: can be a scalar string (comma-separated) or a YAML sequence.
	switch raw.Globs.Kind {
	case yaml.ScalarNode:
		// Comma-separated string: "*.ts, *.tsx"
		for _, g := range strings.Split(raw.Globs.Value, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				fm.Globs = append(fm.Globs, g)
			}
		}
	case yaml.SequenceNode:
		// YAML array: ["*.ts", "*.tsx"]
		for _, item := range raw.Globs.Content {
			if item.Kind == yaml.ScalarNode && item.Value != "" {
				fm.Globs = append(fm.Globs, item.Value)
			}
		}
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return fm, body, nil
}
