package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Crush is the Charmbracelet coding agent. XDG-compliant: global config
// lives under ~/.config/crush/. Supports rules (AGENTS.md project only),
// skills (Agent Skills standard), and MCP. No hooks (see charmbracelet/crush#2038),
// no user-definable agents, no custom commands.
var Crush = Provider{
	Name:      "Crush",
	Slug:      "crush",
	ConfigDir: ".config/crush",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // AGENTS.md at project root
		case catalog.Skills:
			return filepath.Join(homeDir, ".config", "crush", "skills")
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".config", "crush"))
		if err == nil && info.IsDir() {
			return true
		}
		_, err = exec.LookPath("crush")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".crush", "skills")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, "crush.json")}
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
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.MCP:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Skills: true,
		catalog.MCP:    false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.MCP: "crush.json",
	},
	MCPTransports: []string{"stdio", "http", "sse"},
}
