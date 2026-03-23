package provider

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// JSONMergeSentinel is returned by InstallDir when the content type requires
// JSON merge installation (e.g., MCP into config files, hooks into settings.json)
// rather than filesystem placement (symlink or copy).
const JSONMergeSentinel = "__json_merge__"

// ProjectScopeSentinel is returned by InstallDir when the content type is
// supported but lives at project scope (e.g., .kiro/steering/, .clinerules/)
// rather than user scope (~/.provider/). Callers should use DiscoveryPaths
// with the project root to determine the actual install directory.
const ProjectScopeSentinel = "__project_scope__"

// Format identifies a file format used by a provider.
type Format string

const (
	FormatMarkdown Format = "md"
	FormatMDC      Format = "mdc" // Cursor .mdc format
	FormatJSON     Format = "json"
	FormatYAML     Format = "yaml"
	FormatJSONC    Format = "jsonc" // JSON with comments (OpenCode)
	FormatTOML     Format = "toml"  // TOML (Codex multi-agent config)
)

type Provider struct {
	Name      string
	Slug      string // stable identifier, e.g. "claude-code" (matches directory names)
	Detected  bool
	ConfigDir string // e.g. ~/.claude
	// InstallDir returns the target directory for a given content type.
	// Returns empty string if the provider doesn't support that content type.
	InstallDir func(homeDir string, ct catalog.ContentType) string
	// Detect returns true if the provider is installed on the system.
	Detect func(homeDir string) bool

	// Phase 1: discovery and emit fields

	// DiscoveryPaths returns filesystem paths to scan for existing content of
	// the given type within a project root.
	DiscoveryPaths func(projectRoot string, ct catalog.ContentType) []string
	// FileFormat returns the file format used by this provider for a content type.
	FileFormat func(ct catalog.ContentType) Format
	// EmitPath returns the path where syllago should write scan output for this provider.
	EmitPath func(projectRoot string) string
	// SupportsType returns true if this provider handles the given content type.
	SupportsType func(ct catalog.ContentType) bool

	// SymlinkSupport maps content types to whether symlinks are supported.
	// If nil, symlinks are assumed supported for filesystem types.
	// Hooks and MCP are always false (JSON merge, not filesystem).
	SymlinkSupport map[catalog.ContentType]bool
}

// AllProviders returns the full list of known providers (detected or not).
var AllProviders = []Provider{
	ClaudeCode,
	GeminiCLI,
	Cursor,
	Windsurf,
	Codex,
	CopilotCLI,
	Zed,
	Cline,
	RooCode,
	OpenCode,
	Kiro,
	Amp,
}

// DetectedOnly returns the subset of AllProviders that are detected
// in the given home directory.
func DetectedOnly(home string) []Provider {
	var detected []Provider
	for _, prov := range AllProviders {
		if prov.Detect != nil && prov.Detect(home) {
			detected = append(detected, prov)
		}
	}
	return detected
}
