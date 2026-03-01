package converter

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Result holds converted content and data loss warnings.
type Result struct {
	Content    []byte            // Transformed bytes (nil = skip this item)
	Filename   string            // Output filename (e.g. "rule.mdc" for Cursor)
	Warnings   []string          // Human-readable data loss messages
	ExtraFiles map[string][]byte // Additional files to write (e.g. generated hook scripts)
}

// Converter transforms content for a target provider.
type Converter interface {
	// Canonicalize converts provider-specific content to canonical format.
	// Used during import/add.
	Canonicalize(content []byte, sourceProvider string) (*Result, error)

	// Render converts canonical content to a target provider's format.
	// Used during export/install.
	Render(content []byte, target provider.Provider) (*Result, error)

	// ContentType returns which content type this converter handles.
	ContentType() catalog.ContentType
}

// registry maps content types to their converters.
var registry = map[catalog.ContentType]Converter{}

// Register adds a converter to the registry. Called from init() in converter files.
func Register(c Converter) {
	registry[c.ContentType()] = c
}

// For returns the converter for a content type, or nil if none registered.
func For(ct catalog.ContentType) Converter {
	return registry[ct]
}

// SourceDir is the subdirectory name where original source files are preserved.
const SourceDir = ".source"

// HasSourceFile checks whether a .source/ directory exists for a content item.
func HasSourceFile(item catalog.ContentItem) bool {
	info, err := os.Stat(filepath.Join(item.Path, SourceDir))
	return err == nil && info.IsDir()
}

// SourceFilePath returns the path to the first file in the .source/ directory.
// Returns "" if no source file exists.
func SourceFilePath(item catalog.ContentItem) string {
	dir := filepath.Join(item.Path, SourceDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// ResolveContentFile finds the canonical content file for a content item.
// Looks for known canonical filenames first, then falls back to extension matching.
func ResolveContentFile(item catalog.ContentItem) string {
	// Try known canonical filenames in priority order
	knownNames := []string{"rule.md", "command.md", "SKILL.md", "agent.md", "hooks.json", "mcp.json"}
	for _, name := range knownNames {
		p := filepath.Join(item.Path, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	entries, err := os.ReadDir(item.Path)
	if err != nil {
		return ""
	}

	// Look for .md files (excluding README, LLM-PROMPT)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" && e.Name() != "README.md" && e.Name() != "LLM-PROMPT.md" {
			return filepath.Join(item.Path, e.Name())
		}
	}
	// Look for .toml files (Gemini commands)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".toml" {
			return filepath.Join(item.Path, e.Name())
		}
	}
	// Look for .json files (hooks, MCP)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			return filepath.Join(item.Path, e.Name())
		}
	}
	return ""
}
