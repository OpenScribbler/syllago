package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

var Cursor = Provider{
	Name:      "Cursor",
	Slug:      "cursor",
	ConfigDir: ".cursor",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return filepath.Join(homeDir, ".cursor")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".cursor"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, ".cursor", "rules")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Rules:
			return FormatMDC
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, ".cursor", "rules", "nesco-context.mdc")
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
