package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

var Zed = Provider{
	Name:      "Zed",
	Slug:      "zed",
	ConfigDir: ".config/zed",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Rules go in project root as .rules
		case catalog.MCP:
			return JSONMergeSentinel // Merges into ~/.config/zed/settings.json
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".config", "zed"))
		if err == nil && info.IsDir() {
			return true
		}
		_, err = exec.LookPath("zed")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, ".rules")}
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
		return filepath.Join(projectRoot, ".rules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.MCP:
			return true
		default:
			return false
		}
	},
}
