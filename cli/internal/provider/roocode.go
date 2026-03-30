package provider

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var RooCode = Provider{
	Name:      "Roo Code",
	Slug:      "roo-code",
	ConfigDir: ".roo",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Rules go in project root as .roo/rules/
		case catalog.Skills:
			return filepath.Join(homeDir, ".roo", "skills")
		case catalog.MCP:
			return JSONMergeSentinel // Merges into .roo/mcp.json
		case catalog.Agents:
			return ProjectScopeSentinel // Custom modes in project .roomodes or .roo/
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for ~/.roo/ global config directory
		info, err := os.Stat(filepath.Join(homeDir, ".roo"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			homeDir, _ := os.UserHomeDir()
			paths := []string{
				filepath.Join(projectRoot, ".roo", "rules"),
				filepath.Join(projectRoot, ".roo", "rules-code"),
				filepath.Join(projectRoot, ".roo", "rules-architect"),
				filepath.Join(projectRoot, ".roo", "rules-ask"),
				filepath.Join(projectRoot, ".roo", "rules-debug"),
				filepath.Join(projectRoot, ".roo", "rules-orchestrator"),
				filepath.Join(projectRoot, ".roorules"),
			}
			if homeDir != "" {
				paths = append(paths, filepath.Join(homeDir, ".roo", "rules"))
			}
			return paths
		case catalog.Skills:
			return []string{
				filepath.Join(projectRoot, ".roo", "skills"),
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		case catalog.MCP:
			return []string{filepath.Join(projectRoot, ".roo", "mcp.json")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.MCP:
			return FormatJSON
		case catalog.Agents:
			return FormatYAML
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".roo", "rules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.MCP, catalog.Agents:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Skills: true,
		catalog.Agents: true,
		catalog.MCP:    false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.MCP: ".roo/mcp.json",
	},
	MCPTransports: []string{"stdio", "sse", "streamable-http"},
}
