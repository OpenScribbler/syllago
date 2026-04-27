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
	return clineMCPSettingsPathFor(runtime.GOOS, home, os.Getenv("APPDATA"))
}

// clineMCPSettingsPathFor computes the settings path for a given OS, home,
// and APPDATA value. Separated from ClineMCPSettingsPath so unit tests can
// cover every runtime branch — runtime.GOOS is a compile-time constant on
// any single test run, so the live function only ever exercises one branch
// per platform.
func clineMCPSettingsPathFor(goos, home, appdata string) string {
	const ext = "saoudrizwan.claude-dev"
	rel := filepath.Join("settings", "cline_mcp_settings.json")

	switch goos {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", ext, rel)
	case "windows":
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Code", "User", "globalStorage", ext, rel)
	default: // linux and other non-mac/win unix-likes
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
		case catalog.Skills:
			// Global skills live at ~/.cline/skills/<name>/SKILL.md per
			// https://docs.cline.bot/customization/skills.md#where-skills-live
			return filepath.Join(homeDir, ".cline", "skills")
		case catalog.Hooks:
			return ProjectScopeSentinel // Hooks are file-based executables in .clinerules/hooks/
		case catalog.MCP:
			return JSONMergeSentinel // Merges into VS Code globalStorage config
		case catalog.Commands:
			// Global workflows live at ~/Documents/Cline/Workflows on all platforms
			// (Cline uses the Documents folder, which maps to %USERPROFILE%\Documents on Windows).
			return filepath.Join(homeDir, "Documents", "Cline", "Workflows")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Advisory only — see Provider.Detect doc. Cline ships as a VS Code
		// extension, so the only reliable signal is the extension dir at
		// ~/.vscode/extensions/saoudrizwan.claude-dev-*/. ~/.cline/ and
		// ~/.clinerules/ are syllago install paths and don't count.
		return vscodeExtensionInstalled(homeDir, "saoudrizwan.claude-dev")
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			paths := []string{filepath.Join(projectRoot, ".clinerules")}
			if home, err := os.UserHomeDir(); err == nil {
				paths = append(paths, filepath.Join(home, "Documents", "Cline", "Rules"))
			}
			return paths
		case catalog.Skills:
			// Cline discovers skills at three project-scope directories and one global-scope
			// directory. `.cline/skills/` is the canonical path per docs; `.clinerules/skills/`
			// and `.claude/skills/` are documented interop fallbacks.
			paths := []string{
				filepath.Join(projectRoot, ".cline", "skills"),
				filepath.Join(projectRoot, ".clinerules", "skills"),
				filepath.Join(projectRoot, ".claude", "skills"),
			}
			if home, err := os.UserHomeDir(); err == nil {
				paths = append(paths, filepath.Join(home, ".cline", "skills"))
			}
			return paths
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".clinerules", "hooks")}
		case catalog.MCP:
			if p := ClineMCPSettingsPath(); p != "" {
				return []string{p}
			}
			return nil
		case catalog.Commands:
			// Workflows live at .clinerules/workflows/ (project) and ~/Documents/Cline/Workflows (global).
			paths := []string{filepath.Join(projectRoot, ".clinerules", "workflows")}
			if home, err := os.UserHomeDir(); err == nil {
				paths = append(paths, filepath.Join(home, "Documents", "Cline", "Workflows"))
			}
			return paths
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
		case catalog.Rules, catalog.Skills, catalog.Hooks, catalog.MCP, catalog.Commands:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Skills:   true,  // File-based SKILL.md in skill directories
		catalog.Hooks:    true,  // File-based executables
		catalog.Commands: true,  // File-based workflows
		catalog.MCP:      false, // JSON merge
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".clinerules/hooks",
		catalog.MCP:   "cline_mcp_settings.json",
	},
	MCPTransports: []string{"stdio", "sse", "streamable-http"},
	HookTypes:     []string{"command"},
}
