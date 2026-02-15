package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

var GeminiCLI = Provider{
	Name:      "Gemini CLI",
	Slug:      "gemini-cli",
	ConfigDir: ".gemini",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".gemini")
		switch ct {
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Rules:
			return base // GEMINI.md goes in .gemini/
		case catalog.Hooks:
			return JSONMergeSentinel
		case catalog.Commands:
			return base
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.MCP:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".gemini"))
		return err == nil && info.IsDir()
	},
}
