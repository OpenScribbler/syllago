package provider

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Pi is the badlogic/pi-mono coding agent.
// Pi's global config layout is ~/.pi/agent/<type>/, project-scope is .pi/<type>/.
// Pi hooks are programmatic TypeScript files installed to the extensions
// directory (filesystem install, not JSON merge). No native MCP or agent format.
var Pi = Provider{
	Name:      "Pi",
	Slug:      "pi",
	ConfigDir: ".pi",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		base := filepath.Join(homeDir, ".pi", "agent")
		switch ct {
		case catalog.Rules:
			return ProjectScopeSentinel // AGENTS.md at project root
		case catalog.Skills:
			return filepath.Join(base, "skills")
		case catalog.Hooks:
			return filepath.Join(base, "extensions") // TypeScript extension files
		case catalog.Commands:
			return filepath.Join(base, "prompts") // Prompt templates
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		info, err := os.Stat(filepath.Join(homeDir, ".pi"))
		if err == nil && info.IsDir() {
			return true
		}
		_, err = exec.LookPath("pi")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Skills:
			return []string{filepath.Join(projectRoot, ".pi", "skills")}
		case catalog.Hooks:
			return []string{filepath.Join(projectRoot, ".pi", "extensions")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".pi", "prompts")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Hooks:
			return FormatTypeScript
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Skills, catalog.Hooks, catalog.Commands:
			return true
		default:
			return false
		}
	},
	SymlinkSupport: map[catalog.ContentType]bool{
		catalog.Rules:    true,
		catalog.Skills:   true,
		catalog.Hooks:    true, // TypeScript file drop (not JSON merge)
		catalog.Commands: true,
	},
	ConfigLocations: map[catalog.ContentType]string{
		catalog.Hooks: ".pi/extensions/",
	},
	HookTypes: []string{"command"}, // Programmatic TypeScript exposed as command
}
