package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
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
		case catalog.Commands:
			return filepath.Join(base, "commands")
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
		// Check for .copilot directory
		info, err := os.Stat(filepath.Join(homeDir, ".copilot"))
		if err == nil && info.IsDir() {
			return true
		}
		// Also check if gh copilot extension exists
		_, err = exec.LookPath("gh")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, ".github", "copilot-instructions.md")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".copilot", "commands")}
		case catalog.Agents:
			return []string{
				filepath.Join(projectRoot, ".copilot", "agents"),
				filepath.Join(projectRoot, ".github", "agents"),
				filepath.Join(projectRoot, ".claude", "agents"), // compatibility fallback
			}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".copilot", "mcp.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".copilot", "hooks.json")}
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
		case catalog.Rules, catalog.Commands, catalog.Agents, catalog.Hooks, catalog.MCP:
			return true
		default:
			return false
		}
	},
}
