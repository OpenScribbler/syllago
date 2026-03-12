package catalog

import (
	"bytes"
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the YAML frontmatter parsed from content definition files
// (SKILL.md, AGENT.md).
type Frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Providers   []string `yaml:"providers"` // provider slugs, e.g. ["claude-code", "gemini-cli"]
}

// ParseFrontmatter extracts YAML frontmatter from a markdown file's content.
// The expected format is:
//
//	---
//	name: skill-name
//	description: Skill description text
//	---
//
// Returns the parsed frontmatter and nil error, or zero value and error if
// no valid frontmatter is found.
func ParseFrontmatter(content []byte) (Frontmatter, error) {
	fm, _, err := ParseFrontmatterWithBody(content)
	return fm, err
}

// ParseFrontmatterWithBody extracts YAML frontmatter and the body text that follows it.
// Returns the parsed frontmatter, the body (everything after the closing ---), and any error.
func ParseFrontmatterWithBody(content []byte) (Frontmatter, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return Frontmatter{}, "", errors.New("no frontmatter: content does not start with ---")
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		if bytes.HasSuffix(bytes.TrimRight(rest, " \t"), []byte("---")) {
			closingIdx = bytes.Index(rest, []byte("---"))
		}
		if closingIdx == -1 {
			return Frontmatter{}, "", errors.New("no frontmatter: closing --- not found")
		}
	}

	yamlBytes := rest[:closingIdx]

	var fm Frontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return Frontmatter{}, "", err
	}

	// Body is everything after the closing "---\n" (or "---" at EOF).
	bodyStart := closingIdx + len(opening)
	if bodyStart > len(rest) {
		bodyStart = len(rest)
	}
	body := strings.TrimSpace(string(rest[bodyStart:]))

	return fm, body, nil
}
