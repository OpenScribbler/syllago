package provider

import (
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Crush is the Charmbracelet coding agent. XDG-compliant: global config
// lives under ~/.config/crush/. Supports rules (AGENTS.md project only),
// skills (Agent Skills standard), MCP, and hooks (PreToolUse only, flat
// entries in crush.json — see docs/provider-formats/crush.yaml). No
// user-definable agents, no custom commands.
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
		case catalog.Hooks:
			return JSONMergeSentinel // merged into crush.json hooks key
		}
		return ""
	},
	Detect: func(_ string) bool {
		// Advisory only — see Provider.Detect doc.
		return binaryOnPath("crush")
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".crush", "skills")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, "crush.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, "crush.json")}
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
		case catalog.Rules, catalog.Skills, catalog.MCP, catalog.Hooks:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Skills: true,
		catalog.MCP:    false, // JSON merge
		catalog.Hooks:  false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.MCP:   "crush.json",
		catalog.Hooks: "crush.json",
	},
	MCPTransports: []string{"stdio", "http", "sse"},
	HookTypes:     []string{"command"},
}
