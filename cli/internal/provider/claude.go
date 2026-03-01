package provider

import (
	"os"
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
		info, err := os.Stat(filepath.Join(homeDir, ".claude"))
		return err == nil && info.IsDir()
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
			return []string{filepath.Join(projectRoot, ".claude.json")}
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
}
