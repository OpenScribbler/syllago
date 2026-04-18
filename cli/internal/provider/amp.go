package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Amp = Provider{
	Name:      "Amp",
	Slug:      "amp",
	ConfigDir: ".config/amp",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			// Amp rules are AGENTS.md files — no dedicated install directory.
			// Rules go to project root or ~/.config/amp/AGENTS.md (global).
			return filepath.Join(homeDir, ".config", "amp")
		case catalog.Skills:
			// Amp skills: ~/.config/agents/skills/ (primary global)
			// Also searches ~/.config/amp/skills/ but we install to the primary.
			return filepath.Join(homeDir, ".config", "agents", "skills")
		case catalog.MCP:
			return JSONMergeSentinel
		case catalog.Hooks:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".config", "amp"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			// Amp checks AGENTS.md, then falls back to AGENT.md and CLAUDE.md
			// if AGENTS.md doesn't exist. It also walks parent directories up
			// to $HOME, but syllago only discovers project-root files.
			return []string{
				filepath.Join(projectRoot, "AGENTS.md"),
				filepath.Join(projectRoot, "AGENT.md"),
				filepath.Join(projectRoot, "CLAUDE.md"),
			}
		case catalog.Skills:
			return []string{
				filepath.Join(projectRoot, ".agents", "skills"),
				filepath.Join(projectRoot, ".claude", "skills"), // compat fallback
			}
		case catalog.MCP:
			// Workspace MCP: .amp/settings.json (requires user approval in Amp).
			// User-level MCP: ~/.config/amp/settings.json under amp.mcpServers key.
			return []string{
				filepath.Join(projectRoot, ".amp", "settings.json"),
			}
		case catalog.Hooks:
			// Hooks are stored as amp.hooks array inside the same settings.json
			// used for MCP servers (workspace or user-level).
			return []string{
				filepath.Join(projectRoot, ".amp", "settings.json"),
			}
		default:
			return nil
		}
	},
	// Note: Amp also searches ~/.config/amp/skills/ for user-wide skills
	// (second priority after ~/.config/agents/skills/). The Provider struct
	// doesn't have a GlobalDiscoveryPaths field, so this is documented here.
	// InstallDir targets ~/.config/agents/skills/ (highest priority).
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
		catalog.Hooks:  false, // JSON merge into amp.hooks key
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".amp/settings.json", // amp.hooks array inside settings.json
		catalog.MCP:   ".amp/settings.json",
	},
	MCPTransports: []string{"stdio", "sse"},
	HookTypes:     []string{"command"},
}
