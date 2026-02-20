package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
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
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".codex", "commands")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		return FormatMarkdown
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Commands:
			return true
		default:
			return false
		}
	},
}
