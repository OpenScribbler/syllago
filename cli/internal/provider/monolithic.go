package provider

// MonolithicFilenames returns the set of monolithic rule filenames that the
// provider identified by slug authors at project or home scope (D2, §research).
// Each slug may have one or more. Returns nil for unknown slugs.
func MonolithicFilenames(slug string) []string {
	switch slug {
	case "claude-code":
		return []string{"CLAUDE.md"}
	case "codex":
		return []string{"AGENTS.md"}
	case "gemini-cli":
		return []string{"GEMINI.md"}
	case "cursor":
		return []string{".cursorrules"}
	case "cline":
		return []string{".clinerules"}
	case "windsurf":
		return []string{".windsurfrules"}
	}
	return nil
}

// AllMonolithicFilenames returns the union of monolithic filenames across
// every provider that has one. Used by discovery to build a single filename
// filter across all providers.
func AllMonolithicFilenames() []string {
	return []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", ".cursorrules", ".clinerules", ".windsurfrules"}
}

// SlugForMonolithicFilename returns the provider slug that owns the given
// monolithic filename (e.g., "CLAUDE.md" → "claude-code"). Empty string
// means the filename is not a recognized monolithic source.
func SlugForMonolithicFilename(filename string) string {
	switch filename {
	case "CLAUDE.md":
		return "claude-code"
	case "AGENTS.md":
		return "codex"
	case "GEMINI.md":
		return "gemini-cli"
	case ".cursorrules":
		return "cursor"
	case ".clinerules":
		return "cline"
	case ".windsurfrules":
		return "windsurf"
	}
	return ""
}

// MonolithicHint returns a one-line, non-blocking hint for providers with
// strong conventions around monolithic-file install (D10). Empty string
// means the provider has no special guidance.
func MonolithicHint(slug string) string {
	switch slug {
	case "codex":
		return "Codex prefers per-directory AGENTS.md files; consider installing per directory rather than as a single root file."
	case "windsurf":
		return "Windsurf has a 6KB limit on this file; the file rules format (.windsurf/rules/) is recommended for non-trivial content."
	}
	return ""
}
