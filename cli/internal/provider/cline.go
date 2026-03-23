package provider

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ClineMCPSettingsPath returns the platform-aware path to Cline's MCP settings
// in VS Code's globalStorage directory.
func ClineMCPSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	const ext = "saoudrizwan.claude-dev"
	rel := filepath.Join("settings", "cline_mcp_settings.json")

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", ext, rel)
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Code", "User", "globalStorage", ext, rel)
	default: // linux
		return filepath.Join(home, ".config", "Code", "User", "globalStorage", ext, rel)
	}
}

var Cline = Provider{
	Name:      "Cline",
	Slug:      "cline",
	ConfigDir: ".clinerules",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // Rules go in project root as .clinerules/ directory
		case catalog.Hooks:
			return ProjectScopeSentinel // Hooks are file-based executables in .clinerules/hooks/
		case catalog.MCP:
			return JSONMergeSentinel // Merges into VS Code globalStorage config
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for .clinerules/ directory in common project locations
		// Cline is a VS Code extension; detection is project-level
		info, err := os.Stat(filepath.Join(homeDir, "Documents", "Cline", "Rules"))
		return err == nil && info.IsDir()
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			paths := []string{filepath.Join(projectRoot, ".clinerules")}
			if home, err := os.UserHomeDir(); err == nil {
				paths = append(paths, filepath.Join(home, "Documents", "Cline", "Rules"))
			}
			return paths
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".clinerules", "hooks")}
		case catalog.MCP:
			if p := ClineMCPSettingsPath(); p != "" {
				return []string{p}
			}
			return nil
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
		return filepath.Join(projectRoot, ".clinerules")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Hooks, catalog.MCP:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules: true,
		catalog.Hooks: true,  // File-based executables
		catalog.MCP:   false, // JSON merge
	},
}
