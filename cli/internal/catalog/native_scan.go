package catalog

import (
	"os"
	"path/filepath"
)

// NativeProviderContent holds found content for one provider.
type NativeProviderContent struct {
	ProviderSlug string
	ProviderName string
	// Discovered files grouped by type label (e.g. "rules", "skills").
	ByType map[string][]string // type label -> file paths
}

// NativeScanResult holds provider-native content found in a directory.
type NativeScanResult struct {
	Providers []NativeProviderContent
	// HasSyllagoStructure is true if the directory looks like a syllago registry
	// (has registry.yaml or has standard content type directories).
	HasSyllagoStructure bool
}

// ScanNativeContent scans dir for provider-native AI tool content.
// Only scans for providers that syllago supports. Returns findings grouped
// by provider. Does not read file contents -- path existence only.
func ScanNativeContent(dir string) NativeScanResult {
	var result NativeScanResult

	// Check for registry.yaml first.
	if _, err := os.Stat(filepath.Join(dir, "registry.yaml")); err == nil {
		result.HasSyllagoStructure = true
		return result
	}

	// Check for syllago content dirs.
	for _, ct := range AllContentTypes() {
		if _, err := os.Stat(filepath.Join(dir, string(ct))); err == nil {
			result.HasSyllagoStructure = true
			return result
		}
	}

	// Scan for provider-native patterns.
	patterns := providerNativePatterns()
	seen := make(map[string]*NativeProviderContent)

	for _, p := range patterns {
		fullPath := filepath.Join(dir, p.path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		pc, ok := seen[p.providerSlug]
		if !ok {
			seen[p.providerSlug] = &NativeProviderContent{
				ProviderSlug: p.providerSlug,
				ProviderName: p.providerName,
				ByType:       make(map[string][]string),
			}
			pc = seen[p.providerSlug]
		}
		if info.IsDir() {
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					pc.ByType[p.typeLabel] = append(pc.ByType[p.typeLabel], filepath.Join(p.path, e.Name()))
				}
			}
		} else {
			pc.ByType[p.typeLabel] = append(pc.ByType[p.typeLabel], p.path)
		}
	}

	for _, pc := range seen {
		if len(pc.ByType) > 0 {
			result.Providers = append(result.Providers, *pc)
		}
	}
	return result
}

// nativePattern describes one provider-native path to check.
type nativePattern struct {
	providerSlug string
	providerName string
	path         string // relative to repo root
	typeLabel    string // e.g. "rules", "skills"
}

// providerNativePatterns returns all known provider-native content paths.
func providerNativePatterns() []nativePattern {
	return []nativePattern{
		// Claude Code
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/commands", typeLabel: "commands"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/skills", typeLabel: "skills"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/agents", typeLabel: "agents"},
		{providerSlug: "claude-code", providerName: "Claude Code", path: "CLAUDE.md", typeLabel: "rules"},
		// Gemini CLI
		{providerSlug: "gemini-cli", providerName: "Gemini CLI", path: ".gemini", typeLabel: "config"},
		// Cursor
		{providerSlug: "cursor", providerName: "Cursor", path: ".cursorrules", typeLabel: "rules"},
		{providerSlug: "cursor", providerName: "Cursor", path: ".cursor/rules", typeLabel: "rules"},
		// Windsurf
		{providerSlug: "windsurf", providerName: "Windsurf", path: ".windsurfrules", typeLabel: "rules"},
		// Codex
		{providerSlug: "codex", providerName: "Codex", path: ".codex", typeLabel: "config"},
		// Copilot CLI
		{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".github/copilot-instructions.md", typeLabel: "rules"},
		// Zed
		{providerSlug: "zed", providerName: "Zed", path: ".zed", typeLabel: "settings"},
		// Cline
		{providerSlug: "cline", providerName: "Cline", path: ".clinerules", typeLabel: "rules"},
		// Roo Code
		{providerSlug: "roo-code", providerName: "Roo Code", path: ".roo", typeLabel: "config"},
		// OpenCode
		{providerSlug: "opencode", providerName: "OpenCode", path: ".opencode", typeLabel: "config"},
		// Kiro
		{providerSlug: "kiro", providerName: "Kiro", path: ".kiro", typeLabel: "config"},
	}
}
