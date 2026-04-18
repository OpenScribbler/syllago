package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Windsurf = Provider{
	Name:      "Windsurf",
	Slug:      "windsurf",
	ConfigDir: ".codeium/windsurf",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".codeium", "windsurf")
		switch ct {
		case catalog.Rules:
			return base
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Hooks:
			return JSONMergeSentinel
		case catalog.MCP:
			return JSONMergeSentinel
		case catalog.Commands:
			return filepath.Join(base, "global_workflows")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".codeium", "windsurf"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{
				filepath.Join(projectRoot, ".windsurfrules"),
				filepath.Join(projectRoot, ".windsurf", "rules"),
			}
		case catalog.Skills:
			return []string{
				filepath.Join(projectRoot, ".windsurf", "skills"),
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		case catalog.Commands:
			// Workflows live at .windsurf/workflows/ (project) and ~/.codeium/windsurf/global_workflows (global).
			paths := []string{filepath.Join(projectRoot, ".windsurf", "workflows")}
			if home, err := os.UserHomeDir(); err == nil {
				paths = append(paths, filepath.Join(home, ".codeium", "windsurf", "global_workflows"))
			}
			return paths
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Hooks, catalog.MCP:
			return FormatJSON
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".windsurf", "rules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Commands:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Skills:   true,
		catalog.Commands: true,  // File-based workflows
		catalog.Hooks:    false, // JSON merge
		catalog.MCP:      false, // JSON merge
	},
	GlobalSharedReadPaths: func(homeDir string, ct catalog.ContentType) []string {
		if ct == catalog.Skills {
			return []string{filepath.Join(homeDir, ".agents", "skills")}
		}
		return nil
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".windsurf/hooks.json",
		catalog.MCP:   ".windsurf/mcp_config.json",
	},
	MCPTransports: []string{"stdio", "sse"},
	HookTypes:     []string{"command"},
}
