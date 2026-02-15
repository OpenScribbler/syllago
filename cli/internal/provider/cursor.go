package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
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
}
