package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Kiro = Provider{
	Name:      "Kiro",
	Slug:      "kiro",
	ConfigDir: ".kiro",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".kiro")
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Steering files go in project .kiro/steering/
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.Skills:
			return ProjectScopeSentinel // Skills map to steering files in project .kiro/steering/
		case catalog.Hooks:
			return JSONMergeSentinel // Hooks live inside agent JSON files
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".kiro"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, ".kiro", "steering")}
		case catalog.Agents:
			return []string{
				filepath.Join(projectRoot, ".kiro", "agents"),
			}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".kiro", "steering")}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".kiro", "settings", "mcp.json")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".kiro", "agents")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.MCP, catalog.Hooks:
			return FormatJSON
		case catalog.Agents:
			return FormatMarkdown
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".kiro", "steering")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Agents, catalog.Hooks, catalog.MCP, catalog.Skills:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Agents: true,
		catalog.Skills: true,
		catalog.Hooks:  false, // JSON merge
		catalog.MCP:    false, // JSON merge
	},
}
