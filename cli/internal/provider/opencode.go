package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var OpenCode = Provider{
	Name:      "OpenCode",
	Slug:      "opencode",
	ConfigDir: ".config/opencode",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".config", "opencode")
		switch ct {
		case catalog.Rules:
			return base // AGENTS.md lives in home config dir
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Advisory only — see Provider.Detect doc. ~/.config/opencode/ is shared
		// with syllago's content install paths, so we trust the opencode binary
		// on PATH instead.
		return binaryOnPath("opencode")
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, "AGENTS.md"),
				filepath.Join(projectRoot, "CLAUDE.md"),
			}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".opencode", "commands")}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".opencode", "agents")}
		case catalog.Skills:
			return []string{
				filepath.Join(projectRoot, ".opencode", "skills"),
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		case catalog.MCP:
			return []string{
				filepath.Join(projectRoot, "opencode.json"),
				filepath.Join(projectRoot, "opencode.jsonc"),
			}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.MCP:
			return FormatJSONC
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Commands, catalog.Agents, catalog.Skills, catalog.MCP:
			return true
		default:
			return false
		}
	},
	GlobalSharedReadPaths: func(homeDir string, ct catalog.ContentType) []string {
		if ct == catalog.Skills {
			return []string{filepath.Join(homeDir, ".agents", "skills")}
		}
		return nil
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Commands: true,
		catalog.Agents:   true,
		catalog.Skills:   true,
		catalog.MCP:      false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.MCP: "opencode.json",
	},
	MCPTransports: []string{"stdio"},
}
