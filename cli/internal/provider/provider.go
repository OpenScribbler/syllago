package provider

import "github.com/holdenhewett/romanesco/cli/internal/catalog"

// JSONMergeSentinel is returned by InstallDir when the content type requires
// JSON merge installation (e.g., MCP into config files, hooks into settings.json)
// rather than filesystem placement (symlink or copy).
const JSONMergeSentinel = "__json_merge__"

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
}

// AllProviders returns the full list of known providers (detected or not).
var AllProviders = []Provider{
	ClaudeCode,
	GeminiCLI,
	Cursor,
	Windsurf,
	Codex,
}

// SupportsType returns true if the provider has an install path for the given content type.
func (p Provider) SupportsType(ct catalog.ContentType) bool {
	return p.InstallDir("", ct) != ""
}
