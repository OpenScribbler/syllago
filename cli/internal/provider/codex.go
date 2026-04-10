package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Codex = Provider{
	Name:      "Codex",
	Slug:      "codex",
	ConfigDir: ".codex",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return filepath.Join(homeDir, ".codex")
		case catalog.Commands:
			return filepath.Join(homeDir, ".codex")
		case catalog.Agents:
			return filepath.Join(homeDir, ".codex")
		case catalog.Skills:
			return filepath.Join(homeDir, ".agents", "skills")
		case catalog.MCP:
			return "__json_merge__"
		case catalog.Hooks:
			return filepath.Join(homeDir, ".codex")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for .codex directory
		info, err := os.Stat(filepath.Join(homeDir, ".codex"))
		if err == nil && info.IsDir() {
			return true
		}
		// Also check if codex command exists
		_, err = exec.LookPath("codex")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".codex", "commands")}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".codex", "agents")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".agents", "skills")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".codex", "hooks.json")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Agents:
			return FormatTOML
		case catalog.MCP:
			return FormatTOML // Codex MCP config lives in .codex/config.toml
		case catalog.Hooks:
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
		case catalog.Rules, catalog.Commands, catalog.Agents, catalog.Skills, catalog.MCP, catalog.Hooks:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Commands: true,
		catalog.Agents:   true,
		catalog.Skills:   true,
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".codex/hooks.json",
		catalog.MCP:   ".codex/config.toml",
	},
	MCPTransports: []string{"stdio"},
	HookTypes:     []string{"command"},
}
