package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrimaryFileName returns the best default file to display for a content item.
// Skills: SKILL.md; Hooks: first .json/.yaml/.yml; MCP: first .json;
// Agents/Rules: first .md; Commands: first file; Loadouts: loadout.yaml or loadout.yml.
// Returns empty string if no match found.
func PrimaryFileName(files []string, ct ContentType) string {
	for _, f := range files {
		// Use only the base name for matching (files may be relative paths).
		name := strings.ToLower(filepath.Base(f))
		switch ct {
		case Skills:
			if name == "skill.md" {
				return f
			}
		case Agents:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case Rules:
			if strings.HasSuffix(name, ".md") {
				return f
			}
		case Hooks:
			if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				return f
			}
		case MCP:
			if strings.HasSuffix(name, ".json") {
				return f
			}
		case Commands:
			// First file is the primary entry point.
			return f
		case Loadouts:
			if name == "loadout.yaml" || name == "loadout.yml" {
				return f
			}
		}
	}
	return ""
}

// ReadFileContent reads the file at filepath.Join(itemPath, relPath), capping output
// at maxLines lines. If the file exceeds maxLines, the returned string is truncated and
// a "(N more lines)" suffix is appended.
func ReadFileContent(itemPath, relPath string, maxLines int) (string, error) {
	absPath := filepath.Join(itemPath, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", relPath, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		extra := len(lines) - maxLines
		content = strings.Join(lines[:maxLines], "\n")
		content += fmt.Sprintf("\n\n(%d more lines)", extra)
	}

	return content, nil
}
