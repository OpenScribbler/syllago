package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

var Windsurf = Provider{
	Name:      "Windsurf",
	Slug:      "windsurf",
	ConfigDir: ".codeium/windsurf",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return filepath.Join(homeDir, ".codeium", "windsurf")
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
			return []string{filepath.Join(projectRoot, ".windsurfrules")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		return FormatMarkdown
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".windsurfrules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules:
			return true
		default:
			return false
		}
	},
}
