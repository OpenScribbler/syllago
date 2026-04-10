package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var GeminiCLI = Provider{
	Name:      "Gemini CLI",
	Slug:      "gemini-cli",
	ConfigDir: ".gemini",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".gemini")
		switch ct {
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Rules:
			return base // GEMINI.md goes in .gemini/
		case catalog.Hooks:
			return JSONMergeSentinel
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".gemini"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "GEMINI.md")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".gemini", "commands")}
		case catalog.Skills:
			return []string{
				filepath.Join(projectRoot, ".gemini", "skills"),
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".gemini", "agents")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".gemini", "settings.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".gemini", "settings.json")}
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
		return filepath.Join(projectRoot, "GEMINI.md")
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
	GlobalSharedReadPaths: func(homeDir string, ct catalog.ContentType) []string {
		if ct == catalog.Skills {
			return []string{filepath.Join(homeDir, ".agents", "skills")}
		}
		return nil
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".gemini/settings.json",
		catalog.MCP:   ".gemini/settings.json",
	},
	MCPTransports: []string{"stdio", "sse"},
	HookTypes:     []string{"command"},
}
