package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Zed = Provider{
	Name:      "Zed",
	Slug:      "zed",
	ConfigDir: ".config/zed",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Rules go in project root as .rules
		case catalog.MCP:
			return JSONMergeSentinel // Merges into ~/.config/zed/settings.json
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Advisory only — see Provider.Detect doc. Zed writes
		// ~/.config/zed/settings.json on first launch; syllago does not
		// create that file (Zed Rules go to project root). Trust the zed
		// binary on PATH or that marker file.
		return binaryOnPath("zed") || fileExists(filepath.Join(homeDir, ".config", "zed", "settings.json"))
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, ".rules"),
				filepath.Join(projectRoot, ".cursorrules"),
				filepath.Join(projectRoot, "CLAUDE.md"),
			}
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
		return filepath.Join(projectRoot, ".rules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.MCP:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules: true,
		catalog.MCP:   false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.MCP: "~/.config/zed/settings.json",
	},
	MCPTransports: []string{"stdio"},
}
