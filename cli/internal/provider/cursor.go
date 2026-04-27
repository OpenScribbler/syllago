package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Cursor = Provider{
	Name:      "Cursor",
	Slug:      "cursor",
	ConfigDir: ".cursor",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".cursor")
		switch ct {
		case catalog.Rules:
			return base
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.Hooks:
			return JSONMergeSentinel
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Advisory only — see Provider.Detect doc. ~/.cursor/ is shared with
		// syllago install paths (skills/, agents/, mcp.json), so trust the
		// cursor binary on PATH or the Electron app-data dir
		// (~/.config/Cursor on Linux, ~/Library/Application Support/Cursor on
		// macOS) — that path is created by Cursor itself.
		return binaryOnPath("cursor") || dirExists(appDataDir(homeDir, "Cursor"))
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, ".cursor", "rules"),
				filepath.Join(projectRoot, ".cursorrules"),
			}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".cursor", "skills")}
		case catalog.Agents:
			return []string{
				filepath.Join(projectRoot, ".cursor", "agents"),
				filepath.Join(projectRoot, "AGENTS.md"),
			}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".cursor", "settings.json")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".cursor", "mcp.json")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Rules:
			return FormatMDC
		case catalog.Hooks, catalog.MCP:
			return FormatJSON
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".cursor", "rules", "syllago-context.mdc")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Agents:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Skills: true,
		catalog.Agents: true,
		catalog.Hooks:  false, // JSON merge
		catalog.MCP:    false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".cursor/settings.json",
		catalog.MCP:   ".cursor/mcp.json",
	},
	MCPTransports: []string{"stdio", "sse", "streamable-http"},
	HookTypes:     []string{"command"},
}
