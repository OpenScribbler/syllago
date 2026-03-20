package parse

import (
	"bytes"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// CursorFrontmatter represents the YAML frontmatter in .mdc files.
type CursorFrontmatter struct {
	Description string   `yaml:"description"`
	Globs       []string `yaml:"globs,omitempty"`
	AlwaysApply bool     `yaml:"alwaysApply,omitempty"`
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
	var fm CursorFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return CursorFrontmatter{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return fm, body, nil
}
