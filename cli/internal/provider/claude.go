package provider

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

var ClaudeCode = Provider{
	Name:      "Claude Code",
	Slug:      "claude-code",
	ConfigDir: ".claude",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".claude")
		switch ct {
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Rules:
			return filepath.Join(base, "rules")
		case catalog.Commands:
			return filepath.Join(base, "commands")
		case catalog.Agents:
			return filepath.Join(base, "agents")
		case catalog.MCP:
			return JSONMergeSentinel
		case catalog.Hooks:
			return JSONMergeSentinel
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".claude"))
		return err == nil && info.IsDir()
	},
}
