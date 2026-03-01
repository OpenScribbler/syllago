package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Cline = Provider{
	Name:      "Cline",
	Slug:      "cline",
	ConfigDir: ".clinerules",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Rules go in project root as .clinerules/ directory
		case catalog.MCP:
			return JSONMergeSentinel // Merges into VS Code globalStorage config
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for .clinerules/ directory in common project locations
		// Cline is a VS Code extension; detection is project-level
		info, err := os.Stat(filepath.Join(homeDir, "Documents", "Cline", "Rules"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, ".clinerules")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.MCP:
			return FormatJSON
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".clinerules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.MCP:
			return true
		default:
			return false
		}
	},
}
