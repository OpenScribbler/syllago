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
		case catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:  true,
		catalog.Skills: true,
		catalog.Hooks:  false, // JSON merge
		catalog.MCP:    false, // JSON merge
	},
}
