package parse

import (
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

// ClassifyByExtension provides a fallback classification based on file extension
// and path patterns when provider-specific classification isn't available.
func ClassifyByExtension(filePath string) (catalog.ContentType, bool) {
	base := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	// Provider-specific files by name
	switch strings.ToLower(base) {
	case "claude.md", "gemini.md", "agents.md", "copilot-instructions.md":
		return catalog.Rules, true
	case ".claude.json":
		return catalog.MCP, true
	case "settings.json":
		if strings.Contains(dir, ".claude") {
			return catalog.Hooks, true
		}
	}

	// By directory name — check ancestor directories so files in
	// subdirectories (e.g. skills/my-skill/SKILL.md) still match.
	knownDirs := map[string]catalog.ContentType{
		"rules":    catalog.Rules,
		"skills":   catalog.Skills,
		"agents":   catalog.Agents,
		"commands": catalog.Commands,
		"hooks":    catalog.Hooks,
		"prompts":  catalog.Prompts,
	}
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if ct, ok := knownDirs[filepath.Base(d)]; ok {
			return ct, true
		}
	}

	// By extension
	ext := filepath.Ext(filePath)
	switch ext {
	case ".mdc":
		return catalog.Rules, true
	}

	return "", false
}
