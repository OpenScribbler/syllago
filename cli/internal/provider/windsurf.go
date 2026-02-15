package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
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
}
