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
