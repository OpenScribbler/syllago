package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var FactoryDroid = Provider{
	Name:      "Factory Droid",
	Slug:      "factory-droid",
	ConfigDir: ".factory",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".factory")
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // AGENTS.md lives at project root
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Agents:
			return filepath.Join(base, "droids") // Factory calls them "Custom Droids"
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.MCP:
			return JSONMergeSentinel
		case catalog.Hooks:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(_ string) bool {
		// Advisory only — see Provider.Detect doc. Factory ships its CLI as
		// `droid`, not `factory`.
		return binaryOnPath("droid")
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".factory", "skills")}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".factory", "droids")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".factory", "commands")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".factory", "mcp.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".factory", "settings.json")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.MCP, catalog.Hooks:
			return FormatJSON
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Agents, catalog.Commands, catalog.MCP, catalog.Hooks:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Skills:   true,
		catalog.Agents:   true,
		catalog.Commands: true,
		catalog.MCP:      false, // JSON merge
		catalog.Hooks:    false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".factory/settings.json",
		catalog.MCP:   ".factory/mcp.json",
	},
	MCPTransports: []string{"stdio", "http"},
	HookTypes:     []string{"command"},
}
