package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var CopilotCLI = Provider{
	Name:      "Copilot CLI",
	Slug:      "copilot-cli",
	ConfigDir: ".copilot",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".copilot")
		switch ct {
		case catalog.Rules:
			return base
		case catalog.Skills:
			return filepath.Join(homeDir, ".github", "skills")
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.Agents:
			return filepath.Join(homeDir, ".github", "agents")
		case catalog.Hooks:
			return JSONMergeSentinel
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(_ string) bool {
		// Advisory only — see Provider.Detect doc. Copilot CLI ships as a gh
		// extension (`gh copilot`); query `gh extension list` rather than
		// trust ~/.copilot/, which syllago also writes into.
		return ghExtensionInstalled("gh-copilot")
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, ".github", "copilot-instructions.md"),
				filepath.Join(projectRoot, "AGENTS.md"),
			}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".copilot", "commands")}
		case catalog.Agents:
			return []string{
				filepath.Join(projectRoot, ".copilot", "agents"),
				filepath.Join(projectRoot, ".github", "agents"),
				filepath.Join(projectRoot, ".claude", "agents"), // compatibility fallback
			}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".github", "skills")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".copilot", "mcp-config.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".github", "hooks")}
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
		return filepath.Join(projectRoot, ".github", "copilot-instructions.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Commands, catalog.Agents, catalog.Hooks, catalog.MCP:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Skills:   true,
		catalog.Commands: true,
		catalog.Agents:   true,
		catalog.Hooks:    false, // JSON merge
		catalog.MCP:      false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".github/hooks/",
		catalog.MCP:   ".copilot/mcp-config.json",
	},
	MCPTransports: []string{"stdio", "sse"},
	HookTypes:     []string{"command"},
}
