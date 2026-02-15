package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

var Codex = Provider{
	Name:      "Codex",
	Slug:      "codex",
	ConfigDir: ".codex",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return filepath.Join(homeDir, ".codex")
		case catalog.Commands:
			return filepath.Join(homeDir, ".codex")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for .codex directory
		info, err := os.Stat(filepath.Join(homeDir, ".codex"))
		if err == nil && info.IsDir() {
			return true
		}
		// Also check if codex command exists
		_, err = exec.LookPath("codex")
		return err == nil
	},
}
