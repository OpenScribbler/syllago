package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var ClaudeCode = Provider{
	Name:      "Claude Code",
	Slug:      "claude-code",
	ConfigDir: ".claude",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".claude")
		switch ct {
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Rules:
			return filepath.Join(base, "rules")
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.MCP:
			return JSONMergeSentinel
		case catalog.Hooks:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Advisory only — see Provider.Detect doc. Trust the claude binary on
		// PATH or the ~/.claude.json marker file (Claude Code writes it on
		// first launch; syllago never does). The bare ~/.claude/ directory is
		// not evidence — syllago itself populates it with skills/, rules/, etc.
		return binaryOnPath("claude") || fileExists(filepath.Join(homeDir, ".claude.json"))
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, "CLAUDE.md"),
				filepath.Join(projectRoot, ".claude", "rules"),
			}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".claude", "commands")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".claude", "skills")}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".claude", "agents")}
		case catalog.MCP:
			return []string{
				filepath.Join(projectRoot, ".claude.json"),
				filepath.Join(projectRoot, ".mcp.json"),
			}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".claude", "settings.json")}
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
		return filepath.Join(projectRoot, "CLAUDE.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Agents, catalog.Commands, catalog.MCP, catalog.Hooks, catalog.Loadouts:
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
		catalog.Hooks: ".claude/settings.json",
		catalog.MCP:   ".mcp.json",
	},
	MCPTransports: []string{"stdio", "sse", "streamable-http"},
	HookTypes:     []string{"command", "http", "prompt", "agent"},
}
